package steam

// ISteamUtils wraps ISteamUtils005 (or compatible).
type ISteamUtils struct {
	ptr uintptr
}

// NewISteamUtils wraps a raw ISteamUtils interface pointer.
func NewISteamUtils(ptr uintptr) *ISteamUtils {
	return &ISteamUtils{ptr: ptr}
}
