package lnd

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/boltdb/bolt"
	"github.com/lightningnetwork/lnd/libzkchannels"
	"github.com/lightningnetwork/lnd/lnpeer"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/zkchanneldb"
)

type zkChannelManager struct {
}

func (z *zkChannelManager) initZkEstablish(merchPubKey []byte, custBalance int64, merchBalance int64, p lnpeer.Peer) {

	// open the zkchanneldb to load custState
	zkCustDB, err := zkchanneldb.SetupZkCustDB()

	// read custState from ZkCustDB
	var custStateBytes []byte
	err = zkCustDB.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(zkchanneldb.CustBucket).Cursor()
		_, v := c.Seek([]byte("custStateKey"))
		custStateBytes = v
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	zkCustDB.Close()

	// // Below is if merchPubKey is to be loaded from json
	// // Right now it's passed in as a command line argument for openzkchannel
	// // Load channel token provided by Merchant
	// merchPubKeyFile, err := ioutil.ReadFile("../merchPubKey.json")
	// fmt.Println("\n\nread merchPubKey.json file as string =", string(merchPubKeyFile))
	// var merchPubKey []byte
	// err = json.Unmarshal(merchPubKeyFile, &merchPubKey)

	// NOTE: tx will be replaced with update of libzkchannels
	tx := "{\"init_cust_bal\":100,\"init_merch_bal\":100,\"escrow_index\":0,\"merch_index\":0,\"escrow_txid\":\"f6f77d4ff12bbcefd3213aaf2aa61d29b8267f89c57792875dead8f9ba2f303d\",\"escrow_prevout\":\"1a4946d25e4699c69d38899858f1173c5b7ab4e89440cf925205f4f244ce0725\",\"merch_txid\":\"42840a4d79fe3259007d8667b5c377db0d6446c20a8b490cfe9973582e937c3d\",\"merch_prevout\":\"e9af3d3478ee5bab17f97cb9da3e5c60104dec7f777f8a529a0d7ae960866449\"}"

	channelToken, custState, err := libzkchannels.InitCustomer(fmt.Sprintf("\"%v\"", merchPubKey), tx, "cust")
	// channelToken, custState, err := libzkchannels.InitCustomer(fmt.Sprintf("\"%v\"", merchPubKey), 100, 100, "cust")

	_, _ = channelToken, custState
	zkchLog.Infof("Generated channelToken and custState")

	// zkDB add custState, channelToken, and channelState
	zkCustDB, err = zkchanneldb.SetupZkCustDB()

	custStateBytes, _ = json.Marshal(custState)
	zkchanneldb.AddCustState(zkCustDB, custStateBytes)

	channelTokenBytes, _ := json.Marshal(channelToken)
	zkchanneldb.AddCustChannelToken(zkCustDB, channelTokenBytes)

	// channelStateBytes, _ := json.Marshal(channelState)
	// zkchanneldb.AddCustChannelState(zkCustDB, channelStateBytes)

	zkCustDB.Close()

	zkchLog.Infof("Saved custState and channelToken")

	// paymentBytes, err := json.Marshal(payment)
	// zkchLog.Info("\nlength of 'payment': ", len(payment))

	// zkpayproof := lnwire.ZkPayProof{
	// 	Payment: paymentBytes,
	// }

	// TEMPORARY dummy message
	paymentBytes := []byte{'d', 'u', 'm', 'm', 'y'}

	zkEstablishOpen := lnwire.ZkEstablishOpen{
		Payment: paymentBytes,
	}

	p.SendMessage(false, &zkEstablishOpen)
	// peer.SendMessage(false, &openZkChan)

}

func (z *zkChannelManager) processZkEstablishOpen(msg *lnwire.ZkEstablishOpen, p lnpeer.Peer) {

	zkchLog.Info("Just received ZkEstablishOpen with length: ", len(msg.Payment))

	// // To load from rpc message
	var payment string
	err := json.Unmarshal(msg.Payment, &payment)
	_ = err

	// TEMPORARY DUMMY MESSAGE
	paymentBytes := []byte{'d', 'u', 'm', 'm', 'y'}

	zkEstablishAccept := lnwire.ZkEstablishAccept{
		Payment: paymentBytes,
	}
	p.SendMessage(false, &zkEstablishAccept)

}

