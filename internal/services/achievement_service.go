package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unsafe"

	"steamforge/internal/models"
	"steamforge/internal/schema"
	"steamforge/internal/steam"
)

const maxAchievementNameLen = 256

func validateAchievementName(name string) error {
	if name == "" {
		return errors.New("achievement name is empty")
	}
	if len(name) > maxAchievementNameLen {
		return fmt.Errorf("achievement name too long (%d chars)", len(name))
	}
	if strings.ContainsRune(name, 0) {
		return errors.New("achievement name contains null byte")
	}
	return nil
}

type AchievementService struct {
	client *steam.Client
	ctx    context.Context

	mu           sync.RWMutex
	appID        uint32
	achievements []models.Achievement
	statsReady   chan struct{}
	statsLoaded  bool
}

func NewAchievementService(client *steam.Client) *AchievementService {
	return &AchievementService{
		client: client,
	}
}

func (s *AchievementService) SetContext(ctx context.Context) {
	s.ctx = ctx
}

func (s *AchievementService) LoadAchievements(appID uint32) ([]models.Achievement, error) {
	loadStart := time.Now()
	slog.Info("loading achievements", "appID", appID)

	userStats := s.client.UserStats()
	if userStats == nil || !userStats.IsValid() {
		return nil, errors.New("UserStats interface not available")
	}

	s.mu.RLock()
	alreadyLoaded := s.statsLoaded && s.appID == appID
	s.mu.RUnlock()

	var gameSchema *schema.GameSchema
	var schemaErr error
	schemaDone := make(chan struct{})
	go func() {
		schemaStart := time.Now()
		gameSchema, schemaErr = schema.Load(appID, "english")
		slog.Debug("schema load finished", "appID", appID, "elapsed", time.Since(schemaStart))
		close(schemaDone)
	}()

	if !alreadyLoaded {
		s.mu.Lock()
		s.appID = appID
		s.statsReady = make(chan struct{})
		s.statsLoaded = false
		s.mu.Unlock()

		// Capture the ready channel BEFORE registering the callback to avoid
		// a race where the callback fires and nils statsReady before we read it.
		s.mu.RLock()
		ready := s.statsReady
		s.mu.RUnlock()

		s.client.Callbacks().RegisterOne(steam.CallbackUserStatsReceived, func(paramPtr unsafe.Pointer, paramSize int32) {
			if paramSize >= int32(unsafe.Sizeof(steam.UserStatsReceived{})) {
				result := (*steam.UserStatsReceived)(paramPtr)
				if result.Result == steam.ResultOK {
					slog.Debug("stats received callback", "appID", appID, "elapsed", time.Since(loadStart))
					s.mu.Lock()
					s.statsLoaded = true
					if s.statsReady != nil {
						close(s.statsReady)
						s.statsReady = nil
					}
					s.mu.Unlock()
				} else {
					slog.Warn("stats received with error", "appID", appID, "result", result.Result)
					s.mu.Lock()
					if s.statsReady != nil {
						close(s.statsReady)
						s.statsReady = nil
					}
					s.mu.Unlock()
				}
			}
		})

		callHandle := userStats.RequestUserStats(s.client.SteamID())
		slog.Debug("RequestUserStats called", "appID", appID, "steamID", s.client.SteamID(), "callHandle", callHandle)

		if ready != nil {
			select {
			case <-ready:
				slog.Debug("stats ready", "appID", appID, "elapsed", time.Since(loadStart))
			case <-time.After(10 * time.Second):
				slog.Warn("stats request timed out", "appID", appID)
			}
		}
	} else {
		slog.Debug("stats already loaded, skipping RequestUserStats", "appID", appID)
	}

	<-schemaDone
	if schemaErr != nil {
		slog.Warn("schema load failed", "appID", appID, "error", schemaErr)
	}

	enumStart := time.Now()
	var achievements []models.Achievement

	if gameSchema != nil && len(gameSchema.Achievements) > 0 {
		slog.Debug("using schema for achievement enumeration", "appID", appID, "schemaCount", len(gameSchema.Achievements))
		achievements = s.loadFromSchema(appID, userStats, gameSchema)
	} else {
		numAchievements := userStats.GetNumAchievements()
		slog.Debug("using API for achievement enumeration", "appID", appID, "apiCount", numAchievements)
		achievements = s.loadFromAPI(appID, userStats, numAchievements, gameSchema)
	}
	slog.Info("achievement enumeration done", "appID", appID, "count", len(achievements), "enumElapsed", time.Since(enumStart))

	s.mu.Lock()
	s.achievements = achievements
	s.mu.Unlock()

	slog.Info("achievements loaded", "appID", appID, "count", len(achievements), "totalElapsed", time.Since(loadStart))
	return achievements, nil
}

