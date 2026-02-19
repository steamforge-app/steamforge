package steam

import "unsafe"

// Vtable slots: 44 (0-43)
type ISteamUserStats struct {
	ptr uintptr
}

func NewISteamUserStats(ptr uintptr) *ISteamUserStats {
	return &ISteamUserStats{ptr: ptr}
}

// IsValid returns true if the interface has a non-null internal pointer.
// Steam can return a non-nil wrapper struct with ptr == 0 for games
// that have no stats/achievements, which would SIGSEGV on any vtable call.
func (s *ISteamUserStats) IsValid() bool {
	return s != nil && s.ptr != 0
}

// Slot 6
func (s *ISteamUserStats) SetAchievement(name string) bool {
	cs := NewCString(name)
	defer cs.Free()
	ret := CallProc(ReadVtableSlot(s.ptr, 6), s.ptr, cs.Ptr())
	return ret != 0
}

// Slot 7
func (s *ISteamUserStats) ClearAchievement(name string) bool {
	cs := NewCString(name)
	defer cs.Free()
	ret := CallProc(ReadVtableSlot(s.ptr, 7), s.ptr, cs.Ptr())
	return ret != 0
}

// Slot 9
func (s *ISteamUserStats) StoreStats() bool {
	ret := CallProc(ReadVtableSlot(s.ptr, 9), s.ptr)
	return ret != 0
}

// Slot 13
func (s *ISteamUserStats) GetNumAchievements() uint32 {
	ret := CallProc(ReadVtableSlot(s.ptr, 13), s.ptr)
	return uint32(ret)
}

// Slot 14
func (s *ISteamUserStats) GetAchievementName(index uint32) string {
	ret := CallProc(ReadVtableSlot(s.ptr, 14), s.ptr, uintptr(index))
	return GoString(ret)
}

// Slot 15
func (s *ISteamUserStats) RequestUserStats(steamID uint64) uint64 {
	ret := CallProc(ReadVtableSlot(s.ptr, 15), s.ptr, uintptr(steamID))
	return uint64(ret)
}

// --- CString-reuse variants for batch operations ---

// Slot 8
func (s *ISteamUserStats) GetAchievementAndUnlockTimeCS(cs *CString) (bool, uint32, bool) {
	var achieved byte
	var unlockTime uint32
	ret := CallProc(ReadVtableSlot(s.ptr, 8), s.ptr, cs.Ptr(),
		uintptr(unsafe.Pointer(&achieved)),
		ptrOf(&unlockTime),
	)
	return achieved != 0, unlockTime, ret != 0
}

// Slot 11
func (s *ISteamUserStats) GetAchievementDisplayAttributeCS(csName *CString, csKey *CString) string {
	ret := CallProc(ReadVtableSlot(s.ptr, 11), s.ptr, csName.Ptr(), csKey.Ptr())
	return GoString(ret)
}

// Slot 36
func (s *ISteamUserStats) GetAchievementAchievedPercentCS(cs *CString) (float32, bool) {
	var percent float32
	ret := CallProc(ReadVtableSlot(s.ptr, 36), s.ptr, cs.Ptr(), ptrOfFloat32(&percent))
	return percent, ret != 0
}
