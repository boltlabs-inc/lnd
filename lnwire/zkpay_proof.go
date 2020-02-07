package lnwire

import "io"

// ZkPayProof defines a message which is sent by peers periodically to determine if
// the connection is still valid. Each ping message carries the number of bytes
// to pad the pong response with, and also a number of bytes to be ignored at
// the end of the ping message (which is padding).
type ZkPayProof struct {
	// Payment contains the payment from generatePaymentProof
	Payment ZkMsgType
}

// A compile time check to ensure Ping implements the lnwire.Message interface.
var _ Message = (*ZkPayProof)(nil)

// Decode deserializes a serialized Ping message stored in the passed io.Reader
// observing the specified protocol version.
//
// This is part of the lnwire.Message interface.
func (p *ZkPayProof) Decode(r io.Reader, pver uint32) error {
	return ReadElements(r,
		&p.Payment)
}

// Encode serializes the target Ping into the passed io.Writer observing the
// protocol version specified.
//
// This is part of the lnwire.Message interface.
func (p *ZkPayProof) Encode(w io.Writer, pver uint32) error {
	return WriteElements(w,
		p.Payment)
}

// MsgType returns the integer uniquely identifying this message type on the
// wire.
//
// This is part of the lnwire.Message interface.
func (p *ZkPayProof) MsgType() MessageType {
	return ZkMsgPayProof
}

// MaxPayloadLength returns the maximum allowed payload size for a Ping
// complete message observing the specified protocol version.
//
// This is part of the lnwire.Message interface.
func (p ZkPayProof) MaxPayloadLength(uint32) uint32 {
	return 65532
}
