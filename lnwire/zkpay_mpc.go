package lnwire

import "io"

// ZkPayMPC contains the PayTokenMaskCom
type ZkPayMPC struct {
	SessionID       ZkMsgType
	PayTokenMaskCom ZkMsgType
}

// A compile time check to ensure Ping implements the lnwire.Message interface.
var _ Message = (*ZkPayMPC)(nil)

// Decode deserializes a serialized Ping message stored in the passed io.Reader
// observing the specified protocol version.
//
// This is part of the lnwire.Message interface.
func (p *ZkPayMPC) Decode(r io.Reader, pver uint32) error {
	return ReadElements(r,
		&p.SessionID,
		&p.PayTokenMaskCom,
	)
}

// Encode serializes the target Ping into the passed io.Writer observing the
// protocol version specified.
//
// This is part of the lnwire.Message interface.
func (p *ZkPayMPC) Encode(w io.Writer, pver uint32) error {
	return WriteElements(w,
		p.SessionID,
		p.PayTokenMaskCom,
	)
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
