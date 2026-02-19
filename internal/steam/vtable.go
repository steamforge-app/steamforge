package steam

import "unsafe"

func ptrOf(p *uint32) uintptr {
	return uintptr(unsafe.Pointer(p))
}

func ptrOfFloat32(p *float32) uintptr {
	return uintptr(unsafe.Pointer(p))
}
