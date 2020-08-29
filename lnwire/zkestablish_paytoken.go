package lnwire

import "io"

// ZkEstablishPayToken is the first msg sent by the customer to open a zkchannel
type ZkEstablishPayToken struct {
	// Payment contains the payment from generatePaymentProof
	EscrowTxid ZkMsgType
	PayToken0  ZkMsgType
}

// A compile time check to ensure Ping implements the lnwire.Message interface.
var _ Message = (*ZkEstablishPayToken)(nil)

// Decode deserializes a serialized Ping message stored in the passed io.Reader
// observing the specified protocol version.
//
// This is part of the lnwire.Message interface.
func (p *ZkEstablishPayToken) Decode(r io.Reader, pver uint32) error {
	return ReadElements(r,
		&p.EscrowTxid,
		&p.PayToken0)
}

// Encode serializes the target Ping into the passed io.Writer observing the
// protocol version specified.
//
// This is part of the lnwire.Message interface.
func (p *ZkEstablishPayToken) Encode(w io.Writer, pver uint32) error {
	return WriteElements(w,
		p.EscrowTxid,
		p.PayToken0)
}

// MsgType returns the integer uniquely identifying this message type on the
// wire.
//
// This is part of the lnwire.Message interface.
func (p *ZkEstablishPayToken) MsgType() MessageType {
	return ZkMsgEstablishPayToken
}

// MaxPayloadLength returns the maximum allowed payload size for a Ping
// complete message observing the specified protocol version.
//
// This is part of the lnwire.Message interface.
func (p ZkEstablishPayToken) MaxPayloadLength(uint32) uint32 {
	return 65532
}
