package main

import "C"
import (
	"fmt"
	"github.com/lightningnetwork/lnd"
	"unsafe"
)


//export Send
func Send(msg *C.char, length C.int, peer uintptr) (errStr *C.char) {
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
func Receive(peer uintptr) (msg *C.char, length C.int, errStr *C.char) {
	fmt.Println("recv peer: ", peer)
	fmt.Printf("receive: %T\n", lnd.Receive)
	recvMsg := lnd.Receive(peer)
	fmt.Println("recv msg: ", recvMsg)
	return (*C.char)(unsafe.Pointer(&recvMsg[0])), C.int(len(recvMsg)), nil
}

func main() {
	err := Send(C.CString("test"), 4, 1924149968936)
	fmt.Print(err)

	msg, length, errStr := Receive(1924146176224)
	fmt.Println(msg)
	fmt.Println(length)
	fmt.Println(errStr)
}
