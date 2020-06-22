package lnwire

import "io"

// ZkPayNonce defines a message which is sent by peers periodically to determine if
// the connection is still valid. Each ping message carries the number of bytes
// to pad the pong response with, and also a number of bytes to be ignored at
// the end of the ping message (which is padding).
type ZkPayNonce struct {
	// Payment contains the payment from generatePaymentProof
	SessionID     ZkMsgType
	Justification ZkMsgType
	StateNonce    ZkMsgType
	Amount        ZkMsgType
	RevLockCom    ZkMsgType
}

// A compile time check to ensure Ping implements the lnwire.Message interface.
var _ Message = (*ZkPayNonce)(nil)

// Decode deserializes a serialized Ping message stored in the passed io.Reader
// observing the specified protocol version.
//
// This is part of the lnwire.Message interface.
func (p *ZkPayNonce) Decode(r io.Reader, pver uint32) error {
	return ReadElements(r,
		&p.SessionID,
		&p.Justification,
		&p.StateNonce,
		&p.Amount,
		&p.RevLockCom)
}

// Encode serializes the target Ping into the passed io.Writer observing the
// protocol version specified.
//
// This is part of the lnwire.Message interface.
func (p *ZkPayNonce) Encode(w io.Writer, pver uint32) error {
	return WriteElements(w,
		p.SessionID,
		p.Justification,
		p.StateNonce,
		p.Amount,
		p.RevLockCom)
}

// MsgType returns the integer uniquely identifying this message type on the
// wire.
//
// This is part of the lnwire.Message interface.
func (p *ZkPayNonce) MsgType() MessageType {
	return ZkMsgPayNonce
}

// MaxPayloadLength returns the maximum allowed payload size for a Ping
// complete message observing the specified protocol version.
//
// This is part of the lnwire.Message interface.
func (p ZkPayNonce) MaxPayloadLength(uint32) uint32 {
	return 65532
}
