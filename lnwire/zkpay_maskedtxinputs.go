package lnwire

import "io"

// ZkPayMaskedTxInputs contains the paytoken the merchan sends to the Customer.
type ZkPayMaskedTxInputs struct {
	// PayToken is given to the Customer by the Merchant, to allow the
	// Customer to make a new payment on the channel.
	MaskedTxInputs ZkMsgType
}

// A compile time check to ensure Ping implements the lnwire.Message interface.
var _ Message = (*ZkPayMaskedTxInputs)(nil)

// Decode deserializes a serialized Ping message stored in the passed io.Reader
// observing the specified protocol version.
//
// This is part of the lnwire.Message interface.
func (p *ZkPayMaskedTxInputs) Decode(r io.Reader, pver uint32) error {
	return ReadElements(r, &p.MaskedTxInputs)
}

// Encode serializes the target Ping into the passed io.Writer observing the
// protocol version specified.
//
// This is part of the lnwire.Message interface.
func (p *ZkPayMaskedTxInputs) Encode(w io.Writer, pver uint32) error {
	return WriteElements(w, p.MaskedTxInputs)
}

// MsgType returns the integer uniquely identifying this message type on the
// wire.
//
// This is part of the lnwire.Message interface.
func (p *ZkPayMaskedTxInputs) MsgType() MessageType {
	return ZkMsgPayMaskedTxInputs
}

// MaxPayloadLength returns the maximum allowed payload size for a Ping
// complete message observing the specified protocol version.
//
// This is part of the lnwire.Message interface.
func (p ZkPayMaskedTxInputs) MaxPayloadLength(uint32) uint32 {
	return 65532
}
