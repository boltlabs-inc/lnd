package lnd

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/boltdb/bolt"
	"github.com/btcsuite/btcd/wire"
	"github.com/jinzhu/copier"
	"github.com/lightningnetwork/lnd/libzkchannels"
	"github.com/lightningnetwork/lnd/lnpeer"
	"github.com/lightningnetwork/lnd/lnwallet"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/zkchanneldb"
)

type zkChannelManager struct {
}

func (z *zkChannelManager) initZkEstablish(merchPubKey string, custBal int64, merchBal int64, p lnpeer.Peer) {

	inputSats := int64(50 * 100000000)
	cust_utxo_txid := "682f1672534e56c602f5ea227250493229d61713d48347579cc4a46389a227e2"
	custInputSk := fmt.Sprintf("\"%v\"", "5511111111111111111111111111111100000000000000000000000000000000")

	channelToken, custState, err := libzkchannels.InitCustomer(fmt.Sprintf("\"%v\"", merchPubKey), custBal, merchBal, "cust")
	if err != nil {
		log.Fatal(err)
	}

	zkchLog.Debug("Generated channelToken and custState")
	zkchLog.Debug("channelToken => ", channelToken)
	zkchLog.Debug("custState => ", custState)

	custPk := fmt.Sprintf("%v", custState.PkC)
	revLock := fmt.Sprintf("%v", custState.RevLock)

	merchPk := fmt.Sprintf("%v", merchPubKey)
	// changeSk := "4157697b6428532758a9d0f9a73ce58befe3fd665797427d1c5bb3d33f6a132e"
	changePk := "037bed6ab680a171ef2ab564af25eff15c0659313df0bbfb96414da7c7d1e65882"

	zkchLog.Debug("cust_utxo_txid :=> ", cust_utxo_txid)
	zkchLog.Debug("inputSats :=> ", inputSats)
	zkchLog.Debug("custBal :=> ", custBal)
	zkchLog.Debug("custSk :=> ", custInputSk)
	zkchLog.Debug("custPk :=> ", custPk)
	zkchLog.Debug("merchPk :=> ", merchPk)
	zkchLog.Debug("changePk :=> ", changePk)

	zkchLog.Debug("Variables going into FormEscrowTx :=> ", cust_utxo_txid, 0, inputSats, custBal, custInputSk, custPk, merchPk, changePk)

	signedEscrowTx, escrowTxid, escrowPrevout, err := libzkchannels.FormEscrowTx(cust_utxo_txid, uint32(0), inputSats, custBal, custInputSk, custPk, merchPk, changePk)
	if err != nil {
		log.Fatal(err)
	}

	zkchLog.Info("escrow txid => ", escrowTxid)
	zkchLog.Debug("escrow prevout => ", escrowPrevout)
	zkchLog.Info("signedEscrowTx => ", signedEscrowTx)

	// TODO: Write a function to handle the storing of variables in zkchanneldb
	// Add variables to zkchannelsdb
	zkCustDB, err := zkchanneldb.SetupZkCustDB()

	custStateBytes, _ := json.Marshal(custState)
	zkchanneldb.AddCustState(zkCustDB, custStateBytes)

	channelTokenBytes, _ := json.Marshal(channelToken)
	zkchanneldb.AddCustField(zkCustDB, channelTokenBytes, "channelTokenKey")

	merchPkBytes, _ := json.Marshal(merchPk)
	zkchanneldb.AddCustField(zkCustDB, merchPkBytes, "merchPkKey")

	custBalBytes, _ := json.Marshal(custBal)
	zkchanneldb.AddCustField(zkCustDB, custBalBytes, "custBalKey")

	merchBalBytes, _ := json.Marshal(merchBal)
	zkchanneldb.AddCustField(zkCustDB, merchBalBytes, "merchBalKey")

	escrowTxidBytes, _ := json.Marshal(escrowTxid)
	zkchanneldb.AddCustField(zkCustDB, escrowTxidBytes, "escrowTxidKey")

	escrowPrevoutBytes, _ := json.Marshal(escrowPrevout)
	zkchanneldb.AddCustField(zkCustDB, escrowPrevoutBytes, "escrowPrevoutKey")

	signedEscrowTxBytes, _ := json.Marshal(signedEscrowTx)
	zkchanneldb.AddCustField(zkCustDB, signedEscrowTxBytes, "signedEscrowTxKey")

	zkCustDB.Close()

	zkchLog.Debug("Saved custState and channelToken")

	// Convert fields into bytes
	escrowTxidBytes = []byte(escrowTxid)
	custPkBytes := []byte(custPk)
	escrowPrevoutBytes = []byte(escrowPrevout)
	revLockBytes := []byte(revLock)

	custBalBytes = make([]byte, 8)
	binary.LittleEndian.PutUint64(custBalBytes, uint64(custBal))

	merchBalBytes = make([]byte, 8)
	binary.LittleEndian.PutUint64(merchBalBytes, uint64(merchBal))

	zkEstablishOpen := lnwire.ZkEstablishOpen{
		EscrowTxid:    escrowTxidBytes,
		CustPk:        custPkBytes,
		EscrowPrevout: escrowPrevoutBytes,
		RevLock:       revLockBytes,
		CustBal:       custBalBytes,
		MerchBal:      merchBalBytes,
	}

	p.SendMessage(false, &zkEstablishOpen)

}

