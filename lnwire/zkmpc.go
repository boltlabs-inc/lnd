package lnwire

import "io"

// ZkMPC defines a message which is sent by peers while doing MPC
type ZkMPC struct {
	Data []byte
}

// A compile time check to ensure ZkMPC implements the lnwire.Message interface.
var _ Message = (*ZkMPC)(nil)

// Decode deserializes a serialized ZkMPC message stored in the passed io.Reader
// observing the specified protocol version.
//
// This is part of the lnwire.Message interface.
func (p *ZkMPC) Decode(r io.Reader, pver uint32) error {
	return ReadElements(r,
		&p.Data)
}

// Encode serializes the target ZkMPC into the passed io.Writer observing the
// protocol version specified.
//
// This is part of the lnwire.Message interface.
func (p *ZkMPC) Encode(w io.Writer, pver uint32) error {
	return WriteElements(w,
		p.Data)
}

// MsgType returns the integer uniquely identifying this message type on the
// wire.
//
// This is part of the lnwire.Message interface.
func (p *ZkMPC) MsgType() MessageType {
	return ZkMsgMPC
}

// MaxPayloadLength returns the maximum allowed payload size for a ZkMPC
// complete message observing the specified protocol version.
//
// This is part of the lnwire.Message interface.
func (p ZkMPC) MaxPayloadLength(uint32) uint32 {
	return 65532
}
