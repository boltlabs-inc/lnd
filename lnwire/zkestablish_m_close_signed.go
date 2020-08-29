package lnwire

import "io"

// ZkEstablishMCloseSigned is the first msg sent by the customer to open a zkchannel
type ZkEstablishMCloseSigned struct {
	// Payment contains the payment from generatePaymentProof
	EscrowTxid    ZkMsgType
	CustBal       ZkMsgType
	MerchBal      ZkMsgType
	EscrowPrevout ZkMsgType
	CustPk        ZkMsgType
	CustSig       ZkMsgType
	CustClosePk   ZkMsgType
	RevLock       ZkMsgType
	FeeCC         ZkMsgType
	FeeMC         ZkMsgType
}

// A compile time check to ensure Ping implements the lnwire.Message interface.
var _ Message = (*ZkEstablishMCloseSigned)(nil)

// Decode deserializes a serialized Ping message stored in the passed io.Reader
// observing the specified protocol version.
//
// This is part of the lnwire.Message interface.
func (p *ZkEstablishMCloseSigned) Decode(r io.Reader, pver uint32) error {
	return ReadElements(r,
		&p.EscrowTxid,
		&p.CustBal,
		&p.MerchBal,
		&p.EscrowPrevout,
		&p.CustPk,
		&p.CustSig,
		&p.CustClosePk,
		&p.RevLock,
		&p.FeeCC,
		&p.FeeMC,
	)
}

// Encode serializes the target Ping into the passed io.Writer observing the
// protocol version specified.
//
// This is part of the lnwire.Message interface.
func (p *ZkEstablishMCloseSigned) Encode(w io.Writer, pver uint32) error {
	return WriteElements(w,
		p.EscrowTxid,
		p.CustBal,
		p.MerchBal,
		p.EscrowPrevout,
		p.CustPk,
		p.CustSig,
		p.CustClosePk,
		p.RevLock,
		p.FeeCC,
		p.FeeMC,
	)
}

// MsgType returns the integer uniquely identifying this message type on the
// wire.
//
// This is part of the lnwire.Message interface.
func (p *ZkEstablishMCloseSigned) MsgType() MessageType {
	return ZkMsgEstablishMCloseSigned
}

// MaxPayloadLength returns the maximum allowed payload size for a Ping
// complete message observing the specified protocol version.
//
// This is part of the lnwire.Message interface.
func (p ZkEstablishMCloseSigned) MaxPayloadLength(uint32) uint32 {
	return 65532
}