func (s *AchievementService) loadFromSchema(appID uint32, userStats *steam.ISteamUserStats, gameSchema *schema.GameSchema) []models.Achievement {
	achievements := make([]models.Achievement, 0, len(gameSchema.Achievements))
	iconBaseURL := fmt.Sprintf("https://cdn.cloudflare.steamstatic.com/steamcommunity/public/images/apps/%d/", appID)

	csKeyName := steam.NewCString("name")
	csKeyDesc := steam.NewCString("desc")
	csKeyIcon := steam.NewCString("icon")
	csKeyIconGray := steam.NewCString("icon_gray")
	defer csKeyName.Free()
	defer csKeyDesc.Free()
	defer csKeyIcon.Free()
	defer csKeyIconGray.Free()

	for _, def := range gameSchema.Achievements {
		if def.ID == "" {
			continue
		}

		csID := steam.NewCString(def.ID)

		ach := models.Achievement{
			ID:          def.ID,
			Name:        def.Name,
			Description: def.Description,
			IsHidden:    def.IsHidden,
			Permission:  def.Permission,
		}

		achieved, unlockTime, ok := userStats.GetAchievementAndUnlockTimeCS(csID)
		if ok {
			ach.IsAchieved = achieved
			ach.UnlockTime = unlockTime
		}

		if ach.Name == "" {
			if displayName := userStats.GetAchievementDisplayAttributeCS(csID, csKeyName); displayName != "" {
				ach.Name = displayName
			}
		}
		if ach.Description == "" {
			if desc := userStats.GetAchievementDisplayAttributeCS(csID, csKeyDesc); desc != "" {
				ach.Description = desc
			}
		}

		if def.IconNormal != "" {
			ach.IconURL = iconBaseURL + def.IconNormal
		}
		if def.IconLocked != "" {
			ach.IconGrayURL = iconBaseURL + def.IconLocked
		}

		resolveIconsFromSDK(&ach, userStats, csID, csKeyIcon, csKeyIconGray, iconBaseURL)

		percent, ok := userStats.GetAchievementAchievedPercentCS(csID)
		if ok {
			ach.Percent = percent
		}

		csID.Free()
		achievements = append(achievements, ach)
	}

	return achievements
}

