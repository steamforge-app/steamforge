package steam

import (
	"runtime"
	"unsafe"
)

// ISteamApps001 wraps the oldest ISteamApps interface which provides GetAppData.
// This is needed to look up game names by appID (used by SAM).
type ISteamApps001 struct {
	ptr uintptr
}

// NewISteamApps001 wraps a raw ISteamApps001 interface pointer.
func NewISteamApps001(ptr uintptr) *ISteamApps001 {
	return &ISteamApps001{ptr: ptr}
}

// Slot 0: GetAppData(appID uint32, key string) -> string
// Returns game metadata like "name", "logo", "small_capsule/english" etc.
func (a *ISteamApps001) GetAppData(appID uint32, key string) string {
	keyStr := NewCString(key)
	defer keyStr.Free()

	const bufLen = 1024
	buf := make([]byte, bufLen)
	var pinner runtime.Pinner
	pinner.Pin(&buf[0])
	defer pinner.Unpin()

	ret := CallProc(
		ReadVtableSlot(a.ptr, 0),
		a.ptr,
		uintptr(appID),
		keyStr.Ptr(),
		uintptr(unsafe.Pointer(&buf[0])),
		bufLen,
	)
	if ret == 0 {
		return ""
	}

	// Find null terminator
	for i, b := range buf {
		if b == 0 {
			return string(buf[:i])
		}
	}
	return string(buf)
}
