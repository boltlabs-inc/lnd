package lnwire

import "io"

// ZkEstablishAccept is the first msg sent by the customer to open a zkchannel
type ZkEstablishAccept struct {
	// Payment contains the payment from generatePaymentProof
	ToSelfDelay   ZkMsgType
	MerchPayoutPk ZkMsgType
	MerchChildPk  ZkMsgType
	ChannelState  ZkMsgType
}

// A compile time check to ensure Ping implements the lnwire.Message interface.
var _ Message = (*ZkEstablishAccept)(nil)

// Decode deserializes a serialized Ping message stored in the passed io.Reader
// observing the specified protocol version.
//
// This is part of the lnwire.Message interface.
func (p *ZkEstablishAccept) Decode(r io.Reader, pver uint32) error {
	return ReadElements(r,
		&p.ToSelfDelay,
		&p.MerchPayoutPk,
		&p.MerchChildPk,
		&p.ChannelState)
}

// Encode serializes the target Ping into the passed io.Writer observing the
// protocol version specified.
//
// This is part of the lnwire.Message interface.
func (p *ZkEstablishAccept) Encode(w io.Writer, pver uint32) error {
	return WriteElements(w,
		p.ToSelfDelay,
		p.MerchPayoutPk,
		p.MerchChildPk,
		p.ChannelState)
}

// MsgType returns the integer uniquely identifying this message type on the
// wire.
//
// This is part of the lnwire.Message interface.
func (p *ZkEstablishAccept) MsgType() MessageType {
	return ZkMsgEstablishAccept
}

// MaxPayloadLength returns the maximum allowed payload size for a Ping
// complete message observing the specified protocol version.
//
// This is part of the lnwire.Message interface.
func (p ZkEstablishAccept) MaxPayloadLength(uint32) uint32 {
	return 65532
}
