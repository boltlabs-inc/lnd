package lnd

import (
	"fmt"
	"github.com/lightningnetwork/lnd/lnwire"
	"unsafe"
)

func Send(msg []byte, pPtr uintptr) error {
	fmt.Println("Do Send: ", string(msg))
	fmt.Println(unsafe.Pointer(pPtr))
	p := (*peer)(unsafe.Pointer(pPtr))
	fmt.Println("Cast went well:", p)
	mpcMsg := lnwire.ZkMPC{Data: msg}
	return p.SendMessage(false, &mpcMsg)
}

func Receive(pPtr uintptr) []byte {
	fmt.Println("Got into lnd code")
	p := (*peer)(unsafe.Pointer(pPtr))
	fmt.Println("Cast went well: ", p)
	msg := <-p.chanMpcMsgs
	fmt.Println("message received: ", msg)
	return msg.msg.Data
}
