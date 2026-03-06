package steam

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type Client struct {
	lib         Library
	steamClient *ISteamClient

	pipe    int32
	user    int32
	steamID uint64

	callbacks *CallbackDispatcher

	mu      sync.Mutex
	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup

	closed atomic.Bool

	// Interfaces
	steamUser      *ISteamUser
	steamUserStats *ISteamUserStats
	steamApps      *ISteamApps
	steamApps001   *ISteamApps001
	steamUtils     *ISteamUtils
	steamFriends   *ISteamFriends

	// Current game context
	CurrentAppID uint32
}

func NewClient(appID uint32) (*Client, error) {
	if appID > 0 {
		os.Setenv("SteamAppId", strconv.FormatUint(uint64(appID), 10))
		slog.Info("creating Steam client for app", "appID", appID)
	} else {
		os.Unsetenv("SteamAppId")
		slog.Info("creating Steam client")
	}

	libPath, err := SteamClientLibraryPath()
	if err != nil {
		return nil, fmt.Errorf("find Steam client library: %w", err)
	}
	slog.Debug("Steam client library", "path", libPath)

	lib, err := OpenLibrary(libPath)
	if err != nil {
		return nil, fmt.Errorf("open Steam client library: %w", err)
	}

	createInterfaceFn, err := lib.FindProc("CreateInterface")
	if err != nil {
		lib.Close()
		return nil, fmt.Errorf("find CreateInterface: %w", err)
	}

	versionStr := NewCString("SteamClient018")
	defer versionStr.Free()
	clientPtr := CallProc(createInterfaceFn, versionStr.Ptr(), 0)
	if clientPtr == 0 {
		lib.Close()
		return nil, errors.New("CreateInterface(SteamClient018) returned null")
	}

	steamClient := NewISteamClient(clientPtr)

	pipe := steamClient.CreateSteamPipe()
	if pipe == 0 {
		lib.Close()
		return nil, errors.New("CreateSteamPipe failed")
	}

	user := steamClient.ConnectToGlobalUser(pipe)
	if user == 0 {
		steamClient.ReleaseSteamPipe(pipe)
		lib.Close()
		return nil, errors.New("Steam is not running")
	}

	dispatcher, err := NewCallbackDispatcher(lib, pipe)
	if err != nil {
		steamClient.ReleaseUser(pipe, user)
		steamClient.ReleaseSteamPipe(pipe)
		lib.Close()
		return nil, fmt.Errorf("setup callbacks: %w", err)
	}

	steamUserPtr := steamClient.GetISteamUser(pipe, user, "SteamUser012")
	if steamUserPtr == 0 {
		steamClient.ReleaseUser(pipe, user)
		steamClient.ReleaseSteamPipe(pipe)
		lib.Close()
		return nil, errors.New("GetISteamUser failed")
	}

	steamUser := NewISteamUser(steamUserPtr)
	steamID := steamUser.GetSteamID()

	utilsPtr := steamClient.GetISteamUtils(pipe, "SteamUtils005")

	client := &Client{
		lib:          lib,
		steamClient:  steamClient,
		pipe:         pipe,
		user:         user,
		steamID:      steamID,
		callbacks:    dispatcher,
		stopCh:       make(chan struct{}),
		steamUser:    steamUser,
		CurrentAppID: appID,
	}

	if utilsPtr != 0 {
		client.steamUtils = NewISteamUtils(utilsPtr)
	}

	friendsPtr := steamClient.GetISteamFriends(pipe, user, "SteamFriends017")
	if friendsPtr != 0 {
		client.steamFriends = NewISteamFriends(friendsPtr)
		slog.Info("ISteamFriends initialized", "persona", client.steamFriends.GetPersonaName())
	} else {
		slog.Warn("GetISteamFriends returned null")
	}

	// ISteamApps is always available (needed for game enumeration even at appID=0)
	appsPtr := steamClient.GetISteamApps(pipe, user, "STEAMAPPS_INTERFACE_VERSION008")
	if appsPtr != 0 {
		client.steamApps = NewISteamApps(appsPtr)
	}

	apps001Ptr := steamClient.GetISteamApps(pipe, user, "STEAMAPPS_INTERFACE_VERSION001")
	if apps001Ptr != 0 {
		client.steamApps001 = NewISteamApps001(apps001Ptr)
	}

	// ISteamUserStats requires a specific appID
	if appID > 0 {
		userStatsPtr := steamClient.GetISteamUserStats(pipe, user, "STEAMUSERSTATS_INTERFACE_VERSION013")
		if userStatsPtr != 0 {
			client.steamUserStats = NewISteamUserStats(userStatsPtr)
		} else {
			slog.Warn("GetISteamUserStats returned null", "appID", appID)
		}
	}

	slog.Info("Steam client created", "steamID", steamID, "appID", appID)
	return client, nil
}

func (c *Client) SteamID() uint64 {
	return c.steamID
}

func (c *Client) StartCallbackLoop() {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return
	}
	c.running = true
	dispatcher := c.callbacks // capture under lock
	stopCh := c.stopCh       // capture under lock
	c.mu.Unlock()

	slog.Debug("callback loop started")

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		runtime.LockOSThread()
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stopCh:
				slog.Debug("callback loop stopped")
				return
			case <-ticker.C:
				dispatcher.Poll()
			}
		}
	}()
}

func (c *Client) UserStats() *ISteamUserStats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.steamUserStats
}

func (c *Client) Apps() *ISteamApps {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.steamApps
}

func (c *Client) Apps001() *ISteamApps001 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.steamApps001
}

func (c *Client) Utils() *ISteamUtils {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.steamUtils
}

func (c *Client) PersonaName() string {
	if name := ParsePersonaName(c.steamID); name != "" {
		return name
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.steamFriends == nil {
		return ""
	}
	return c.steamFriends.GetPersonaName()
}

func (c *Client) Callbacks() *CallbackDispatcher {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.callbacks
}

func (c *Client) Close() {
	if !c.closed.CompareAndSwap(false, true) {
		return
	}

	c.mu.Lock()
	if c.running {
		close(c.stopCh)
		c.running = false
	}
	callbacks := c.callbacks
	c.callbacks = nil
	c.mu.Unlock()

	if callbacks != nil {
		callbacks.Close()
	}

	c.wg.Wait()

	c.mu.Lock()
	defer c.mu.Unlock()

	slog.Info("closing Steam client")

	if c.user != 0 {
		c.steamClient.ReleaseUser(c.pipe, c.user)
		c.user = 0
	}
	if c.pipe != 0 {
		c.steamClient.ReleaseSteamPipe(c.pipe)
		c.pipe = 0
	}
	c.steamClient.BShutdownIfAllPipesClosed()
	if c.lib != nil {
		c.lib.Close()
		c.lib = nil
	}
}