func (z *zkChannelManager) processZkEstablishAccept(msg *lnwire.ZkEstablishAccept, p lnpeer.Peer) {

	zkchLog.Info("Just received ZkEstablishAccept with length: ", len(msg.Payment))

	// // To load from rpc message
	var payment string
	err := json.Unmarshal(msg.Payment, &payment)
	_ = err

	// ******** TODO ********
	// FormEscrowTx()

	// HandleCustTransactions(...)
	// // requires that the <escrow-tx> and <merch-close-tx> are already formed (but not signed)
	// (A) Customer calls FormCustCloseTxs(...) - form the <cust-close-from-escrow-tx> and <cust-close-from-merch-close-tx>
	// (B) Customer calls SignCustCloseTxs(txInfo, channelToken, channelState, custState)
	// 	channelToken, custState
	// (C) Customer calls SignMerchCloseTx(...) - sign the <merch-close-tx>

	// ******** TODO ********
	// Customer sends 1 signature on <merch-close-tx>

	// // TEMPORARY DUMMY MESSAGE
	// signatureBytes := []byte{'d', 'u', 'm', 'm', 'y', 'y', 'y', 'y'}
	// _ = signatureBytes
	// // zkEstablishCustSig := lnwire.ZkEstablishCustSig{
	// // 	Signature: signatureBytes,
	// // }
	// // p.SendMessage(false, &zkEstablishCustSig)

	// TEMPORARY DUMMY MESSAGE
	paymentBytes := []byte{'d', 'u', 'm', 'm', 'y'}

	zkEstablishMCloseSigned := lnwire.ZkEstablishMCloseSigned{
		Payment: paymentBytes,
	}
	p.SendMessage(false, &zkEstablishMCloseSigned)

}

func (z *zkChannelManager) processZkEstablishMCloseSigned(msg *lnwire.ZkEstablishMCloseSigned, p lnpeer.Peer) {

	zkchLog.Info("Just received MCloseSigned with length: ", len(msg.Payment))

	// // To load from rpc message
	var payment string
	err := json.Unmarshal(msg.Payment, &payment)
	_ = err

	// TEMPORARY DUMMY MESSAGE
	paymentBytes := []byte{'d', 'u', 'm', 'm', 'y'}
	zkEstablishCCloseSigned := lnwire.ZkEstablishCCloseSigned{
		Payment: paymentBytes,
	}
	p.SendMessage(false, &zkEstablishCCloseSigned)

}

func (z *zkChannelManager) processZkEstablishCCloseSigned(msg *lnwire.ZkEstablishCCloseSigned, p lnpeer.Peer) {

	zkchLog.Info("Just received CCloseSigned with length: ", len(msg.Payment))

	// // To load from rpc message
	var payment string
	err := json.Unmarshal(msg.Payment, &payment)
	_ = err

	// TEMPORARY DUMMY MESSAGE
	paymentBytes := []byte{'d', 'u', 'm', 'm', 'y'}
	zkEstablishFundingLocked := lnwire.ZkEstablishFundingLocked{
		Payment: paymentBytes,
	}
	p.SendMessage(false, &zkEstablishFundingLocked)

}

func (z *zkChannelManager) processZkEstablishFundingLocked(msg *lnwire.ZkEstablishFundingLocked, p lnpeer.Peer) {

	zkchLog.Info("Just received FundingLocked with length: ", len(msg.Payment))

	// // To load from rpc message
	var payment string
	err := json.Unmarshal(msg.Payment, &payment)
	_ = err

	// TEMPORARY DUMMY MESSAGE
	paymentBytes := []byte{'d', 'u', 'm', 'm', 'y'}
	zkEstablishFundingConfirmed := lnwire.ZkEstablishFundingConfirmed{
		Payment: paymentBytes,
	}
	p.SendMessage(false, &zkEstablishFundingConfirmed)

}

