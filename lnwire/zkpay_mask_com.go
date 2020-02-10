package lnwire

import "io"

// ZkMaskCom contains the close token the merchant sends to the customer,
// after having verified the customer's payment proof.
// The close token is given to the Customer by the Merchant, to allow the
// Customer to close the channel unilaterally, paying out to each party
// the channel balances specified in the Customer's original openChannel
// request.
type ZkMaskCom struct {
	// CloseToken is given to the Customer by the Merchant, to allow the
	// Customer to close the channel unilaterally, paying out to each party
	// the channel balances specified in the Customer's original openChannel
	// request.
	PayTokenMaskCom ZkChannelSigType
}

// A compile time check to ensure Ping implements the lnwire.Message interface.
var _ Message = (*ZkMaskCom)(nil)

// Decode deserializes a serialized Ping message stored in the passed io.Reader
// observing the specified protocol version.
//
// This is part of the lnwire.Message interface.
func (p *ZkMaskCom) Decode(r io.Reader, pver uint32) error {
	return ReadElements(r, &p.PayTokenMaskCom)
}

// Encode serializes the target Ping into the passed io.Writer observing the
// protocol version specified.
//
// This is part of the lnwire.Message interface.
func (p *ZkMaskCom) Encode(w io.Writer, pver uint32) error {
	return WriteElements(w, p.PayTokenMaskCom)
}

// MsgType returns the integer uniquely identifying this message type on the
// wire.
//
// This is part of the lnwire.Message interface.
func (p *ZkMaskCom) MsgType() MessageType {
	return ZkMsgMaskCom
}

// MaxPayloadLength returns the maximum allowed payload size for a Ping
// complete message observing the specified protocol version.
//
// This is part of the lnwire.Message interface.
func (p ZkMaskCom) MaxPayloadLength(uint32) uint32 {
	return 65532
}
