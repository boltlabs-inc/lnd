package main

import "C"
import (
	"fmt"
	"github.com/lightningnetwork/lnd"
	"unsafe"
)

//export Send
func Send(msg *C.char, length C.int, peer unsafe.Pointer) (errStr *C.char) {
	fmt.Println("send msg: ", msg)
	fmt.Println("send peer: ", peer)
	fmt.Printf("send: %T\n", lnd.Send)
	err := lnd.Send(C.GoBytes(unsafe.Pointer(msg), length), peer)
	if err != nil {
		fmt.Println("send err: ", err)
		return C.CString(err.Error())
	}
	return nil
}

//export Receive
func Receive(peer unsafe.Pointer) (msg unsafe.Pointer, length C.int, errStr *C.char) {
	recvMsg := lnd.Receive(peer)
	fmt.Println("received: ", recvMsg)
	return C.CBytes(recvMsg), C.int(len(recvMsg)), nil
}

func main() {}