func (z *zkChannelManager) processZkEstablishOpen(msg *lnwire.ZkEstablishOpen, p lnpeer.Peer) {

	// NOTE: For now, toSelfDelay is hardcoded
	toSelfDelay := "cf05"

	zkchLog.Debug("Just received ZkEstablishOpen")

	// // Convert variables received
	// escrowTxid := string(msg.EscrowTxid)
	// custPk := string(msg.CustPk)
	// escrowPrevout := string(msg.EscrowPrevout)
	// revLock := string(msg.RevLock)

	custBal := int64(binary.LittleEndian.Uint64(msg.CustBal))
	merchBal := int64(binary.LittleEndian.Uint64(msg.MerchBal))

	zkchLog.Info("Initial Customer Balance: ", custBal)
	zkchLog.Info("Initial Merchant Balance: ", merchBal)

	// zkchLog.Debug("received escrow txid => ", escrowTxid)

	// TODO: If the variables are not checked, they can be saved directly, skipping the step above/

	// Add variables to zkchannelsdb
	zkMerchDB, err := zkchanneldb.SetupZkMerchDB()
	if err != nil {
		log.Fatal(err)
	}

	toSelfDelayBytes, _ := json.Marshal(toSelfDelay)
	zkchanneldb.AddMerchField(zkMerchDB, toSelfDelayBytes, "toSelfDelayKey")

	zkMerchDB.Close()

	// open the zkchanneldb to load merchState and channelState
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

	// read channelState from ZkMerchDB
	var channelStateBytes []byte
	err = zkMerchDB.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(zkchanneldb.MerchBucket).Cursor()
		_, v := c.Seek([]byte("channelStateKey"))
		channelStateBytes = v
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	// TODO: Since we already have channelStateBytes above, we might not need to convert it back and forth again
	var channelState libzkchannels.ChannelState
	err = json.Unmarshal(channelStateBytes, &channelState)

	zkMerchDB.Close()

	zkchLog.Debug("channelState MerchPayOutPk => ", *channelState.MerchPayOutPk)
	zkchLog.Debug("channelState MerchDisputePk => ", *channelState.MerchDisputePk)

	merchClosePk := fmt.Sprintf("%v", *merchState.PayoutPk)

	// Convert fields into bytes
	merchClosePkBytes := []byte(merchClosePk)
	toSelfDelayBytes = []byte(toSelfDelay)
	channelStateBytes, _ = json.Marshal(channelState)

	zkEstablishAccept := lnwire.ZkEstablishAccept{
		ToSelfDelay:   toSelfDelayBytes,
		MerchPayoutPk: merchClosePkBytes,
		ChannelState:  channelStateBytes,
	}
	p.SendMessage(false, &zkEstablishAccept)

}

func (z *zkChannelManager) processZkEstablishAccept(msg *lnwire.ZkEstablishAccept, p lnpeer.Peer) {

	zkchLog.Debug("Just received ZkEstablishAccept")

	toSelfDelay := string(msg.ToSelfDelay)
	merchClosePk := string(msg.MerchPayoutPk)

	var channelState libzkchannels.ChannelState
	err := json.Unmarshal(msg.ChannelState, &channelState)

	// TODO: Might not have to convert back and forth between bytes here
	// Add variables to zkchannelsdb
	zkCustDB, err := zkchanneldb.SetupZkCustDB()

	channelStateBytes, _ := json.Marshal(channelState)
	zkchanneldb.AddCustField(zkCustDB, channelStateBytes, "channelStateKey")

	zkCustDB.Close()

	// open the zkchanneldb to load custState
	zkCustDB, err = zkchanneldb.SetupZkCustDB()

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

	// read escrowPrevoutBytes from ZkCustDB
	var escrowPrevoutBytes []byte
	err = zkCustDB.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(zkchanneldb.CustBucket).Cursor()
		_, v := c.Seek([]byte("escrowPrevoutKey"))
		escrowPrevoutBytes = v
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	var escrowPrevout string
	err = json.Unmarshal(escrowPrevoutBytes, &escrowPrevout)

	zkCustDB.Close()

	custSk := fmt.Sprintf("\"%v\"", custState.SkC)
	custPk := fmt.Sprintf("%v", custState.PkC)
	custClosePk := fmt.Sprintf("%v", custState.PayoutPk)

	merchTxPreimage, err := libzkchannels.FormMerchCloseTx(escrowTxid, custPk, merchPk, merchClosePk, custBal, merchBal, toSelfDelay)

	zkchLog.Debug("merch TxPreimage => ", merchTxPreimage)

	custSig, err := libzkchannels.CustomerSignMerchCloseTx(custSk, merchTxPreimage)
	if err != nil {
		log.Fatal(err)
	}

	zkchLog.Debug("custSig on merchCloseTx=> ", custSig)

	// Convert variables to bytes before sending

	custBalBytes = make([]byte, 8)
	binary.LittleEndian.PutUint64(custBalBytes, uint64(custBal))

	merchBalBytes = make([]byte, 8)
	binary.LittleEndian.PutUint64(merchBalBytes, uint64(merchBal))

	escrowTxidBytes = []byte(escrowTxid)

	escrowPrevoutBytes = []byte(escrowPrevout)

	custPkBytes := []byte(custPk)
	custSigBytes := []byte(custSig)
	custClosePkBytes := []byte(custClosePk)

	revLock := fmt.Sprintf("%v", custState.RevLock)
	revLockBytes := []byte(revLock)

	zkEstablishMCloseSigned := lnwire.ZkEstablishMCloseSigned{
		// MerchTxPreimage: merchTxPreimageBytes,
		CustBal:       custBalBytes,
		MerchBal:      merchBalBytes,
		EscrowTxid:    escrowTxidBytes,
		EscrowPrevout: escrowPrevoutBytes,
		CustPk:        custPkBytes,
		CustSig:       custSigBytes,
		CustClosePk:   custClosePkBytes,
		RevLock:       revLockBytes,
	}
	p.SendMessage(false, &zkEstablishMCloseSigned)

}

