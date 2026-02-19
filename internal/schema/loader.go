package schema

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"steamforge/internal/keyvalue"
	"steamforge/internal/steam"
)

type GameSchema struct {
	AppID        uint32
	Achievements []AchievementDefinition
}

func Load(appID uint32, language string) (*GameSchema, error) {
	installPath, err := steam.GetInstallPath()
	if err != nil {
		return nil, fmt.Errorf("get Steam install path: %w", err)
	}

	fileName := fmt.Sprintf("UserGameStatsSchema_%d.bin", appID)
	path := filepath.Join(installPath, "appcache", "stats", fileName)

	slog.Debug("loading schema", "appID", appID, "path", path)

	kv, err := keyvalue.LoadBinary(path)
	if err != nil {
		return nil, fmt.Errorf("load schema %s: %w", path, err)
	}

	if language == "" {
		language = "english"
	}

	schema := &GameSchema{AppID: appID}

	appNode := kv.Get(fmt.Sprintf("%d", appID))
	if len(appNode.Children) == 0 {
		// ReadBinary unwraps single-root files, so kv itself is the appID node
		appNode = kv
	}
	statsNode := appNode.Get("stats")

	for _, stat := range statsNode.Children {
		// Prefer type_int (integer) over type (may be string like "ACHIEVEMENTS")
		typeIntNode := stat.Get("type_int")
		statType := typeIntNode.AsInt(-1)
		if statType == -1 {
			statType = stat.Get("type").AsInt(-1)
		}

		// Achievement types: 4 = achievements, 5 = group achievements
		// If no type found, check if stat has a "bits" child (implicit achievement)
		if statType != 4 && statType != 5 {
			if statType != -1 {
				continue
			}
			// No type info — check for bits child as a fallback
			hasBits := false
			for _, child := range stat.Children {
				if strings.EqualFold(child.Name, "bits") {
					hasBits = true
					break
				}
			}
			if !hasBits {
				continue
			}
		}

		for _, child := range stat.Children {
			if !strings.EqualFold(child.Name, "bits") {
				continue
			}
			for _, bit := range child.Children {
				schema.Achievements = append(schema.Achievements, AchievementDefinition{
					ID:          bit.Get("name").AsString(""),
					Name:        getLocalizedString(bit.Get("display").Get("name"), language, bit.Get("name").AsString("")),
					Description: getLocalizedString(bit.Get("display").Get("desc"), language, ""),
					IconNormal:  bit.Get("display").Get("icon").AsString(""),
					IconLocked:  bit.Get("display").Get("icon_gray").AsString(""),
					IsHidden:    bit.Get("display").Get("hidden").AsBool(false),
					Permission:  bit.Get("permission").AsInt(0),
				})
			}
		}
	}

	slog.Debug("schema loaded", "appID", appID, "achievements", len(schema.Achievements))
	return schema, nil
}

func getLocalizedString(node *keyvalue.Node, language, defaultValue string) string {
	if node == nil {
		return defaultValue
	}

	val := node.Get(language).AsString("")
	if val != "" {
		return val
	}

	if language != "english" {
		val = node.Get("english").AsString("")
		if val != "" {
			return val
		}
	}

	val = node.AsString("")
	if val != "" {
		return val
	}

	return defaultValue
}
