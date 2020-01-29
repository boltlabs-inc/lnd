package lnd

import (
	"encoding/json"

	"github.com/lightningnetwork/lnd/lnpeer"
	"github.com/lightningnetwork/lnd/lnwire"
)

type zkChannelManager struct {
}

func (z *zkChannelManager) processZkPayProof(msg *lnwire.ZkPayProof, p lnpeer.Peer) {

	// // To load from rpc message
	var payment string
	err := json.Unmarshal(msg.Payment, &payment)
	_ = err

	closeTokenBytes := []byte{'d', 'u', 'm', 'm', 'y', 'y', 'y', 'y'}

	zkPayClose := lnwire.ZkPayClose{
		CloseToken: closeTokenBytes,
	}
	p.SendMessage(false, &zkPayClose)

}