func (z *zkChannelManager) processZkEstablishFundingConfirmed(msg *lnwire.ZkEstablishFundingConfirmed, p lnpeer.Peer) {

	zkchLog.Info("Just received FundingConfirmed with length: ", len(msg.Payment))

	// // To load from rpc message
	var payment string
	err := json.Unmarshal(msg.Payment, &payment)
	_ = err

	// TEMPORARY DUMMY MESSAGE
	paymentBytes := []byte{'d', 'u', 'm', 'm', 'y'}
	zkEstablishCustActivated := lnwire.ZkEstablishCustActivated{
		Payment: paymentBytes,
	}
	p.SendMessage(false, &zkEstablishCustActivated)

}

func (z *zkChannelManager) processZkEstablishCustActivated(msg *lnwire.ZkEstablishCustActivated, p lnpeer.Peer) {

	zkchLog.Info("Just received CustActivated with length: ", len(msg.Payment))

	// // To load from rpc message
	var payment string
	err := json.Unmarshal(msg.Payment, &payment)
	_ = err

	// TEMPORARY DUMMY MESSAGE
	paymentBytes := []byte{'d', 'u', 'm', 'm', 'y'}
	zkEstablishPayToken := lnwire.ZkEstablishPayToken{
		Payment: paymentBytes,
	}
	p.SendMessage(false, &zkEstablishPayToken)

}

func (z *zkChannelManager) processZkEstablishPayToken(msg *lnwire.ZkEstablishPayToken, p lnpeer.Peer) {

	zkchLog.Info("Just received PayToken with length: ", len(msg.Payment))

	// // To load from rpc message
	var payment string
	err := json.Unmarshal(msg.Payment, &payment)
	_ = err

	// // TEMPORARY DUMMY MESSAGE
	// paymentBytes := []byte{'d', 'u', 'm', 'm', 'y'}
	// zkEstablish := lnwire.ZkEstablish{
	// 	Payment: paymentBytes,
	// }
	// p.SendMessage(false, &zkEstablish)

}

func (z *zkChannelManager) processZkPayProof(msg *lnwire.ZkPayProof, p lnpeer.Peer) {

	zkchLog.Info("Just received ZkPayProof with length: ", len(msg.Payment))

	// // To load from rpc message
	var payment string
	err := json.Unmarshal(msg.Payment, &payment)
	_ = err

	// TEMPORARY DUMMY MESSAGE
	closeTokenBytes := []byte{'d', 'u', 'm', 'm', 'y', 'y', 'y', 'y'}

	zkPayClose := lnwire.ZkPayClose{
		CloseToken: closeTokenBytes,
	}
	p.SendMessage(false, &zkPayClose)

}

func (z *zkChannelManager) processZkPayClose(msg *lnwire.ZkPayClose, p lnpeer.Peer) {

	zkchLog.Info("Just received ZkPayClose with length: ", len(msg.CloseToken))

	// TEMPORARY dummy message
	revokeTokenBytes := []byte{'d', 'u', 'm', 'm', 'y'}

	zkPayRevoke := lnwire.ZkPayRevoke{
		RevokeToken: revokeTokenBytes,
	}
	p.SendMessage(false, &zkPayRevoke)

}

func (z *zkChannelManager) processZkPayRevoke(msg *lnwire.ZkPayRevoke, p lnpeer.Peer) {
	zkchLog.Info("Just received ZkPayRevoke with length: ", len(msg.RevokeToken))

	// TEMPORARY dummy message
	payTokenBytes := []byte{'d', 'u', 'm', 'm', 'y'}

	zkPayToken := lnwire.ZkPayToken{
		PayToken: payTokenBytes,
	}
	p.SendMessage(false, &zkPayToken)
}

func (z *zkChannelManager) processZkPayToken(msg *lnwire.ZkPayToken, p lnpeer.Peer) {
	zkchLog.Info("Just received ZkPayToken with length: ", len(msg.PayToken))
}