func (z *zkChannelManager) processZkEstablishMCloseSigned(msg *lnwire.ZkEstablishMCloseSigned, p lnpeer.Peer) {

	zkchLog.Debug("Just received MCloseSigned")

	custPk := string(msg.CustPk)
	custBal := int64(binary.LittleEndian.Uint64(msg.CustBal))
	merchBal := int64(binary.LittleEndian.Uint64(msg.MerchBal))
	escrowTxid := string(msg.EscrowTxid)
	escrowPrevout := string(msg.EscrowPrevout)
	revLock := string(msg.RevLock)

	// Convert variables received
	custSig := string(msg.CustSig)
	custClosePk := string(msg.CustClosePk)

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

	zkMerchDB.Close()

	merchPk := fmt.Sprintf("%v", *merchState.PkM)
	merchSk := fmt.Sprintf("\"%v\"", *merchState.SkM)

	merchClosePk := fmt.Sprintf("%v", *merchState.PayoutPk)

	zkchLog.Debug("Variables going into MerchantSignMerchClose:", escrowTxid, custPk, merchPk, merchClosePk, custBal, merchBal, toSelfDelay, custSig, merchSk)
	signedMerchCloseTx, merchTxid, merchPrevout, err := libzkchannels.MerchantSignMerchCloseTx(escrowTxid, custPk, merchPk, merchClosePk, custBal, merchBal, toSelfDelay, custSig, merchSk)
	if err != nil {
		log.Fatal(err)
	}

	zkchLog.Info("Signed merch close tx => ", signedMerchCloseTx)
	zkchLog.Info("Merch close txid = ", merchTxid)
	zkchLog.Debug("merch prevout = ", merchPrevout)

	// Update merchState in zkchannelsdb
	zkMerchDB, err = zkchanneldb.SetupZkMerchDB()

	signedMerchCloseTxBytes, _ := json.Marshal(signedMerchCloseTx)
	zkchanneldb.AddMerchField(zkMerchDB, signedMerchCloseTxBytes, "signedMerchCloseTxKey")

	zkMerchDB.Close()

	// MERCH SIGN CUST CLOSE

	txInfo := libzkchannels.FundingTxInfo{
		EscrowTxId:    escrowTxid,
		EscrowPrevout: escrowPrevout,
		MerchTxId:     merchTxid,
		MerchPrevout:  merchPrevout,
		InitCustBal:   custBal,
		InitMerchBal:  merchBal,
	}

	zkchLog.Debug("RevLock => ", revLock)

	escrowSig, merchSig, err := libzkchannels.MerchantSignInitCustCloseTx(txInfo, revLock, custPk, custClosePk, toSelfDelay, merchState)
	// assert.Nil(t, err)
	zkchLog.Debug("escrow sig: ", escrowSig)
	zkchLog.Debug("merch sig: ", merchSig)

	// Convert variables to bytes before sending
	escrowSigBytes := []byte(escrowSig)
	merchSigBytes := []byte(merchSig)
	merchTxidBytes := []byte(merchTxid)
	merchPrevoutBytes := []byte(merchPrevout)

	zkEstablishCCloseSigned := lnwire.ZkEstablishCCloseSigned{
		EscrowSig:    escrowSigBytes,
		MerchSig:     merchSigBytes,
		MerchTxid:    merchTxidBytes,
		MerchPrevout: merchPrevoutBytes,
	}

	p.SendMessage(false, &zkEstablishCCloseSigned)

}

func (z *zkChannelManager) processZkEstablishCCloseSigned(msg *lnwire.ZkEstablishCCloseSigned, p lnpeer.Peer) {

	zkchLog.Debug("Just received CCloseSigned")

	// Convert variables received
	escrowSig := string(msg.EscrowSig)
	merchSig := string(msg.MerchSig)
	merchTxid := string(msg.MerchTxid)
	merchPrevout := string(msg.MerchPrevout)

	zkchLog.Debug("escrow sig: ", escrowSig)
	zkchLog.Debug("merch sig: ", merchSig)

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

	// read escrowPrevoutBytes from ZkCustDB
	var escrowPrevoutBytes []byte
	err = zkCustDB.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(zkchanneldb.CustBucket).Cursor()
		_, v := c.Seek([]byte("escrowPrevoutKey"))
		escrowPrevoutBytes = v
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	var escrowPrevout string
	err = json.Unmarshal(escrowPrevoutBytes, &escrowPrevout)

	// read channelStateBytes from ZkCustDB
	var channelStateBytes []byte
	err = zkCustDB.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(zkchanneldb.CustBucket).Cursor()
		_, v := c.Seek([]byte("channelStateKey"))
		channelStateBytes = v
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	var channelState libzkchannels.ChannelState
	err = json.Unmarshal(channelStateBytes, &channelState)

	zkchLog.Debug("channelState MerchPayOutPk => ", *channelState.MerchPayOutPk)
	zkchLog.Debug("channelState MerchDisputePk => ", *channelState.MerchDisputePk)

	// read channelTokenBytes from ZkCustDB
	var channelTokenBytes []byte
	err = zkCustDB.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(zkchanneldb.CustBucket).Cursor()
		_, v := c.Seek([]byte("channelTokenKey"))
		channelTokenBytes = v
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	var channelToken libzkchannels.ChannelToken
	err = json.Unmarshal(channelTokenBytes, &channelToken)

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

	txInfo := libzkchannels.FundingTxInfo{
		EscrowTxId:    escrowTxid,
		EscrowPrevout: escrowPrevout,
		MerchTxId:     merchTxid,
		MerchPrevout:  merchPrevout,
		InitCustBal:   custBal,
		InitMerchBal:  merchBal,
	}

	zkchLog.Debug("Variables going into CustomerSignInitCustCloseTx:", txInfo, channelState, channelToken, escrowSig, merchSig, custState)

	isOk, channelToken, custState, err := libzkchannels.CustomerSignInitCustCloseTx(txInfo, channelState, channelToken, escrowSig, merchSig, custState)
	if err != nil {
		log.Fatal(err)
	}

	switch isOk {
	case true:
		zkchLog.Info("Merch signature on Cust Close is valid")
	case false:
		zkchLog.Info("Merch signature on Cust Close is invalid")
	}

	zkchLog.Info("CloseEscrowTx: ", string(custState.CloseEscrowTx))
	zkchLog.Info("CloseMerchTx: ", string(custState.CloseMerchTx))

	// Add variables to zkchannelsdb
	zkCustDB, err = zkchanneldb.SetupZkCustDB()

	custStateBytes, _ = json.Marshal(custState)
	zkchanneldb.AddCustState(zkCustDB, custStateBytes)

	channelTokenBytes, _ = json.Marshal(channelToken)
	zkchanneldb.AddCustField(zkCustDB, channelTokenBytes, "channelTokenKey")

	zkCustDB.Close()

	initCustState, initHash, err := libzkchannels.CustomerGetInitialState(custState)

	initCustStateBytes, _ := json.Marshal(initCustState)
	initHashBytes := []byte(initHash)

	zkEstablishInitialState := lnwire.ZkEstablishInitialState{
		ChannelToken:  channelTokenBytes,
		InitCustState: initCustStateBytes,
		InitHash:      initHashBytes,
	}
	p.SendMessage(false, &zkEstablishInitialState)
}

