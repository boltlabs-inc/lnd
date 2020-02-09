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

	// Add variables to zkchannelsdb
	zkCustDB, err := zkchanneldb.SetupZkCustDB()

	custStateBytes, _ := json.Marshal(custState)
	zkchanneldb.AddCustState(zkCustDB, custStateBytes)

	channelTokenBytes, _ := json.Marshal(channelToken)
	zkchanneldb.AddCustField(zkCustDB, channelTokenBytes, "channelTokenKey")

	custSkBytes, _ := json.Marshal(custSk)
	zkchanneldb.AddCustField(zkCustDB, custSkBytes, "custSkKey")

	merchPkBytes, _ := json.Marshal(merchPk)
	zkchanneldb.AddCustField(zkCustDB, merchPkBytes, "merchPkKey")

	custBalBytes, _ := json.Marshal(custBal)
	zkchanneldb.AddCustField(zkCustDB, custBalBytes, "custBalKey")

	merchBalBytes, _ := json.Marshal(merchBal)
	zkchanneldb.AddCustField(zkCustDB, merchBalBytes, "merchBalKey")

	escrowTxidBytes, _ := json.Marshal(escrowTxid)
	zkchanneldb.AddCustField(zkCustDB, escrowTxidBytes, "escrowTxidKey")

	zkCustDB.Close()

	zkchLog.Infof("Saved custState and channelToken")

	// Convert fields into bytes
	escrowTxidBytes = []byte(escrowTxid)
	custPkBytes := []byte(custPk)

	custBalBytes = make([]byte, 8)
	binary.LittleEndian.PutUint64(custBalBytes, uint64(custBal))

	merchBalBytes = make([]byte, 8)
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

	// NOTE: For now, toSelfDelay is hardcoded
	toSelfDelay := "cf05"

	zkchLog.Info("Just received ZkEstablishOpen with length: ", len(msg.EscrowTxid))

	// Convert variables received
	escrowTxid := string(msg.EscrowTxid)
	custPk := string(msg.CustPk)

	zkchLog.Info("msg.CustBal: ", msg.CustBal)
	zkchLog.Info("msg.MerchBal: ", msg.MerchBal)

	custBal := int64(binary.LittleEndian.Uint64(msg.CustBal))
	merchBal := int64(binary.LittleEndian.Uint64(msg.MerchBal))

	zkchLog.Info("custBal =>:", custBal)
	zkchLog.Info("merchBal =>:", merchBal)

	fmt.Println("received escrow txid => ", escrowTxid)

	// TODO: If the variables are not checked, they can be saved directly, skipping the step above/

	// Add variables to zkchannelsdb
	zkMerchDB, err := zkchanneldb.SetupZkMerchDB()

	toSelfDelayBytes, _ := json.Marshal(toSelfDelay)
	zkchanneldb.AddMerchField(zkMerchDB, toSelfDelayBytes, "toSelfDelayKey")

	custPkBytes, _ := json.Marshal(custPk)
	zkchanneldb.AddMerchField(zkMerchDB, custPkBytes, "custPkKey")

	custBalBytes, _ := json.Marshal(custBal)
	zkchanneldb.AddMerchField(zkMerchDB, custBalBytes, "custBalKey")

	merchBalBytes, _ := json.Marshal(merchBal)
	zkchanneldb.AddMerchField(zkMerchDB, merchBalBytes, "merchBalKey")

	escrowTxidBytes, _ := json.Marshal(escrowTxid)
	zkchanneldb.AddMerchField(zkMerchDB, escrowTxidBytes, "escrowTxidKey")

	zkMerchDB.Close()

	// open the zkchanneldb to load merchState
	zkMerchDB, err = zkchanneldb.SetupZkMerchDB()

	// read merchState from ZkMerchDB
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

	var merchState libzkchannels.MerchState
	err = json.Unmarshal(merchStateBytes, &merchState)

	zkMerchDB.Close()

	merchClosePk := fmt.Sprintf("%v", *merchState.PayoutPk)

	// Convert fields into bytes
	merchClosePkBytes := []byte(merchClosePk)
	toSelfDelayBytes = []byte(toSelfDelay)
	zkchLog.Info("converting done")

	zkEstablishAccept := lnwire.ZkEstablishAccept{
		ToSelfDelay:   toSelfDelayBytes,
		MerchPayoutPk: merchClosePkBytes,
	}
	p.SendMessage(false, &zkEstablishAccept)

}

