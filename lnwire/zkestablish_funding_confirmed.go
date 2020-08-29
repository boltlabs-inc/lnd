package lnwire

import "io"

// ZkEstablishFundingConfirmed is the first msg sent by the customer to open a zkchannel
type ZkEstablishFundingConfirmed struct {
	// FundingConfirmed contains the payment from generatePaymentProof
	EscrowTxid       ZkMsgType
	FundingConfirmed ZkMsgType
}

// A compile time check to ensure Ping implements the lnwire.Message interface.
var _ Message = (*ZkEstablishFundingConfirmed)(nil)

// Decode deserializes a serialized Ping message stored in the passed io.Reader
// observing the specified protocol version.
//
// This is part of the lnwire.Message interface.
func (p *ZkEstablishFundingConfirmed) Decode(r io.Reader, pver uint32) error {
	return ReadElements(r,
		&p.EscrowTxid,
		&p.FundingConfirmed)
}

// Encode serializes the target Ping into the passed io.Writer observing the
// protocol version specified.
//
// This is part of the lnwire.Message interface.
func (p *ZkEstablishFundingConfirmed) Encode(w io.Writer, pver uint32) error {
	return WriteElements(w,
		p.EscrowTxid,
		p.FundingConfirmed)
}

// MsgType returns the integer uniquely identifying this message type on the
// wire.
//
// This is part of the lnwire.Message interface.
func (p *ZkEstablishFundingConfirmed) MsgType() MessageType {
	return ZkMsgEstablishFundingConfirmed
}

// MaxPayloadLength returns the maximum allowed payload size for a Ping
// complete message observing the specified protocol version.
//
// This is part of the lnwire.Message interface.
func (p ZkEstablishFundingConfirmed) MaxPayloadLength(uint32) uint32 {
	return 65532
}
