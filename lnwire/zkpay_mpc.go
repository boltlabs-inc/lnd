package lnwire

import "io"

// ZkPayMPC contains the close token the merchant sends to the customer,
// after having verified the customer's payment proof.
// The close token is given to the Customer by the Merchant, to allow the
// Customer to close the channel unilaterally, paying out to each party
// the channel balances specified in the Customer's original openChannel
// request.
type ZkPayMPC struct {
	// CloseToken is given to the Customer by the Merchant, to allow the
	// Customer to close the channel unilaterally, paying out to each party
	// the channel balances specified in the Customer's original openChannel
	// request.
	PayTokenMaskCom ZkChannelSigType
	RevLockCom 		ZkChannelSigType
}

// A compile time check to ensure Ping implements the lnwire.Message interface.
var _ Message = (*ZkPayMPC)(nil)

// Decode deserializes a serialized Ping message stored in the passed io.Reader
// observing the specified protocol version.
//
// This is part of the lnwire.Message interface.
func (p *ZkPayMPC) Decode(r io.Reader, pver uint32) error {
	return ReadElements(r, 
		&p.PayTokenMaskCom,
		&p.RevLockCom)
}

// Encode serializes the target Ping into the passed io.Writer observing the
// protocol version specified.
//
// This is part of the lnwire.Message interface.
func (p *ZkPayMPC) Encode(w io.Writer, pver uint32) error {
	return WriteElements(w, 
		p.PayTokenMaskCom,
		p.RevLockCom)
}

// MsgType returns the integer uniquely identifying this message type on the
// wire.
//
// This is part of the lnwire.Message interface.
func (p *ZkPayMPC) MsgType() MessageType {
	return ZkMsgPayMPC
}

// MaxPayloadLength returns the maximum allowed payload size for a Ping
// complete message observing the specified protocol version.
//
// This is part of the lnwire.Message interface.
func (p ZkPayMPC) MaxPayloadLength(uint32) uint32 {
	return 65532
}
