package steam

// GetSteamID returns the 64-bit SteamID of the logged-in user.
//
// On Linux (SysV ABI), CSteamID is 8 bytes and fits in a register,
// so it is returned directly in RAX regardless of constructors.
func (u *ISteamUser) GetSteamID() uint64 {
	ret := CallProc(ReadVtableSlot(u.ptr, 2), u.ptr)
	return uint64(ret)
}