func (z *zkChannelManager) processZkEstablishInitialState(msg *lnwire.ZkEstablishInitialState, p lnpeer.Peer) {

	zkchLog.Info("Just received InitialState")

	var channelToken libzkchannels.ChannelToken
	err := json.Unmarshal(msg.ChannelToken, &channelToken)
	_ = err

	var initCustState libzkchannels.InitCustState
	err = json.Unmarshal(msg.InitCustState, &initCustState)

	initHash := string(msg.InitHash)

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

	zkMerchDB.Close()

	isOk, merchState, err := libzkchannels.MerchantValidateInitialState(channelToken, initCustState, initHash, merchState)
	if err != nil {
		log.Fatal(err)
	}

	switch isOk {
	case true:
		zkchLog.Info("Customer's initial state is valid")
	case false:
		zkchLog.Info("Customer's initial state is invalid")
	}

	// Update merchState in zkchannelsdb
	zkMerchDB, err = zkchanneldb.SetupZkMerchDB()

	merchStateBytes, _ = json.Marshal(merchState)
	zkchanneldb.AddMerchState(zkMerchDB, merchStateBytes)

	zkMerchDB.Close()

	var successMsg string

	switch isOk {
	case true:
		successMsg = "Initial State Validation Successful"
	case false:
		successMsg = "Initial State Validation Unsuccessful"
	}

	zkEstablishStateValidated := lnwire.ZkEstablishStateValidated{
		SuccessMsg: []byte(successMsg),
	}
	p.SendMessage(false, &zkEstablishStateValidated)

}

func (z *zkChannelManager) processZkEstablishStateValidated(msg *lnwire.ZkEstablishStateValidated, p lnpeer.Peer, wallet *lnwallet.LightningWallet) {

	zkchLog.Debug("Just received ZkEstablishStateValidated: ", string(msg.SuccessMsg))

	// TODO: For now, we assume isOk is true
	// Add alternative path for when isOk is false

	// open the zkchanneldb to load custState
	zkCustDB, err := zkchanneldb.SetupZkCustDB()

	// read signedEscrowTxBytes from ZkCustDB
	var signedEscrowTxBytes []byte
	err = zkCustDB.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(zkchanneldb.CustBucket).Cursor()
		_, v := c.Seek([]byte("signedEscrowTxKey"))
		signedEscrowTxBytes = v
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	var signedEscrowTx string
	err = json.Unmarshal(signedEscrowTxBytes, &signedEscrowTx)

	zkCustDB.Close()

	// Broadcast escrow tx on chain
	serializedTx, err := hex.DecodeString(signedEscrowTx)
	if err != nil {
		zkchLog.Error(err)
	}

	var msgTx wire.MsgTx
	err = msgTx.Deserialize(bytes.NewReader(serializedTx))
	if err != nil {
		zkchLog.Error(err)
	}

	err = wallet.PublishTransaction(&msgTx)
	if err != nil {
		zkchLog.Error(err)
	}

	// TEMPORARY DUMMY MESSAGE
	fundingLockedBytes := []byte("Funding Locked")
	zkEstablishFundingLocked := lnwire.ZkEstablishFundingLocked{
		FundingLocked: fundingLockedBytes,
	}

	closeInitiated := false

	// Add a flag to zkchannelsdb to say that closeChannel has not been initiated.
	// This is used to prevent another payment being made
	zkCustDB, err = zkchanneldb.SetupZkCustDB()
	closeInitiatedBytes, _ := json.Marshal(closeInitiated)
	zkchanneldb.AddCustField(zkCustDB, closeInitiatedBytes, "closeInitiatedKey")
	zkCustDB.Close()

	p.SendMessage(false, &zkEstablishFundingLocked)

}

func (z *zkChannelManager) processZkEstablishFundingLocked(msg *lnwire.ZkEstablishFundingLocked, p lnpeer.Peer) {

	zkchLog.Debug("Just received FundingLocked: ", msg.FundingLocked)

	// TEMPORARY DUMMY MESSAGE
	fundingConfirmedBytes := []byte("Funding Confirmed")
	zkEstablishFundingConfirmed := lnwire.ZkEstablishFundingConfirmed{
		FundingConfirmed: fundingConfirmedBytes,
	}
	p.SendMessage(false, &zkEstablishFundingConfirmed)

}

func (z *zkChannelManager) processZkEstablishFundingConfirmed(msg *lnwire.ZkEstablishFundingConfirmed, p lnpeer.Peer) {

	zkchLog.Debug("Just received FundingConfirmed: ", string(msg.FundingConfirmed))

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

	var custState libzkchannels.CustState
	err = json.Unmarshal(custStateBytes, &custState)

	// read channelToken from ZkCustDB
	var channelTokenBytes []byte
	err = zkCustDB.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(zkchanneldb.CustBucket).Cursor()
		_, v := c.Seek([]byte("channelTokenKey"))
		channelTokenBytes = v
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	var channelToken libzkchannels.ChannelToken
	err = json.Unmarshal(channelTokenBytes, &channelToken)
	zkchLog.Debug("ActivateCustomer, channelToken =>:", channelToken)

	zkCustDB.Close()

	state, custState, err := libzkchannels.ActivateCustomer(custState)
	zkchLog.Debug("ActivateCustomer, state =>:", state)

	// Add variables to zkchannelsdb
	zkCustDB, err = zkchanneldb.SetupZkCustDB()

	custStateBytes, _ = json.Marshal(custState)
	zkchanneldb.AddCustState(zkCustDB, custStateBytes)

	stateBytes, _ := json.Marshal(state)
	zkchanneldb.AddCustField(zkCustDB, stateBytes, "stateKey")

	zkCustDB.Close()

	channelTokenBytes, _ = json.Marshal(channelToken)

	zkEstablishCustActivated := lnwire.ZkEstablishCustActivated{
		State:        stateBytes,
		ChannelToken: channelTokenBytes,
	}
	p.SendMessage(false, &zkEstablishCustActivated)

}

