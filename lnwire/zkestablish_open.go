package lnwire

import "io"

// ZkEstablishOpen is the first msg sent by the customer to open a zkchannel
type ZkEstablishOpen struct {
	// Payment contains the payment from generatePaymentProof
	EscrowTxid ZkMsgType
	CustPk     ZkMsgType
	CustBal    ZkMsgType
	MerchBal   ZkMsgType
}

// A compile time check to ensure Ping implements the lnwire.Message interface.
var _ Message = (*ZkEstablishOpen)(nil)

// Decode deserializes a serialized Ping message stored in the passed io.Reader
// observing the specified protocol version.
//
// This is part of the lnwire.Message interface.
func (p *ZkEstablishOpen) Decode(r io.Reader, pver uint32) error {
	return ReadElements(r,
		&p.EscrowTxid,
		&p.CustPk)
}

// Encode serializes the target Ping into the passed io.Writer observing the
// protocol version specified.
//
// This is part of the lnwire.Message interface.
func (p *ZkEstablishOpen) Encode(w io.Writer, pver uint32) error {
	return WriteElements(w,
		p.EscrowTxid,
		p.CustPk)
}

// MsgType returns the integer uniquely identifying this message type on the
// wire.
//
// This is part of the lnwire.Message interface.
func (p *ZkEstablishOpen) MsgType() MessageType {
	return ZkMsgEstablishOpen
}

// MaxPayloadLength returns the maximum allowed payload size for a Ping
// complete message observing the specified protocol version.
//
// This is part of the lnwire.Message interface.
func (p ZkEstablishOpen) MaxPayloadLength(uint32) uint32 {
	return 65532
}
