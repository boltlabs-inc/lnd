package lnwire

import "io"

// ZkPayMPCResult contains the success message from the Customer of whether
// the MPC protocol was successful
type ZkPayMPCResult struct {
	SessionID ZkMsgType
	Success   ZkMsgType
}

// A compile time check to ensure Ping implements the lnwire.Message interface.
var _ Message = (*ZkPayMPCResult)(nil)

// Decode deserializes a serialized Ping message stored in the passed io.Reader
// observing the specified protocol version.
//
// This is part of the lnwire.Message interface.
func (p *ZkPayMPCResult) Decode(r io.Reader, pver uint32) error {
	return ReadElements(r,
		&p.SessionID,
		&p.Success)
}

// Encode serializes the target Ping into the passed io.Writer observing the
// protocol version specified.
//
// This is part of the lnwire.Message interface.
func (p *ZkPayMPCResult) Encode(w io.Writer, pver uint32) error {
	return WriteElements(w,
		p.SessionID,
		p.Success)
}

// MsgType returns the integer uniquely identifying this message type on the
// wire.
//
// This is part of the lnwire.Message interface.
func (p *ZkPayMPCResult) MsgType() MessageType {
	return ZkMsgPayMPCResult
}

// MaxPayloadLength returns the maximum allowed payload size for a Ping
// complete message observing the specified protocol version.
//
// This is part of the lnwire.Message interface.
func (p ZkPayMPCResult) MaxPayloadLength(uint32) uint32 {
	return 65532
}