func (z *zkChannelManager) processZkEstablishAccept(msg *lnwire.ZkEstablishAccept, p lnpeer.Peer) {

	zkchLog.Info("Just received ZkEstablishAccept.ToSelfDelay with length: ", len(msg.ToSelfDelay))

	// open the zkchanneldb to load merchState
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

	var custState libzkchannels.CustState
	err = json.Unmarshal(custStateBytes, &custState)

	// read merchPkBytes from ZkCustDB
	var merchPkBytes []byte
	err = zkCustDB.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(zkchanneldb.CustBucket).Cursor()
		_, v := c.Seek([]byte("merchPkKey"))
		merchPkBytes = v
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	var merchPk string
	err = json.Unmarshal(merchPkBytes, &merchPk)

	// read escrowTxidBytes from ZkCustDB
	var escrowTxidBytes []byte
	err = zkCustDB.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(zkchanneldb.CustBucket).Cursor()
		_, v := c.Seek([]byte("escrowTxidKey"))
		escrowTxidBytes = v
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	var escrowTxid string
	err = json.Unmarshal(escrowTxidBytes, &escrowTxid)

	// read custBalBytes from ZkCustDB
	var custBalBytes []byte
	err = zkCustDB.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(zkchanneldb.CustBucket).Cursor()
		_, v := c.Seek([]byte("custBalKey"))
		custBalBytes = v
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	var custBal int64
	err = json.Unmarshal(custBalBytes, &custBal)

	// read merchBalBytes from ZkCustDB
	var merchBalBytes []byte
	err = zkCustDB.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(zkchanneldb.CustBucket).Cursor()
		_, v := c.Seek([]byte("merchBalKey"))
		merchBalBytes = v
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	var merchBal int64
	err = json.Unmarshal(merchBalBytes, &merchBal)

	zkCustDB.Close()

	custSk := fmt.Sprintf("%v", custState.SkC)
	custPk := fmt.Sprintf("%v", custState.PkC)

	toSelfDelay := string(msg.ToSelfDelay)
	merchClosePk := string(msg.MerchPayoutPk)

	merchTxPreimage, err := libzkchannels.FormMerchCloseTx(escrowTxid, custPk, merchPk, merchClosePk, custBal, merchBal, toSelfDelay)

	zkchLog.Info("merch TxPreimage => ", merchTxPreimage)

	custSig, err := libzkchannels.CustomerSignMerchCloseTx(custSk, merchTxPreimage)

	zkchLog.Info("custSig on merchCloseTx=> ", custSig)

	// Convert variables to bytes before sending
	// TODO: Delete merchTxPreimageBytes if never used by merch
	// merchTxPreimageBytes := []byte(merchTxPreimage)
	custSigBytes := []byte(custSig)

	zkEstablishMCloseSigned := lnwire.ZkEstablishMCloseSigned{
		// MerchTxPreimage: merchTxPreimageBytes,
		CustSig: custSigBytes,
	}
	p.SendMessage(false, &zkEstablishMCloseSigned)

}

func (z *zkChannelManager) processZkEstablishMCloseSigned(msg *lnwire.ZkEstablishMCloseSigned, p lnpeer.Peer) {

	zkchLog.Info("Just received MCloseSigned.CustSig with length: ", len(msg.CustSig))

	// Convert variables received
	custSig := string(msg.CustSig)

	// open the zkchanneldb to load merchState
	zkMerchDB, err := zkchanneldb.SetupZkMerchDB()

	// read merchState from ZkMerchDB
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

	var merchState libzkchannels.MerchState
	err = json.Unmarshal(merchStateBytes, &merchState)

	// read escrowTxidBytes from ZkMerchDB
	var toSelfDelayBytes []byte
	err = zkMerchDB.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(zkchanneldb.MerchBucket).Cursor()
		_, v := c.Seek([]byte("toSelfDelayKey"))
		toSelfDelayBytes = v
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	var toSelfDelay string
	err = json.Unmarshal(toSelfDelayBytes, &toSelfDelay)

	// read escrowTxidBytes from ZkMerchDB
	var escrowTxidBytes []byte
	err = zkMerchDB.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(zkchanneldb.MerchBucket).Cursor()
		_, v := c.Seek([]byte("escrowTxidKey"))
		escrowTxidBytes = v
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	var escrowTxid string
	err = json.Unmarshal(escrowTxidBytes, &escrowTxid)

	// read custPkBytes from ZkMerchDB
	var custPkBytes []byte
	err = zkMerchDB.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(zkchanneldb.MerchBucket).Cursor()
		_, v := c.Seek([]byte("custPkKey"))
		custPkBytes = v
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	var custPk string
	err = json.Unmarshal(custPkBytes, &custPk)

	// read custBalBytes from ZkMerchDB
	var custBalBytes []byte
	err = zkMerchDB.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(zkchanneldb.MerchBucket).Cursor()
		_, v := c.Seek([]byte("custBalKey"))
		custBalBytes = v
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	var custBal int64
	err = json.Unmarshal(custBalBytes, &custBal)

	// read merchBalBytes from ZkCustDB
	var merchBalBytes []byte
	err = zkMerchDB.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(zkchanneldb.MerchBucket).Cursor()
		_, v := c.Seek([]byte("merchBalKey"))
		merchBalBytes = v
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	var merchBal int64
	err = json.Unmarshal(merchBalBytes, &merchBal)

	zkMerchDB.Close()

	merchPk := fmt.Sprintf("%v", *merchState.PkM)
	merchSk := fmt.Sprintf("%v", *merchState.SkM)
	merchClosePk := fmt.Sprintf("%v", *merchState.PayoutPk)

	signedMerchCloseTx, merchTxid, merchPrevout, err := libzkchannels.MerchantSignMerchCloseTx(escrowTxid, custPk, merchPk, merchClosePk, custBal, merchBal, toSelfDelay, custSig, merchSk)
	_ = signedMerchCloseTx
	_ = merchTxid
	_ = merchPrevout

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