func (z *zkChannelManager) processZkEstablishCustActivated(msg *lnwire.ZkEstablishCustActivated, p lnpeer.Peer) {

	// To load from rpc message
	var state libzkchannels.State
	err := json.Unmarshal(msg.State, &state)
	_ = err
	zkchLog.Debug("Just received ActivateCustomer, state =>:", state)

	var channelToken libzkchannels.ChannelToken
	err = json.Unmarshal(msg.ChannelToken, &channelToken)
	zkchLog.Debug("Just received ActivateCustomer, channelToken =>:", channelToken)

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

	zkMerchDB.Close()

	payToken0, merchState, err := libzkchannels.ActivateMerchant(channelToken, state, merchState)

	// Darius: Do we save 'state' here?

	// Add variables to zkchannelsdb
	zkMerchDB, err = zkchanneldb.SetupZkMerchDB()

	merchStateBytes, _ = json.Marshal(merchState)
	zkchanneldb.AddMerchState(zkMerchDB, merchStateBytes)

	zkMerchDB.Close()

	// TEMPORARY DUMMY MESSAGE
	payToken0Bytes := []byte(payToken0)
	zkEstablishPayToken := lnwire.ZkEstablishPayToken{
		PayToken0: payToken0Bytes,
	}
	p.SendMessage(false, &zkEstablishPayToken)

}

func (z *zkChannelManager) processZkEstablishPayToken(msg *lnwire.ZkEstablishPayToken, p lnpeer.Peer) {

	payToken0 := string(msg.PayToken0)
	zkchLog.Debug("Just received PayToken0: ", payToken0)

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

	var custState libzkchannels.CustState
	err = json.Unmarshal(custStateBytes, &custState)

	zkCustDB.Close()

	custState, err = libzkchannels.ActivateCustomerFinalize(payToken0, custState)
	if err != nil {
		log.Fatal(err)
	}

	// Add variables to zkchannelsdb
	zkCustDB, err = zkchanneldb.SetupZkCustDB()

	custStateBytes, _ = json.Marshal(custState)
	zkchanneldb.AddCustState(zkCustDB, custStateBytes)

	zkCustDB.Close()

}

func (z *zkChannelManager) InitZkPay(Amount int64, p lnpeer.Peer) {

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

	// read channelState from ZkCustDB
	var channelStateBytes []byte
	err = zkCustDB.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(zkchanneldb.CustBucket).Cursor()
		_, v := c.Seek([]byte("channelStateKey"))
		channelStateBytes = v

		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	var channelState libzkchannels.ChannelState
	err = json.Unmarshal(channelStateBytes, &channelState)

	zkchLog.Debug("channelState MerchPayOutPk => ", *channelState.MerchPayOutPk)
	zkchLog.Debug("channelState MerchDisputePk => ", *channelState.MerchDisputePk)

	zkCustDB.Close()

	var oldState libzkchannels.State
	copier.Copy(&oldState, *custState.State)

	revState, newState, custState, err := libzkchannels.PreparePaymentCustomer(channelState, Amount, custState)
	if err != nil {
		log.Fatal(err)
	}

	// Add variables to zkchannelsdb
	zkCustDB, err = zkchanneldb.SetupZkCustDB()

	custStateBytes, _ = json.Marshal(custState)
	zkchanneldb.AddCustState(zkCustDB, custStateBytes)

	revStateBytes, _ := json.Marshal(revState)
	zkchanneldb.AddCustField(zkCustDB, revStateBytes, "revStateKey")

	newStateBytes, _ := json.Marshal(newState)
	zkchanneldb.AddCustField(zkCustDB, newStateBytes, "newStateKey")

	oldStateBytes, _ := json.Marshal(oldState)
	zkchanneldb.AddCustField(zkCustDB, oldStateBytes, "oldStateKey")

	amountBytes, _ := json.Marshal(Amount)
	zkchanneldb.AddCustField(zkCustDB, amountBytes, "amountKey")

	zkCustDB.Close()

	oldStateNonce := oldState.Nonce
	oldStateNonceBytes := []byte(oldStateNonce)

	// amountBytes = make([]byte, 8)
	// binary.LittleEndian.PutUint64(amountBytes, uint64(Amount))

	// TODO: Add amount
	zkpaynonce := lnwire.ZkPayNonce{
		StateNonce: oldStateNonceBytes,
		// TODO: Remove amount from msg
		// Amount:     amountBytes,
	}

	p.SendMessage(false, &zkpaynonce)

}

func (z *zkChannelManager) processZkPayNonce(msg *lnwire.ZkPayNonce, p lnpeer.Peer) {

	stateNonce := string(msg.StateNonce)
	// amount := int64(binary.LittleEndian.Uint64(msg.Amount))
	zkchLog.Debug("Just received ZkPayNonce")

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

	zkMerchDB.Close()

	payTokenMaskCom, merchState, err := libzkchannels.PreparePaymentMerchant(stateNonce, merchState)
	if err != nil {
		log.Fatal(err)
	}

	// Add variables to zkchannelsdb
	zkMerchDB, err = zkchanneldb.SetupZkMerchDB()

	merchStateBytes, _ = json.Marshal(merchState)
	zkchanneldb.AddMerchState(zkMerchDB, merchStateBytes)

	stateNonceBytes, _ := json.Marshal(stateNonce)
	zkchanneldb.AddMerchField(zkMerchDB, stateNonceBytes, "stateNonceKey")

	// amountBytes, _ := json.Marshal(amount)
	// zkchanneldb.AddMerchField(zkMerchDB, amountBytes, "amountKey")

	zkMerchDB.Close()

	payTokenMaskComBytes := []byte(payTokenMaskCom)

	zkPayMaskCom := lnwire.ZkPayMaskCom{
		PayTokenMaskCom: payTokenMaskComBytes,
	}
	p.SendMessage(false, &zkPayMaskCom)

}

