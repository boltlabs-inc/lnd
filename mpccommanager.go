package lnd

/*
   #include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/lightningnetwork/lnd/lnwire"
)

var (
	mutex sync.Mutex
	store = map[unsafe.Pointer]interface{}{}
)

//export send
func send(msg *C.char, length C.int, pPtr unsafe.Pointer) *C.char {
	msgBytes := C.GoBytes(unsafe.Pointer(msg), length)
	p := RestorePointer(pPtr).(*peer)
	mpcMsg := lnwire.ZkMPC{Data: msgBytes}
	err := p.SendMessage(false, &mpcMsg)
	if err != nil {
		fmt.Println(err.Error())
		return C.CString(err.Error())
	}
	fmt.Println("message sent: ", mpcMsg)
	return nil
}

//export receive
func receive(pPtr unsafe.Pointer) (*C.char, C.int, *C.char) {
	p := RestorePointer(pPtr).(*peer)
	msg := <-p.chanMpcMsgs
	fmt.Println("message received: ", msg)
	return (*C.char)(C.CBytes(msg.Data)), C.int(len(msg.Data)), nil
}

func SavePointer(v interface{}) unsafe.Pointer {
	if v == nil {
		return nil
	}

	// Generate real fake C pointer.
	// This pointer will not store any data, but will bi used for indexing purposes.
	// Since Go doest allow to cast dangling pointer to unsafe.Pointer, we do rally allocate one byte.
	// Why we need indexing, because Go doest allow C code to store pointers to Go data.
	var ptr unsafe.Pointer = C.malloc(C.size_t(1))
	if ptr == nil {
		panic("can't allocate 'cgo-pointer hack index pointer': ptr == nil")
	}

	mutex.Lock()
	store[ptr] = v
	mutex.Unlock()

	return ptr
}

func RestorePointer(ptr unsafe.Pointer) (v interface{}) {
	if ptr == nil {
		return nil
	}

	mutex.Lock()
	v = store[ptr]
	mutex.Unlock()
	return
}

func UnrefPointer(ptr unsafe.Pointer) {
	if ptr == nil {
		return
	}

	mutex.Lock()
	delete(store, ptr)
	mutex.Unlock()

	C.free(ptr)
}
