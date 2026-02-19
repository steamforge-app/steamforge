package steam

// ISteamUser wraps ISteamUser012 (or compatible).
type ISteamUser struct {
	ptr uintptr
}

// NewISteamUser wraps a raw ISteamUser interface pointer.
func NewISteamUser(ptr uintptr) *ISteamUser {
	return &ISteamUser{ptr: ptr}
}

// GetSteamID is implemented in platform-specific files:
//   isteamuser_windows.go — MSVC x64 hidden return parameter convention
//   isteamuser_linux.go   — SysV ABI, returns in RAX
