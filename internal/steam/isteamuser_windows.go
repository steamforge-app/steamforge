package steam

import "unsafe"

// GetSteamID returns the 64-bit SteamID of the logged-in user.
//
// On Windows MSVC x64, CSteamID is a non-trivial C++ class (has constructors),
// so the compiler uses a hidden return parameter: the caller passes a pointer
// in RDX where the callee writes the result, instead of returning it in RAX.
// This matches how SAM handles it (void delegate with out parameter).
func (u *ISteamUser) GetSteamID() uint64 {
	var result uint64
	CallProc(ReadVtableSlot(u.ptr, 2), u.ptr, uintptr(unsafe.Pointer(&result)))
	return result
}
