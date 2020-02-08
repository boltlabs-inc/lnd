package lnd

import (
	"encoding/binary"
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

func (z *zkChannelManager) initZkEstablish(merchPubKey string, custBalance int64, merchBalance int64, p lnpeer.Peer) {

	inputSats := int64(10000)
	custBal := int64(9000)
	merchBal := int64(100)

	channelToken, custState, err := libzkchannels.InitCustomer(fmt.Sprintf("\"%v\"", merchPubKey), custBal, merchBal, "cust")
	_ = err
	// assert.Nil(t, err)

	cust_utxo_txid := "f4df16149735c2963832ccaa9627f4008a06291e8b932c2fc76b3a5d62d462e1"

	custSk := fmt.Sprintf("%v", custState.SkC)
	custPk := fmt.Sprintf("%v", custState.PkC)

	// merchSk := fmt.Sprintf("%v", *merchState.SkM)
	// merchPk := fmt.Sprintf("%v", *merchState.PkM)
	merchPk := fmt.Sprintf("%v", merchPubKey)
	// changeSk := "4157697b6428532758a9d0f9a73ce58befe3fd665797427d1c5bb3d33f6a132e"
	changePk := "037bed6ab680a171ef2ab564af25eff15c0659313df0bbfb96414da7c7d1e65882"

	// merchClosePk := fmt.Sprintf("%v", *merchState.PayoutPk)
	// toSelfDelay := "cf05"
	zkchLog.Info("custSk :=> ", custSk)
	fmt.Println("custPk :=> ", custPk)
	// fmt.Println("merchSk :=> ", merchSk)
	fmt.Println("merchPk :=> ", merchPk)
	// fmt.Println("merchClosePk :=> ", merchClosePk)

	signedEscrowTx, escrowTxid, escrowPrevout, err := libzkchannels.FormEscrowTx(cust_utxo_txid, 0, inputSats, custBal, custSk, custPk, merchPk, changePk)
	// assert.Nil(t, err)

	_ = escrowTxid
	_ = escrowPrevout
	fmt.Println("escrow txid => ", escrowTxid)
	fmt.Println("escrow prevout => ", escrowPrevout)
	fmt.Println("signedEscrowTx => ", signedEscrowTx)

	_, _ = channelToken, custState
	zkchLog.Infof("Generated channelToken and custState")

	// zkDB add custState, channelToken, and channelState
	zkCustDB, err := zkchanneldb.SetupZkCustDB()

	custStateBytes, _ := json.Marshal(custState)
	zkchanneldb.AddCustState(zkCustDB, custStateBytes)

	channelTokenBytes, _ := json.Marshal(channelToken)
	zkchanneldb.AddCustChannelToken(zkCustDB, channelTokenBytes)

	zkCustDB.Close()

	zkchLog.Infof("Saved custState and channelToken")

	// Convert fields into bytes
	escrowTxidBytes := []byte(escrowTxid)
	custPkBytes := []byte(custPk)

	custBalBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(custBalBytes, uint64(custBal))

	merchBalBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(merchBalBytes, uint64(merchBal))

	zkEstablishOpen := lnwire.ZkEstablishOpen{
		EscrowTxid: escrowTxidBytes,
		CustPk:     custPkBytes,
		CustBal:    custBalBytes,
		MerchBal:   merchBalBytes,
	}

	p.SendMessage(false, &zkEstablishOpen)

}

func (z *zkChannelManager) processZkEstablishOpen(msg *lnwire.ZkEstablishOpen, p lnpeer.Peer) {

	zkchLog.Info("Just received ZkEstablishOpen with length: ", len(msg.EscrowTxid))

	// // Variables to save
	// escrowTxid := string(msg.EscrowTxid)
	// custPk := string(msg.CustPk)
	// custBal := int64(msg.CustBal)
	// merchBal := int64(msg.MerchBal)

	// fmt.Println("received escrow txid => ", escrowTxid)

	// open the zkchanneldb to load merchState
	zkMerchDB, err := zkchanneldb.SetupZkMerchDB()

	// // read custState from ZkCustDB
	var merchStateBytes []byte
	err = zkMerchDB.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(zkchanneldb.MerchBucket).Cursor()
		_, v := c.Seek([]byte("merchStateKey"))
		merchStateBytes = v
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	zkMerchDB.Close()

	zkchLog.Info("zkMerchDB closed")

	var merchState libzkchannels.MerchState
	zkchLog.Info("libzkchannels.MerchState")
	err = json.Unmarshal(merchStateBytes, &merchState)

	if err != nil {
		log.Fatal(err)
	}

	zkchLog.Info("Unmarshal done")

	merchClosePk := fmt.Sprintf("%v", *merchState.PayoutPk)
	toSelfDelay := "cf05"

	// Convert fields into bytes
	merchClosePkBytes := []byte(merchClosePk)
	toSelfDelayBytes := []byte(toSelfDelay)
	zkchLog.Info("converting done")

	zkEstablishAccept := lnwire.ZkEstablishAccept{
		ToSelfDelay:   toSelfDelayBytes,
		MerchPayoutPk: merchClosePkBytes,
	}
	p.SendMessage(false, &zkEstablishAccept)

}

func (z *zkChannelManager) processZkEstablishAccept(msg *lnwire.ZkEstablishAccept, p lnpeer.Peer) {

	zkchLog.Info("Just received ZkEstablishAccept with length: ", len(msg.ToSelfDelay))

	// // To load from rpc message
	var toSelfDelay string
	err := json.Unmarshal(msg.ToSelfDelay, &toSelfDelay)
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