func (z *zkChannelManager) processZkPayMaskCom(msg *lnwire.ZkPayMaskCom, p lnpeer.Peer) {

	payTokenMaskCom := string(msg.PayTokenMaskCom)

	zkchLog.Debug("Just received ZkPayMaskCom")

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

	var custState libzkchannels.CustState
	err = json.Unmarshal(custStateBytes, &custState)

	// read channelState from ZkCustDB
	var channelStateBytes []byte
	err = zkCustDB.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(zkchanneldb.CustBucket).Cursor()
		_, v := c.Seek([]byte("channelStateKey"))
		channelStateBytes = v

		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	var channelState libzkchannels.ChannelState
	err = json.Unmarshal(channelStateBytes, &channelState)

	zkchLog.Debug("channelState MerchPayOutPk => ", *channelState.MerchPayOutPk)
	zkchLog.Debug("channelState MerchDisputePk => ", *channelState.MerchDisputePk)

	// read newstate from ZkCustDB
	var newStateBytes []byte
	err = zkCustDB.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(zkchanneldb.CustBucket).Cursor()
		_, v := c.Seek([]byte("newStateKey"))
		newStateBytes = v

		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	var newState libzkchannels.State
	err = json.Unmarshal(newStateBytes, &newState)

	// read oldstate from ZkCustDB
	var oldStateBytes []byte
	err = zkCustDB.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(zkchanneldb.CustBucket).Cursor()
		_, v := c.Seek([]byte("oldStateKey"))
		oldStateBytes = v

		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	var oldState libzkchannels.State
	err = json.Unmarshal(oldStateBytes, &oldState)

	// read channelToken from ZkCustDB
	var channelTokenBytes []byte
	err = zkCustDB.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(zkchanneldb.CustBucket).Cursor()
		_, v := c.Seek([]byte("channelTokenKey"))
		channelTokenBytes = v

		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	var channelToken libzkchannels.ChannelToken
	err = json.Unmarshal(channelTokenBytes, &channelToken)

	// read state from ZkCustDB
	var revStateBytes []byte
	err = zkCustDB.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(zkchanneldb.CustBucket).Cursor()
		_, v := c.Seek([]byte("revStateKey"))
		revStateBytes = v

		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	var revState libzkchannels.RevokedState
	err = json.Unmarshal(revStateBytes, &revState)

	// read amountBytes from ZkCustDB
	var amountBytes []byte
	err = zkCustDB.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(zkchanneldb.CustBucket).Cursor()
		_, v := c.Seek([]byte("amountKey"))
		amountBytes = v

		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	var amount int64
	err = json.Unmarshal(amountBytes, &amount)

	zkCustDB.Close()
	revLockCom := revState.RevLockCom

	zkchLog.Debug("Variables going into PayCustomer:")
	zkchLog.Debug("channelState => ", channelState)
	zkchLog.Debug("channelToken => ", channelToken)
	zkchLog.Debug("oldState => ", oldState)
	zkchLog.Debug("newState => ", newState)
	zkchLog.Debug("payTokenMaskCom => ", payTokenMaskCom)
	zkchLog.Debug("revLockCom => ", revLockCom)
	zkchLog.Debug("amount => ", amount)
	zkchLog.Debug("custState => ", custState)

	zkchLog.Debug("channelState MerchPayOutPk => ", *channelState.MerchPayOutPk)
	zkchLog.Debug("channelState MerchDisputePk => ", *channelState.MerchDisputePk)

	revLockComBytes := []byte(revLockCom)

	amountBytes = make([]byte, 8)
	binary.LittleEndian.PutUint64(amountBytes, uint64(amount))

	ZkPayMPC := lnwire.ZkPayMPC{
		Amount:          amountBytes,
		PayTokenMaskCom: msg.PayTokenMaskCom,
		RevLockCom:      revLockComBytes,
	}
	p.SendMessage(false, &ZkPayMPC)

	zkchLog.Debug("channelState channelTokenPkM => ", channelToken.PkM)

	isOk, custState, err := libzkchannels.PayCustomer(channelState, channelToken, oldState, newState, payTokenMaskCom, revLockCom, amount, custState)
	if err != nil {
		log.Fatal(err)
	}

	switch isOk {
	case true:
		zkchLog.Info("MPC pay protocol succeeded")
	case false:
		zkchLog.Info("MPC pay protocol failed")
	}

	// TODO?: SEND MESSAGE FOR IsOK. e.g.
	// ZkPayMPCResult := lnwire.ZkPayMPCResult{
	// 	isOk: isOkBytes,
	// }
	// p.SendMessage(false, &ZkPayMPCResult)

	// Add variables to zkchannelsdb
	zkCustDB, err = zkchanneldb.SetupZkCustDB()

	custStateBytes, _ = json.Marshal(custState)
	zkchanneldb.AddCustState(zkCustDB, custStateBytes)

	zkCustDB.Close()

}

func (z *zkChannelManager) processZkPayMPC(msg *lnwire.ZkPayMPC, p lnpeer.Peer) {

	amount := int64(binary.LittleEndian.Uint64(msg.Amount))
	payTokenMaskCom := string(msg.PayTokenMaskCom)
	revLockCom := string(msg.RevLockCom)

	zkchLog.Debug("Just received ZkPayMPC")

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

	// read merchState from ZkMerchDB
	var channelStateBytes []byte
	err = zkMerchDB.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(zkchanneldb.MerchBucket).Cursor()
		_, v := c.Seek([]byte("channelStateKey"))
		channelStateBytes = v
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	var channelState libzkchannels.ChannelState
	err = json.Unmarshal(channelStateBytes, &channelState)

	// read stateNonce from ZkMerchDB
	var stateNonceBytes []byte
	err = zkMerchDB.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(zkchanneldb.MerchBucket).Cursor()
		_, v := c.Seek([]byte("stateNonceKey"))
		stateNonceBytes = v
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	var stateNonce string
	err = json.Unmarshal(stateNonceBytes, &stateNonce)

	// // read amount from ZkMerchDB
	// var amountBytes []byte
	// err = zkMerchDB.View(func(tx *bolt.Tx) error {
	// 	c := tx.Bucket(zkchanneldb.MerchBucket).Cursor()
	// 	_, v := c.Seek([]byte("amountKey"))
	// 	amountBytes = v
	// 	return nil
	// })
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// var amount int64
	// err = json.Unmarshal(amountBytes, &amount)

	// read totalReceived from ZkMerchDB
	var totalReceivedBytes []byte
	err = zkMerchDB.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(zkchanneldb.MerchBucket).Cursor()
		_, v := c.Seek([]byte("totalReceivedKey"))
		totalReceivedBytes = v
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	var totalReceived int64
	err = json.Unmarshal(totalReceivedBytes, &totalReceived)

	zkMerchDB.Close()

	zkchLog.Debug("Variables going into PayMerchant:")
	zkchLog.Debug("channelState => ", channelState)
	zkchLog.Debug("stateNonce => ", stateNonce)
	zkchLog.Debug("revLock => ", revLockCom)
	zkchLog.Debug("merchState => ", merchState)
	zkchLog.Debug("payTokenMaskCom => ", payTokenMaskCom)
	zkchLog.Debug("amount => ", amount)

	zkchLog.Debug("channelState MerchPayOutPk => ", *channelState.MerchPayOutPk)
	zkchLog.Debug("channelState MerchDisputePk => ", *channelState.MerchDisputePk)

	zkchLog.Debug("channelState MerchStatePkM => ", *merchState.PkM)

	maskedTxInputs, merchState, err := libzkchannels.PayMerchant(channelState, stateNonce, payTokenMaskCom, revLockCom, amount, merchState)

	totalReceived += amount

	// Update merchState in zkMerchDB
	zkMerchDB, err = zkchanneldb.SetupZkMerchDB()

	merchStateBytes, _ = json.Marshal(merchState)
	zkchanneldb.AddMerchState(zkMerchDB, merchStateBytes)

	totalReceivedBytes, _ = json.Marshal(totalReceived)
	zkchanneldb.AddMerchField(zkMerchDB, totalReceivedBytes, "totalReceivedKey")

	zkMerchDB.Close()

	maskedTxInputsBytes, _ := json.Marshal(maskedTxInputs)

	zkPayMaskedTxInputs := lnwire.ZkPayMaskedTxInputs{
		MaskedTxInputs: maskedTxInputsBytes,
	}
	p.SendMessage(false, &zkPayMaskedTxInputs)
}

