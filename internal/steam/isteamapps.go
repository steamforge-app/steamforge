package steam

// ISteamApps wraps ISteamApps008.
type ISteamApps struct {
	ptr uintptr
}

func NewISteamApps(ptr uintptr) *ISteamApps {
	return &ISteamApps{ptr: ptr}
}

// Slot 6
func (a *ISteamApps) BIsSubscribedApp(appID uint32) bool {
	ret := CallProc(ReadVtableSlot(a.ptr, 6), a.ptr, uintptr(appID))
	return ret != 0
}
