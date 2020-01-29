package lnwire

import "io"

// ZkPayToken contains the paytoken the merchan sends to the Customer.
type ZkPayToken struct {
	// PayToken is given to the Customer by the Merchant, to allow the
	// Customer to make a new payment on the channel.
	PayToken ZkChannelSigType
}

// A compile time check to ensure Ping implements the lnwire.Message interface.
var _ Message = (*ZkPayToken)(nil)

// Decode deserializes a serialized Ping message stored in the passed io.Reader
// observing the specified protocol version.
//
// This is part of the lnwire.Message interface.
func (p *ZkPayToken) Decode(r io.Reader, pver uint32) error {
	return ReadElements(r, &p.PayToken)
}

// Encode serializes the target Ping into the passed io.Writer observing the
// protocol version specified.
//
// This is part of the lnwire.Message interface.
func (p *ZkPayToken) Encode(w io.Writer, pver uint32) error {
	return WriteElements(w, p.PayToken)
}

// MsgType returns the integer uniquely identifying this message type on the
// wire.
//
// This is part of the lnwire.Message interface.
func (p *ZkPayToken) MsgType() MessageType {
	return ZkMsgPayToken
}

// MaxPayloadLength returns the maximum allowed payload size for a Ping
// complete message observing the specified protocol version.
//
// This is part of the lnwire.Message interface.
func (p ZkPayToken) MaxPayloadLength(uint32) uint32 {
	return 65532
}