func (z *zkChannelManager) processZkPayMaskedTxInputs(msg *lnwire.ZkPayMaskedTxInputs, p lnpeer.Peer) {

	var maskedTxInputs libzkchannels.MaskedTxInputs
	err := json.Unmarshal(msg.MaskedTxInputs, &maskedTxInputs)
	if err != nil {
		log.Fatal(err)
	}

	zkchLog.Debug("Just received ZkPayMaskedTxInputs: ", maskedTxInputs)

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

	var custState libzkchannels.CustState
	err = json.Unmarshal(custStateBytes, &custState)

	// read channelState from ZkCustDB
	var channelStateBytes []byte
	err = zkCustDB.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(zkchanneldb.CustBucket).Cursor()
		_, v := c.Seek([]byte("channelStateKey"))
		channelStateBytes = v

		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	var channelState libzkchannels.ChannelState
	err = json.Unmarshal(channelStateBytes, &channelState)

	// read channelToken from ZkCustDB
	var channelTokenBytes []byte
	err = zkCustDB.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(zkchanneldb.CustBucket).Cursor()
		_, v := c.Seek([]byte("channelTokenKey"))
		channelTokenBytes = v

		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	var channelToken libzkchannels.ChannelToken
	err = json.Unmarshal(channelTokenBytes, &channelToken)

	zkCustDB.Close()

	isOk, custState, err := libzkchannels.PayUnmaskTxCustomer(channelState, channelToken, maskedTxInputs, custState)

	switch isOk {
	case true:
		zkchLog.Info("PayUnmaskTxCustomer successful")
	case false:
		zkchLog.Info("PayUnmaskTxCustomer failed")
	}

	zkchLog.Debug("After PayUnmaskTxCustomer, custState =>:", *custState.State)

	// Update custState in zkchannelsdb
	zkCustDB, err = zkchanneldb.SetupZkCustDB()

	custStateBytes, _ = json.Marshal(custState)
	zkchanneldb.AddCustState(zkCustDB, custStateBytes)

	zkCustDB.Close()

	// REVOKE OLD STATE
	// open the zkchanneldb to load custState
	zkCustDB, err = zkchanneldb.SetupZkCustDB()

	// read state from ZkCustDB
	var revStateBytes []byte
	err = zkCustDB.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(zkchanneldb.CustBucket).Cursor()
		_, v := c.Seek([]byte("revStateKey"))
		revStateBytes = v

		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	var revState libzkchannels.RevokedState
	err = json.Unmarshal(revStateBytes, &revState)

	zkCustDB.Close()

	revStateBytes, _ = json.Marshal(revState)

	zkPayRevoke := lnwire.ZkPayRevoke{
		RevState: revStateBytes,
	}
	p.SendMessage(false, &zkPayRevoke)

}

func (z *zkChannelManager) processZkPayRevoke(msg *lnwire.ZkPayRevoke, p lnpeer.Peer) {

	var revState libzkchannels.RevokedState
	err := json.Unmarshal(msg.RevState, &revState)
	if err != nil {
		log.Fatal(err)
	}

	zkchLog.Info("Just received ZkPayRevoke: ", revState)

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

	zkMerchDB.Close()

	payTokenMask, payTokenMaskR, merchState, err := libzkchannels.PayValidateRevLockMerchant(revState, merchState)
	if err != nil {
		log.Fatal(err)
	}

	// Update merchState in zkMerchDB
	zkMerchDB, err = zkchanneldb.SetupZkMerchDB()

	merchStateBytes, _ = json.Marshal(merchState)
	zkchanneldb.AddMerchState(zkMerchDB, merchStateBytes)

	zkMerchDB.Close()

	payTokenMaskBytes := []byte(payTokenMask)
	payTokenMaskRBytes := []byte(payTokenMaskR)

	zkPayTokenMask := lnwire.ZkPayTokenMask{
		PayTokenMask:  payTokenMaskBytes,
		PayTokenMaskR: payTokenMaskRBytes,
	}
	p.SendMessage(false, &zkPayTokenMask)

}

