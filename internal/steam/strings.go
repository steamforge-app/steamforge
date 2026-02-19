package steam

import (
	"runtime"
	"unsafe"
)

// CString allocates a null-terminated C string from a Go string.
// The caller must call Free() on the returned CString when done.
type CString struct {
	ptr    unsafe.Pointer
	pinner runtime.Pinner
}

// NewCString creates a C string from a Go string.
// The memory is pinned to prevent GC from moving it.
func NewCString(s string) *CString {
	// Allocate byte slice with null terminator
	buf := make([]byte, len(s)+1)
	copy(buf, s)
	buf[len(s)] = 0

	cs := &CString{
		ptr: unsafe.Pointer(&buf[0]),
	}
	cs.pinner.Pin(&buf[0])
	return cs
}

// Ptr returns the uintptr of the C string for passing to syscalls.
func (c *CString) Ptr() uintptr {
	return uintptr(c.ptr)
}

// Free releases the pinned memory.
func (c *CString) Free() {
	c.pinner.Unpin()
}

// GoString converts a C string pointer to a Go string.
// Returns empty string if ptr is 0.
func GoString(ptr uintptr) string {
	if ptr == 0 {
		return ""
	}

	// Find null terminator
	p := unsafe.Pointer(ptr)
	length := 0
	for {
		b := *(*byte)(unsafe.Pointer(uintptr(p) + uintptr(length)))
		if b == 0 {
			break
		}
		length++
		if length > 4096 {
			break // safety limit
		}
	}

	if length == 0 {
		return ""
	}

	return string(unsafe.Slice((*byte)(p), length))
}
