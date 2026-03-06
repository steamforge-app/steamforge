package steam

// ISteamFriends wraps ISteamFriends017 (or compatible).
type ISteamFriends struct {
	ptr uintptr
}

// NewISteamFriends wraps a raw ISteamFriends interface pointer.
func NewISteamFriends(ptr uintptr) *ISteamFriends {
	return &ISteamFriends{ptr: ptr}
}

// Slot 0 — returns const char* (pointer to internal Steam buffer).
func (f *ISteamFriends) GetPersonaName() string {
	ret := CallProc(ReadVtableSlot(f.ptr, 0), f.ptr)
	return GoString(ret)
}