func (s *AchievementService) loadFromAPI(appID uint32, userStats *steam.ISteamUserStats, numAchievements uint32, gameSchema *schema.GameSchema) []models.Achievement {
	achievements := make([]models.Achievement, 0, numAchievements)
	iconBaseURL := fmt.Sprintf("https://cdn.cloudflare.steamstatic.com/steamcommunity/public/images/apps/%d/", appID)

	csKeyName := steam.NewCString("name")
	csKeyDesc := steam.NewCString("desc")
	csKeyHidden := steam.NewCString("hidden")
	csKeyIcon := steam.NewCString("icon")
	csKeyIconGray := steam.NewCString("icon_gray")
	defer csKeyName.Free()
	defer csKeyDesc.Free()
	defer csKeyHidden.Free()
	defer csKeyIcon.Free()
	defer csKeyIconGray.Free()

	schemaMap := make(map[string]*schema.AchievementDefinition)
	if gameSchema != nil {
		for i := range gameSchema.Achievements {
			schemaMap[gameSchema.Achievements[i].ID] = &gameSchema.Achievements[i]
		}
	}

	for i := range numAchievements {
		name := userStats.GetAchievementName(i)
		if name == "" {
			continue
		}

		csID := steam.NewCString(name)

		achieved, unlockTime, _ := userStats.GetAchievementAndUnlockTimeCS(csID)
		displayName := userStats.GetAchievementDisplayAttributeCS(csID, csKeyName)
		description := userStats.GetAchievementDisplayAttributeCS(csID, csKeyDesc)
		hidden := userStats.GetAchievementDisplayAttributeCS(csID, csKeyHidden) == "1"

		ach := models.Achievement{
			ID:          name,
			Name:        displayName,
			Description: description,
			IsAchieved:  achieved,
			UnlockTime:  unlockTime,
			IsHidden:    hidden,
		}

		if def, ok := schemaMap[name]; ok {
			if def.Name != "" && ach.Name == "" {
				ach.Name = def.Name
			}
			if def.Description != "" && ach.Description == "" {
				ach.Description = def.Description
			}
			ach.IsHidden = def.IsHidden
			ach.Permission = def.Permission

			if def.IconNormal != "" {
				ach.IconURL = iconBaseURL + def.IconNormal
			}
			if def.IconLocked != "" {
				ach.IconGrayURL = iconBaseURL + def.IconLocked
			}
		}

		resolveIconsFromSDK(&ach, userStats, csID, csKeyIcon, csKeyIconGray, iconBaseURL)

		percent, ok := userStats.GetAchievementAchievedPercentCS(csID)
		if ok {
			ach.Percent = percent
		}

		csID.Free()
		achievements = append(achievements, ach)
	}

	return achievements
}

// resolveIconsFromSDK fills in missing icon URLs from the SDK display attributes.
func resolveIconsFromSDK(ach *models.Achievement, userStats *steam.ISteamUserStats, csID, csKeyIcon, csKeyIconGray *steam.CString, iconBaseURL string) {
	if ach.IconURL == "" {
		if iconName := userStats.GetAchievementDisplayAttributeCS(csID, csKeyIcon); iconName != "" {
			ach.IconURL = iconBaseURL + iconName
		}
	}
	if ach.IconGrayURL == "" {
		if iconGray := userStats.GetAchievementDisplayAttributeCS(csID, csKeyIconGray); iconGray != "" {
			ach.IconGrayURL = iconBaseURL + iconGray
		}
	}
}

// HasAchievementsFromSchema checks the local schema file for achievement count
// without needing a Steam connection. Returns (total, true) if schema exists,
// or (0, false) if no schema file found.
func HasAchievementsFromSchema(appID uint32) (int, bool) {
	gameSchema, err := schema.Load(appID, "english")
	if err != nil {
		return 0, false
	}
	return len(gameSchema.Achievements), true
}

// MergeSchemaPermissions loads the local schema for a game and applies the
// permission field to community-loaded achievements. The community profile
// endpoint doesn't include permission data.
func MergeSchemaPermissions(appID uint32, achievements []models.Achievement) {
	gameSchema, err := schema.Load(appID, "english")
	if err != nil {
		return
	}
	perms := make(map[string]int, len(gameSchema.Achievements))
	for _, def := range gameSchema.Achievements {
		if def.Permission > 0 {
			perms[def.ID] = def.Permission
		}
	}
	if len(perms) == 0 {
		return
	}
	for i := range achievements {
		if p, ok := perms[achievements[i].ID]; ok {
			achievements[i].Permission = p
		}
	}
}

func (s *AchievementService) GetAchievements() []models.Achievement {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.achievements
}

