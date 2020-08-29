package lnwire

import "io"

// ZkEstablishStateValidated is the first msg sent by the customer to open a zkchannel
type ZkEstablishStateValidated struct {
	// Payment contains the payment from generatePaymentProof
	EscrowTxid ZkMsgType
	SuccessMsg ZkMsgType
}

// A compile time check to ensure Ping implements the lnwire.Message interface.
var _ Message = (*ZkEstablishStateValidated)(nil)

// Decode deserializes a serialized Ping message stored in the passed io.Reader
// observing the specified protocol version.
//
// This is part of the lnwire.Message interface.
func (p *ZkEstablishStateValidated) Decode(r io.Reader, pver uint32) error {
	return ReadElements(r,
		&p.EscrowTxid,
		&p.SuccessMsg)
}

// Encode serializes the target Ping into the passed io.Writer observing the
// protocol version specified.
//
// This is part of the lnwire.Message interface.
func (p *ZkEstablishStateValidated) Encode(w io.Writer, pver uint32) error {
	return WriteElements(w,
		p.EscrowTxid,
		p.SuccessMsg)
}

// MsgType returns the integer uniquely identifying this message type on the
// wire.
//
// This is part of the lnwire.Message interface.
func (p *ZkEstablishStateValidated) MsgType() MessageType {
	return ZkMsgEstablishStateValidated
}

// MaxPayloadLength returns the maximum allowed payload size for a Ping
// complete message observing the specified protocol version.
//
// This is part of the lnwire.Message interface.
func (p ZkEstablishStateValidated) MaxPayloadLength(uint32) uint32 {
	return 65532
}
