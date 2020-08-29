package lnwire

import "io"

// ZkEstablishInitialState is the first msg sent by the customer to open a zkchannel
type ZkEstablishInitialState struct {
	EscrowTxid    ZkMsgType
	ChannelToken  ZkMsgType
	InitCustState ZkMsgType
	InitHash      ZkMsgType
}

// A compile time check to ensure Ping implements the lnwire.Message interface.
var _ Message = (*ZkEstablishInitialState)(nil)

// Decode deserializes a serialized Ping message stored in the passed io.Reader
// observing the specified protocol version.
//
// This is part of the lnwire.Message interface.
func (p *ZkEstablishInitialState) Decode(r io.Reader, pver uint32) error {
	return ReadElements(r,
		&p.EscrowTxid,
		&p.ChannelToken,
		&p.InitCustState,
		&p.InitHash)
}

// Encode serializes the target Ping into the passed io.Writer observing the
// protocol version specified.
//
// This is part of the lnwire.Message interface.
func (p *ZkEstablishInitialState) Encode(w io.Writer, pver uint32) error {
	return WriteElements(w,
		p.EscrowTxid,
		p.ChannelToken,
		p.InitCustState,
		p.InitHash)
}

// MsgType returns the integer uniquely identifying this message type on the
// wire.
//
// This is part of the lnwire.Message interface.
func (p *ZkEstablishInitialState) MsgType() MessageType {
	return ZkMsgEstablishInitialState
}

// MaxPayloadLength returns the maximum allowed payload size for a Ping
// complete message observing the specified protocol version.
//
// This is part of the lnwire.Message interface.
func (p ZkEstablishInitialState) MaxPayloadLength(uint32) uint32 {
	return 65532
}
