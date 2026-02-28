package steam

import "unsafe"

type Library interface {
	FindProc(name string) (uintptr, error)
	Close() error
}

// ReadVtableSlot reads a function pointer from a C++ vtable at the given index.
// The obj parameter is a C++ object pointer obtained from Steam SDK calls.
func ReadVtableSlot(obj uintptr, index int) uintptr {
	vtable := *(*unsafe.Pointer)(unsafe.Pointer(obj))                                  //nolint:govet // C++ object pointer from Steam SDK
	return *(*uintptr)(unsafe.Add(vtable, uintptr(index)*unsafe.Sizeof(uintptr(0))))
}