func (s *AchievementService) SetAchievement(name string) (bool, error) {
	if err := validateAchievementName(name); err != nil {
		return false, err
	}
	userStats := s.client.UserStats()
	if userStats == nil {
		return false, errors.New("UserStats not available")
	}

	ok := s.retrySDKCall(func() bool { return userStats.SetAchievement(name) })
	slog.Info("set achievement", "name", name, "ok", ok)

	if ok {
		s.mu.Lock()
		for i := range s.achievements {
			if s.achievements[i].ID == name {
				s.achievements[i].IsAchieved = true
				break
			}
		}
		s.mu.Unlock()
	}

	return ok, nil
}

func (s *AchievementService) ClearAchievement(name string) (bool, error) {
	if err := validateAchievementName(name); err != nil {
		return false, err
	}
	userStats := s.client.UserStats()
	if userStats == nil {
		return false, errors.New("UserStats not available")
	}

	ok := s.retrySDKCall(func() bool { return userStats.ClearAchievement(name) })
	slog.Info("clear achievement", "name", name, "ok", ok)

	if ok {
		s.mu.Lock()
		for i := range s.achievements {
			if s.achievements[i].ID == name {
				s.achievements[i].IsAchieved = false
				s.achievements[i].UnlockTime = 0
				break
			}
		}
		s.mu.Unlock()
	}

	return ok, nil
}

// retrySDKCall retries an SDK operation that may fail transiently right after
// connecting to a game, giving the Steam client time to fully register the session.
func (s *AchievementService) retrySDKCall(call func() bool) bool {
	for attempt := range 3 {
		if call() {
			return true
		}
		slog.Debug("SDK call failed, retrying", "attempt", attempt+1)
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

func (s *AchievementService) SetAllAchievements() (int, error) {
	userStats := s.client.UserStats()
	if userStats == nil {
		return 0, errors.New("UserStats not available")
	}

	count := 0
	s.mu.Lock()
	for i := range s.achievements {
		if s.achievements[i].Permission > 0 || s.achievements[i].IsAchieved {
			continue
		}
		if userStats.SetAchievement(s.achievements[i].ID) {
			s.achievements[i].IsAchieved = true
			count++
		}
	}
	s.mu.Unlock()

	slog.Info("set all achievements", "count", count)
	return count, nil
}

func (s *AchievementService) ClearAllAchievements() (int, error) {
	userStats := s.client.UserStats()
	if userStats == nil {
		return 0, errors.New("UserStats not available")
	}

	count := 0
	s.mu.Lock()
	for i := range s.achievements {
		if s.achievements[i].Permission > 0 || !s.achievements[i].IsAchieved {
			continue
		}
		if userStats.ClearAchievement(s.achievements[i].ID) {
			s.achievements[i].IsAchieved = false
			s.achievements[i].UnlockTime = 0
			count++
		}
	}
	s.mu.Unlock()

	slog.Info("clear all achievements", "count", count)
	return count, nil
}

func (s *AchievementService) StoreStats() (bool, error) {
	userStats := s.client.UserStats()
	if userStats == nil {
		return false, errors.New("UserStats not available")
	}

	stored := make(chan bool, 1)
	s.client.Callbacks().RegisterOne(steam.CallbackUserStatsStored, func(paramPtr unsafe.Pointer, paramSize int32) {
		if paramSize >= int32(unsafe.Sizeof(steam.UserStatsStored{})) {
			result := (*steam.UserStatsStored)(paramPtr)
			stored <- result.Result == steam.ResultOK
		} else {
			stored <- false
		}
	})

	ok := userStats.StoreStats()
	slog.Info("store stats request sent", "ok", ok)
	if !ok {
		return false, nil
	}

	select {
	case success := <-stored:
		slog.Info("store stats confirmed", "success", success)
		if !success {
			return false, errors.New("Steam rejected the stats update")
		}
		return true, nil
	case <-time.After(5 * time.Second):
		slog.Warn("store stats callback timed out — save may still succeed")
		return true, nil
	}
}
