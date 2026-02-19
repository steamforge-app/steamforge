package steam

// ISteamClient018 - Vtable slots: 40 (0-39)
type ISteamClient struct {
	ptr uintptr
}

func NewISteamClient(ptr uintptr) *ISteamClient {
	return &ISteamClient{ptr: ptr}
}

// Slot 0
func (c *ISteamClient) CreateSteamPipe() int32 {
	ret := CallProc(ReadVtableSlot(c.ptr, 0), c.ptr)
	return int32(ret)
}

// Slot 1
func (c *ISteamClient) ReleaseSteamPipe(pipe int32) bool {
	ret := CallProc(ReadVtableSlot(c.ptr, 1), c.ptr, uintptr(pipe))
	return ret != 0
}

// Slot 2
func (c *ISteamClient) ConnectToGlobalUser(pipe int32) int32 {
	ret := CallProc(ReadVtableSlot(c.ptr, 2), c.ptr, uintptr(pipe))
	return int32(ret)
}

// Slot 4
func (c *ISteamClient) ReleaseUser(pipe, user int32) {
	CallProc(ReadVtableSlot(c.ptr, 4), c.ptr, uintptr(pipe), uintptr(user))
}

// Slot 5
func (c *ISteamClient) GetISteamUser(pipe, user int32, version string) uintptr {
	cs := NewCString(version)
	defer cs.Free()
	return CallProc(ReadVtableSlot(c.ptr, 5), c.ptr, uintptr(pipe), uintptr(user), cs.Ptr())
}

// Slot 9
func (c *ISteamClient) GetISteamUtils(pipe int32, version string) uintptr {
	cs := NewCString(version)
	defer cs.Free()
	return CallProc(ReadVtableSlot(c.ptr, 9), c.ptr, uintptr(pipe), cs.Ptr())
}

// Slot 13
func (c *ISteamClient) GetISteamUserStats(pipe, user int32, version string) uintptr {
	cs := NewCString(version)
	defer cs.Free()
	return CallProc(ReadVtableSlot(c.ptr, 13), c.ptr, uintptr(pipe), uintptr(user), cs.Ptr())
}

// Slot 23
func (c *ISteamClient) BShutdownIfAllPipesClosed() bool {
	ret := CallProc(ReadVtableSlot(c.ptr, 23), c.ptr)
	return ret != 0
}

// Slot 15
func (c *ISteamClient) GetISteamApps(pipe, user int32, version string) uintptr {
	cs := NewCString(version)
	defer cs.Free()
	return CallProc(ReadVtableSlot(c.ptr, 15), c.ptr, uintptr(pipe), uintptr(user), cs.Ptr())
}