func (z *zkChannelManager) processZkPayTokenMask(msg *lnwire.ZkPayTokenMask, p lnpeer.Peer) {

	payTokenMask := string(msg.PayTokenMask)
	payTokenMaskR := string(msg.PayTokenMaskR)

	zkchLog.Info("Just received PayTokenMask and PayTokenMaskR: ", payTokenMask, payTokenMaskR)

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

	var custState libzkchannels.CustState
	err = json.Unmarshal(custStateBytes, &custState)

	zkCustDB.Close()

	isOk, custState, err := libzkchannels.PayUnmaskPayTokenCustomer(payTokenMask, payTokenMaskR, custState)
	if err != nil {
		log.Fatal(err)
	}

	switch isOk {
	case true:
		zkchLog.Info("Unmask Pay Token successful")
	case false:
		zkchLog.Info("Unmask Pay Token failed")
	}

	// Update custState in zkchannelsdb
	zkCustDB, err = zkchanneldb.SetupZkCustDB()

	custStateBytes, _ = json.Marshal(custState)
	zkchanneldb.AddCustState(zkCustDB, custStateBytes)

	zkCustDB.Close()
}

// CloseZkChannel broadcasts a close transaction
func (z *zkChannelManager) CloseZkChannel(wallet *lnwallet.LightningWallet) {

	// TODO: If --force is not set, initiate a mutual close

	// Set closeInitiated flag to prevent further zkpayments
	closeInitiated := true

	var CloseEscrowTx string

	user, err := CustOrMerch()
	if err != nil {
		log.Fatal(err)
	}

	switch user {
	case "cust":
		// Add a flag to zkchannelsdb to say that closeChannel has been initiated.
		// This is used to prevent another payment being made
		zkCustDB, err := zkchanneldb.SetupZkCustDB()
		closeInitiatedBytes, _ := json.Marshal(closeInitiated)
		zkchanneldb.AddCustField(zkCustDB, closeInitiatedBytes, "closeInitiatedKey")
		zkCustDB.Close()

		// open the zkchanneldb to load custState
		zkCustDB, err = zkchanneldb.SetupZkCustDB()

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

		zkCustDB.Close()

		CloseEscrowTx = custState.CloseEscrowTx

	case "merch":

		// Add a flag to zkchannelsdb to say that closeChannel has been initiated.
		// This is used to prevent another payment being made
		zkMerchDB, err := zkchanneldb.SetupZkMerchDB()
		closeInitiatedBytes, _ := json.Marshal(closeInitiated)
		zkchanneldb.AddMerchField(zkMerchDB, closeInitiatedBytes, "closeInitiatedKey")
		zkMerchDB.Close()

		// open the zkchanneldb to load signedMerchCloseTx
		zkMerchDB, err = zkchanneldb.SetupZkMerchDB()

		// read merchState from ZkMerchDB
		var signedMerchCloseTxBytes []byte
		err = zkMerchDB.View(func(tx *bolt.Tx) error {
			c := tx.Bucket(zkchanneldb.MerchBucket).Cursor()
			_, v := c.Seek([]byte("signedMerchCloseTxKey"))
			signedMerchCloseTxBytes = v
			return nil
		})
		if err != nil {
			log.Fatal(err)
		}

		var signedMerchCloseTx string
		err = json.Unmarshal(signedMerchCloseTxBytes, &signedMerchCloseTx)

		zkMerchDB.Close()

		CloseEscrowTx = signedMerchCloseTx
	}

	zkchLog.Debug("Loaded CloseEscrowTx =>:", CloseEscrowTx)

	// Broadcast escrow tx on chain
	serializedTx, err := hex.DecodeString(CloseEscrowTx)
	if err != nil {
		zkchLog.Error(err)
	}

	var msgTx wire.MsgTx
	err = msgTx.Deserialize(bytes.NewReader(serializedTx))
	if err != nil {
		zkchLog.Error(err)
	}

	zkchLog.Info("Broadcasting close transaction")
	wallet.PublishTransaction(&msgTx)

}

// ZkChannelBalance returns the balance on the customer's zkchannel
func ZkChannelBalance() (int64, error) {

	var zkbalance int64

	// open the zkchanneldb to load custState
	zkCustDB, err := zkchanneldb.SetupZkCustDB()

	// TODO: Add error message for the case where custState does not exist,
	// it's because the user hasn't opened a zkchannel.

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
	if err != nil {
		log.Fatal(err)
	}

	zkchLog.Info("Cust balance:", custState.CustBalance)

	zkCustDB.Close()

	zkbalance = custState.CustBalance

	return zkbalance, nil
}

// TotalReceived returns the balance on the customer's zkchannel
func TotalReceived() (int64, error) {

	zkMerchDB, err := zkchanneldb.SetupZkMerchDB()

	// read totalReceived from ZkMerchDB
	var totalReceivedBytes []byte
	err = zkMerchDB.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(zkchanneldb.MerchBucket).Cursor()
		_, v := c.Seek([]byte("totalReceivedKey"))
		totalReceivedBytes = v
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	var totalReceived int64
	err = json.Unmarshal(totalReceivedBytes, &totalReceived)

	zkMerchDB.Close()

	return totalReceived, nil
}

// ZkInfo returns info about this zklnd node
func ZkInfo() (string, error) {

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

	zkMerchDB.Close()

	return *merchState.PkM, nil
}

// DetermineIfCust is used to check the user is a customer
func DetermineIfCust() bool {
	if user, err := CustOrMerch(); user == "cust" {
		if err != nil {
			log.Fatal(err)
		}
		return true
	}
	return false
}

// DetermineIfMerch is used to check the user is a merchant
func DetermineIfMerch() bool {
	if user, err := CustOrMerch(); user == "merch" {
		if err != nil {
			log.Fatal(err)
		}
		return true
	}
	return false
}

// CustOrMerch determines if the user is a customer or merchant,
// based on whether they have zkcust.db or zkmerch.db set up
func CustOrMerch() (string, error) {

	var custdbExists, merchdbExists bool
	if _, err := os.Stat("zkcust.db"); err == nil {
		custdbExists = true
	}
	if _, err := os.Stat("zkmerch.db"); err == nil {
		merchdbExists = true
	}

	if custdbExists && merchdbExists {
		return "both", fmt.Errorf("Cannot run both a Customer and Merchant node. " +
			"Both zkcust.cb and zkmerch.db exist")
	} else if custdbExists {
		return "cust", nil
	} else if merchdbExists {
		return "merch", nil
	}
	return "neither", fmt.Errorf("neither zkcust.db or zkmerch.db found")
}
