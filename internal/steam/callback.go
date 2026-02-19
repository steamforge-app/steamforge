package steam

import (
	"sync"
	"unsafe"
)

type CallbackHandler func(paramPtr uintptr, paramSize int32)

type CallbackDispatcher struct {
	lib      Library
	pipe     int32
	getCallbackFn   uintptr
	freeCallbackFn  uintptr

	mu       sync.RWMutex
	handlers map[int32][]CallbackHandler
}

func NewCallbackDispatcher(lib Library, pipe int32) (*CallbackDispatcher, error) {
	getCallback, err := lib.FindProc("Steam_BGetCallback")
	if err != nil {
		return nil, err
	}
	freeCallback, err := lib.FindProc("Steam_FreeLastCallback")
	if err != nil {
		return nil, err
	}

	return &CallbackDispatcher{
		lib:            lib,
		pipe:           pipe,
		getCallbackFn:  getCallback,
		freeCallbackFn: freeCallback,
		handlers:       make(map[int32][]CallbackHandler),
	}, nil
}

func (d *CallbackDispatcher) Register(callbackID int32, handler CallbackHandler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers[callbackID] = append(d.handlers[callbackID], handler)
}

func (d *CallbackDispatcher) RegisterOne(callbackID int32, handler CallbackHandler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers[callbackID] = []CallbackHandler{handler}
}

func (d *CallbackDispatcher) Poll() {
	var msg CallbackMessage
	var call int32

	for {
		ret := CallProc(d.getCallbackFn,
			uintptr(d.pipe),
			uintptr(unsafe.Pointer(&msg)),
			uintptr(unsafe.Pointer(&call)),
		)

		if ret == 0 {
			break
		}

		d.mu.RLock()
		handlers := d.handlers[msg.Callback]
		d.mu.RUnlock()

		for _, h := range handlers {
			h(msg.ParamPtr, msg.ParamSize)
		}

		CallProc(d.freeCallbackFn, uintptr(d.pipe))
	}
}
