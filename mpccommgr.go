package lnd

import (
	"fmt"
	"github.com/lightningnetwork/lnd/lnwire"
	"unsafe"
	"github.com/lightningnetwork/lnd/pointer"
)

func Send(msg []byte, pPtr unsafe.Pointer) error {
	fmt.Println("Do Send: ", string(msg))
	fmt.Println(unsafe.Pointer(pPtr))
	p := unsafe.Pointer(pPtr)
	fmt.Println("Cast went well1:", p)
	pe := pointer.Restore(p).(*peer)
	fmt.Println("Cast went well2:", pe)
	mpcMsg := lnwire.ZkMPC{Data: msg}
	return pe.SendMessage(false, &mpcMsg)
}

func Receive(pPtr unsafe.Pointer) []byte {
	test := []byte("do something")
	fmt.Println("Got into lnd code: ", string(test))
	p := unsafe.Pointer(pPtr)
	fmt.Println("Cast went well1: ", p)
	pe := pointer.Restore(p).(*peer)
	fmt.Println("Cast went well2:", pe)
	msg := <-pe.chanMpcMsgs
	fmt.Println("message received: ", msg)
	return msg.Data
}
