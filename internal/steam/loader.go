package steam

import "unsafe"

type Library interface {
	FindProc(name string) (uintptr, error)
	Close() error
}

// ReadVtableSlot reads a function pointer from a C++ vtable at the given index.
func ReadVtableSlot(obj uintptr, index int) uintptr {
	vtable := *(*uintptr)(unsafe.Pointer(obj))
	slot := vtable + uintptr(index)*unsafe.Sizeof(uintptr(0))
	return *(*uintptr)(unsafe.Pointer(slot))
}
