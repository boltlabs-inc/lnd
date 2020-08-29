package lnwire

import "io"

// ZkEstablishCCloseSigned is the first msg sent by the customer to open a zkchannel
type ZkEstablishCCloseSigned struct {
	// Payment contains the payment from generatePaymentProof
	EscrowTxid   ZkMsgType
	EscrowSig    ZkMsgType
	MerchSig     ZkMsgType
	MerchTxid    ZkMsgType
	MerchPrevout ZkMsgType
}

// A compile time check to ensure Ping implements the lnwire.Message interface.
var _ Message = (*ZkEstablishCCloseSigned)(nil)

// Decode deserializes a serialized Ping message stored in the passed io.Reader
// observing the specified protocol version.
//
// This is part of the lnwire.Message interface.
func (p *ZkEstablishCCloseSigned) Decode(r io.Reader, pver uint32) error {
	return ReadElements(r,
		&p.EscrowTxid,
		&p.EscrowSig,
		&p.MerchSig,
		&p.MerchTxid,
		&p.MerchPrevout)
}

// Encode serializes the target Ping into the passed io.Writer observing the
// protocol version specified.
//
// This is part of the lnwire.Message interface.
func (p *ZkEstablishCCloseSigned) Encode(w io.Writer, pver uint32) error {
	return WriteElements(w,
		p.EscrowTxid,
		p.EscrowSig,
		p.MerchSig,
		p.MerchTxid,
		p.MerchPrevout)
}

// MsgType returns the integer uniquely identifying this message type on the
// wire.
//
// This is part of the lnwire.Message interface.
func (p *ZkEstablishCCloseSigned) MsgType() MessageType {
	return ZkMsgEstablishCCloseSigned
}

// MaxPayloadLength returns the maximum allowed payload size for a Ping
// complete message observing the specified protocol version.
//
// This is part of the lnwire.Message interface.
func (p ZkEstablishCCloseSigned) MaxPayloadLength(uint32) uint32 {
	return 65532
}
