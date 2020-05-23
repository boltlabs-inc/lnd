package lnd

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/jinzhu/copier"
	"github.com/lightningnetwork/lnd/chainntnfs"
	"github.com/lightningnetwork/lnd/libzkchannels"
	"github.com/lightningnetwork/lnd/lnpeer"
	"github.com/lightningnetwork/lnd/lnwallet"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/zkchanneldb"
)

type zkChannelManager struct {
	zkChannelName string
	Notifier      chainntnfs.ChainNotifier
	wg            sync.WaitGroup
}

func (z *zkChannelManager) initZkEstablish(inputSats int64, custUtxoTxid_LE string, index uint32, custInputSk string, custStateSk string, custPayoutSk string, changeScriptPK string, merchPubKey string, zkChannelName string, custBal int64, merchBal int64, p lnpeer.Peer) {

	channelToken, custState, err := libzkchannels.InitCustomer(fmt.Sprintf("\"%v\"", merchPubKey), custBal, merchBal, "cust", custStateSk, custPayoutSk)
	if err != nil {
		log.Fatal(err)
	}

	zkchLog.Debug("Generated channelToken and custState")
	zkchLog.Debugf("%#v", channelToken)

	custPk := fmt.Sprintf("%v", custState.PkC)
	revLock := fmt.Sprintf("%v", custState.RevLock)

	merchPk := fmt.Sprintf("%v", merchPubKey)

	changePkIsHash := true

	zkchLog.Debug("Variables going into FormEscrowTx :=> ", custUtxoTxid_LE, index, inputSats, custBal, custInputSk, custPk, merchPk, changeScriptPK, changePkIsHash)

	signedEscrowTx, escrowTxid, escrowPrevout, err := libzkchannels.FormEscrowTx(custUtxoTxid_LE, index, inputSats, custBal, custInputSk, custPk, merchPk, changeScriptPK, changePkIsHash)
	if err != nil {
		log.Fatal(err)
	}

	zkchLog.Info("escrow txid => ", escrowTxid)
	zkchLog.Info("signedEscrowTx => ", signedEscrowTx)

	zkchLog.Info("storing new zkchannel variables for:", zkChannelName)
	// TODO: Write a function to handle the storing of variables in zkchanneldb
	// Add variables to zkchannelsdb
	zkCustDB, err := zkchanneldb.OpenZkChannelBucket(zkChannelName)

	custStateBytes, _ := json.Marshal(custState)
	zkchanneldb.AddCustState(zkCustDB, zkChannelName, custStateBytes)

	channelTokenBytes, _ := json.Marshal(channelToken)
	zkchanneldb.AddCustField(zkCustDB, zkChannelName, channelTokenBytes, "channelTokenKey")

	merchPkBytes, _ := json.Marshal(merchPk)
	zkchanneldb.AddCustField(zkCustDB, zkChannelName, merchPkBytes, "merchPkKey")

	custBalBytes, _ := json.Marshal(custBal)
	zkchanneldb.AddCustField(zkCustDB, zkChannelName, custBalBytes, "custBalKey")

	merchBalBytes, _ := json.Marshal(merchBal)
	zkchanneldb.AddCustField(zkCustDB, zkChannelName, merchBalBytes, "merchBalKey")

	escrowTxidBytes, _ := json.Marshal(escrowTxid)
	zkchanneldb.AddCustField(zkCustDB, zkChannelName, escrowTxidBytes, "escrowTxidKey")

	escrowPrevoutBytes, _ := json.Marshal(escrowPrevout)
	zkchanneldb.AddCustField(zkCustDB, zkChannelName, escrowPrevoutBytes, "escrowPrevoutKey")

	signedEscrowTxBytes, _ := json.Marshal(signedEscrowTx)
	zkchanneldb.AddCustField(zkCustDB, zkChannelName, signedEscrowTxBytes, "signedEscrowTxKey")

	zkCustDB.Close()

	zkchLog.Debug("Saved custState and channelToken")

	// TODO: see if it's necessary to be sending these
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
	toSelfDelay := "05cf"

	zkchLog.Debug("Just received ZkEstablishOpen")

	// // Convert variables received
	// escrowTxid := string(msg.EscrowTxid)
	// custPk := string(msg.CustPk)
	// escrowPrevout := string(msg.EscrowPrevout)
	// revLock := string(msg.RevLock)

	// custBal := int64(binary.LittleEndian.Uint64(msg.CustBal))
	// merchBal := int64(binary.LittleEndian.Uint64(msg.MerchBal))

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

	var merchState libzkchannels.MerchState
	merchStateBytes, err := zkchanneldb.GetMerchState(zkMerchDB)
	err = json.Unmarshal(merchStateBytes, &merchState)

	var channelState libzkchannels.ChannelState
	channelStateBytes, err := zkchanneldb.GetMerchField(zkMerchDB, "channelStateKey")
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

func (z *zkChannelManager) processZkEstablishAccept(msg *lnwire.ZkEstablishAccept, p lnpeer.Peer, zkChannelName string) {

	zkchLog.Debugf("Just received ZkEstablishAccept for %v", zkChannelName)

	toSelfDelay := string(msg.ToSelfDelay)
	merchClosePk := string(msg.MerchPayoutPk)

	var channelState libzkchannels.ChannelState
	err := json.Unmarshal(msg.ChannelState, &channelState)

	// TODO: Might not have to convert back and forth between bytes here
	// Add variables to zkchannelsdb
	zkCustDB, err := zkchanneldb.OpenZkChannelBucket(zkChannelName)

	channelStateBytes, _ := json.Marshal(channelState)
	zkchanneldb.AddCustField(zkCustDB, zkChannelName, channelStateBytes, "channelStateKey")

	zkCustDB.Close()

	// open the zkchanneldb to load custState
	zkCustDB, err = zkchanneldb.OpenZkChannelBucket(zkChannelName)

	var custState libzkchannels.CustState
	custStateBytes, err := zkchanneldb.GetCustState(zkCustDB, zkChannelName)
	err = json.Unmarshal(custStateBytes, &custState)

	var merchPk string
	merchPkBytes, err := zkchanneldb.GetCustField(zkCustDB, zkChannelName, "merchPkKey")
	err = json.Unmarshal(merchPkBytes, &merchPk)

	var escrowTxid string
	escrowTxidBytes, err := zkchanneldb.GetCustField(zkCustDB, zkChannelName, "escrowTxidKey")
	err = json.Unmarshal(escrowTxidBytes, &escrowTxid)

	var custBal int64
	custBalBytes, err := zkchanneldb.GetCustField(zkCustDB, zkChannelName, "custBalKey")
	err = json.Unmarshal(custBalBytes, &custBal)

	var merchBal int64
	merchBalBytes, err := zkchanneldb.GetCustField(zkCustDB, zkChannelName, "merchBalKey")
	err = json.Unmarshal(merchBalBytes, &merchBal)

	var escrowPrevout string
	escrowPrevoutBytes, err := zkchanneldb.GetCustField(zkCustDB, zkChannelName, "escrowPrevoutKey")
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
	if err != nil {
		log.Fatal(err)
	}

	var merchState libzkchannels.MerchState
	merchStateBytes, err := zkchanneldb.GetMerchState(zkMerchDB)
	err = json.Unmarshal(merchStateBytes, &merchState)

	var toSelfDelay string
	toSelfDelayBytes, err := zkchanneldb.GetMerchField(zkMerchDB, "toSelfDelayKey")
	err = json.Unmarshal(toSelfDelayBytes, &toSelfDelay)

	zkMerchDB.Close()

	isOk, merchTxid, merchPrevout, merchState, err := libzkchannels.MerchantVerifyMerchCloseTx(escrowTxid, custPk, custBal, merchBal, toSelfDelay, custSig, merchState)
	if err != nil {
		log.Fatal(err)
	}

	// Add update merchState in db
	zkMerchDB, err = zkchanneldb.SetupZkMerchDB()

	merchStateBytes, _ = json.Marshal(merchState)
	zkchanneldb.AddMerchState(zkMerchDB, merchStateBytes)

	zkMerchDB.Close()

	switch isOk {
	case true:
		zkchLog.Info("MerchantVerifyMerchCloseTx succeeded")
	case false:
		zkchLog.Info("MerchantVerifyMerchCloseTx failed")
	}

	zkchLog.Info("Merch close txid = ", merchTxid)
	zkchLog.Debug("merch prevout = ", merchPrevout)

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

	// Create and save pkScript and escrowTxid
	// Note that we are reversing the bytes to correct escrowTxid into little endian

	merchPk := fmt.Sprintf("%v", *merchState.PkM)
	multisigScriptHex := []byte("5221" + merchPk + "21" + custPk + "52ae")
	multisigScript := make([]byte, hex.DecodedLen(len(multisigScriptHex)))
	_, err = hex.Decode(multisigScript, multisigScriptHex)

	h := sha256.New()
	h.Write(multisigScript)

	scriptSha := fmt.Sprintf("%x", h.Sum(nil))

	pkScriptHex := []byte("0020" + scriptSha)

	pkScript := make([]byte, hex.DecodedLen(len(pkScriptHex)))
	_, err = hex.Decode(pkScript, pkScriptHex)

	zkchLog.Debugf("multisigScript: %#x\n", multisigScript)
	zkchLog.Debugf("pkScript: %#x\n", pkScript)

	// Update merchState in zkMerchDB
	zkMerchDB, err = zkchanneldb.SetupZkMerchDB()

	escrowTxidBytes, _ := json.Marshal(escrowTxid)
	zkchanneldb.AddMerchField(zkMerchDB, escrowTxidBytes, "escrowTxidKey")

	pkScriptBytes, _ := json.Marshal(pkScript)
	zkchanneldb.AddMerchField(zkMerchDB, pkScriptBytes, "pkScriptKey")

	zkMerchDB.Close()

}

func (z *zkChannelManager) processZkEstablishCCloseSigned(msg *lnwire.ZkEstablishCCloseSigned, p lnpeer.Peer, zkChannelName string) {

	zkchLog.Debugf("Just received CCloseSigned for %v", zkChannelName)

	// Convert variables received
	escrowSig := string(msg.EscrowSig)
	merchSig := string(msg.MerchSig)
	merchTxid := string(msg.MerchTxid)
	merchPrevout := string(msg.MerchPrevout)

	zkchLog.Debug("escrow sig: ", escrowSig)
	zkchLog.Debug("merch sig: ", merchSig)

	// open the zkchanneldb to load custState
	zkCustDB, err := zkchanneldb.OpenZkChannelBucket(zkChannelName)

	var custState libzkchannels.CustState
	custStateBytes, err := zkchanneldb.GetCustState(zkCustDB, zkChannelName)
	err = json.Unmarshal(custStateBytes, &custState)

	var merchPk string
	merchPkBytes, err := zkchanneldb.GetCustField(zkCustDB, zkChannelName, "merchPkKey")
	err = json.Unmarshal(merchPkBytes, &merchPk)

	var escrowTxid string
	escrowTxidBytes, err := zkchanneldb.GetCustField(zkCustDB, zkChannelName, "escrowTxidKey")
	err = json.Unmarshal(escrowTxidBytes, &escrowTxid)

	var escrowPrevout string
	escrowPrevoutBytes, err := zkchanneldb.GetCustField(zkCustDB, zkChannelName, "escrowPrevoutKey")
	err = json.Unmarshal(escrowPrevoutBytes, &escrowPrevout)

	var channelState libzkchannels.ChannelState
	channelStateBytes, err := zkchanneldb.GetCustField(zkCustDB, zkChannelName, "channelStateKey")
	err = json.Unmarshal(channelStateBytes, &channelState)

	var channelToken libzkchannels.ChannelToken
	channelTokenBytes, err := zkchanneldb.GetCustField(zkCustDB, zkChannelName, "channelTokenKey")
	err = json.Unmarshal(channelTokenBytes, &channelToken)

	var custBal int64
	custBalBytes, err := zkchanneldb.GetCustField(zkCustDB, zkChannelName, "custBalKey")
	err = json.Unmarshal(custBalBytes, &custBal)

	var merchBal int64
	merchBalBytes, err := zkchanneldb.GetCustField(zkCustDB, zkChannelName, "merchBalKey")
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

	isOk, channelToken, custState, err := libzkchannels.CustomerVerifyInitCustCloseTx(txInfo, channelState, channelToken, escrowSig, merchSig, custState)
	if err != nil {
		log.Fatal(err)
	}

	switch isOk {
	case true:
		zkchLog.Info("Merch signature on Cust Close is valid")
	case false:
		zkchLog.Info("Merch signature on Cust Close is invalid")
	}

	// Add variables to zkchannelsdb
	zkCustDB, err = zkchanneldb.OpenZkChannelBucket(zkChannelName)

	custStateBytes, _ = json.Marshal(custState)
	zkchanneldb.AddCustState(zkCustDB, zkChannelName, custStateBytes)

	channelTokenBytes, _ = json.Marshal(channelToken)
	zkchanneldb.AddCustField(zkCustDB, zkChannelName, channelTokenBytes, "channelTokenKey")

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

func (z *zkChannelManager) processZkEstablishInitialState(msg *lnwire.ZkEstablishInitialState, p lnpeer.Peer, notifier chainntnfs.ChainNotifier) {

	zkchLog.Info("Just received InitialState")

	var channelToken libzkchannels.ChannelToken
	err := json.Unmarshal(msg.ChannelToken, &channelToken)
	_ = err

	var initCustState libzkchannels.InitCustState
	err = json.Unmarshal(msg.InitCustState, &initCustState)

	initHash := string(msg.InitHash)

	// open the zkchanneldb to load merchState
	zkMerchDB, err := zkchanneldb.SetupZkMerchDB()

	var merchState libzkchannels.MerchState
	merchStateBytes, err := zkchanneldb.GetMerchState(zkMerchDB)
	err = json.Unmarshal(merchStateBytes, &merchState)

	var escrowTxid string
	escrowTxidBytes, err := zkchanneldb.GetMerchField(zkMerchDB, "escrowTxidKey")
	err = json.Unmarshal(escrowTxidBytes, &escrowTxid)

	var pkScript []byte
	pkScriptBytes, err := zkchanneldb.GetMerchField(zkMerchDB, "pkScriptKey")
	err = json.Unmarshal(pkScriptBytes, &pkScript)

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

	// Wait for on chain confirmations of escrow transaction

	zkchLog.Debugf("waitForFundingWithTimeout\npkScript: %#x\n", pkScript)

	confChannel, err := z.waitForFundingWithTimeout(notifier, escrowTxid, pkScript)
	if err != nil {
		zkchLog.Infof("error waiting for funding "+
			"confirmation: %v", err)
	}

	zkchLog.Debugf("%#v\n", confChannel)

}

func (z *zkChannelManager) processZkEstablishStateValidated(msg *lnwire.ZkEstablishStateValidated, p lnpeer.Peer, zkChannelName string, wallet *lnwallet.LightningWallet, notifier chainntnfs.ChainNotifier) {

	zkchLog.Debugf("Just received ZkEstablishStateValidated for %v", zkChannelName)

	// TODO: For now, we assume isOk is true
	// Add alternative path for when isOk is false

	// open the zkchanneldb to load custState
	zkCustDB, err := zkchanneldb.OpenZkChannelBucket(zkChannelName)

	var signedEscrowTx string
	signedEscrowTxBytes, err := zkchanneldb.GetCustField(zkCustDB, zkChannelName, "signedEscrowTxKey")
	err = json.Unmarshal(signedEscrowTxBytes, &signedEscrowTx)

	var escrowTxid string
	escrowTxidBytes, err := zkchanneldb.GetCustField(zkCustDB, zkChannelName, "escrowTxidKey")
	err = json.Unmarshal(escrowTxidBytes, &escrowTxid)

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

	zkchLog.Debugf("Broadcasting signedEscrowTx: %#v\n", signedEscrowTx)

	err = wallet.PublishTransaction(&msgTx)
	if err != nil {
		zkchLog.Error(err)
	}

	pkScript := msgTx.TxOut[0].PkScript

	// Wait for confirmations
	confChannel, err := z.waitForFundingWithTimeout(notifier, escrowTxid, pkScript)
	if err != nil {
		zkchLog.Infof("error waiting for funding "+
			"confirmation: %v", err)
	}

	zkchLog.Debugf("%#v\n", confChannel)

	// TEMPORARY DUMMY MESSAGE
	fundingLockedBytes := []byte("Funding Locked")
	zkEstablishFundingLocked := lnwire.ZkEstablishFundingLocked{
		FundingLocked: fundingLockedBytes,
	}

	closeInitiated := false

	// Add a flag to zkchannelsdb to say that closeChannel has not been initiated.
	// This is used to prevent another payment being made
	zkCustDB, err = zkchanneldb.OpenZkChannelBucket(zkChannelName)
	closeInitiatedBytes, _ := json.Marshal(closeInitiated)
	zkchanneldb.AddCustField(zkCustDB, zkChannelName, closeInitiatedBytes, "closeInitiatedKey")
	zkCustDB.Close()

	p.SendMessage(false, &zkEstablishFundingLocked)
}

// waitForFundingWithTimeout is a wrapper around waitForFundingConfirmation and
// waitForTimeout that will return ErrConfirmationTimeout if we are not the
// channel initiator and the maxWaitNumBlocksFundingConf has passed from the
// funding broadcast height. In case of confirmation, the short channel ID of
// // the channel and the funding transaction will be returned.
// func (z *zkChannelManager) waitForFundingWithTimeout(
// 	ch *channeldb.OpenChannel) (*confirmedChannel, error) {

func (z *zkChannelManager) waitForFundingWithTimeout(notifier chainntnfs.ChainNotifier, escrowTxid string, pkScript []byte) (*confirmedChannel, error) {

	confChan := make(chan *confirmedChannel)
	timeoutChan := make(chan error, 1)
	cancelChan := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(1)

	// TEMPORARY CODE TO FLIP BYTES
	s := ""
	for i := 0; i < len(escrowTxid)/2; i++ {
		s = escrowTxid[i*2:i*2+2] + s
	}
	escrowTxid = s

	go z.waitForFundingConfirmation(notifier, cancelChan, confChan, wg, escrowTxid, pkScript)

	// If we are not the initiator, we have no money at stake and will
	// timeout waiting for the funding transaction to confirm after a
	// while.
	IsInitiator := true
	if !IsInitiator {
		wg.Add(1)
		go z.waitForTimeout(notifier, cancelChan, timeoutChan, wg)
	}

	defer close(cancelChan)

	select {
	case err := <-timeoutChan:

		if err != nil {
			return nil, err
		}
		return nil, ErrConfirmationTimeout

	// case <-z.quit:
	// 	// The fundingManager is shutting down, and will resume wait on
	// 	// startup.
	// 	return nil, ErrFundingManagerShuttingDown

	case confirmedChannel, ok := <-confChan:
		zkchLog.Debug("waitForFundingConfirmation: confirmedChannel")

		if !ok {
			return nil, fmt.Errorf("waiting for funding" +
				"confirmation failed")
		}
		return confirmedChannel, nil
	}
}

// waitForFundingConfirmation handles the final stages of the channel funding
// process once the funding transaction has been broadcast. The primary
// function of waitForFundingConfirmation is to wait for blockchain
// confirmation, and then to notify the other systems that must be notified
// when a channel has become active for lightning transactions.
// The wait can be canceled by closing the cancelChan. In case of success,
// a *lnwire.ShortChannelID will be passed to confChan.
//
// NOTE: This MUST be run as a goroutine.
func (z *zkChannelManager) waitForFundingConfirmation(notifier chainntnfs.ChainNotifier,
	cancelChan <-chan struct{},
	confChan chan<- *confirmedChannel, wg sync.WaitGroup, escrowTxid string, pkScript []byte) {

	defer wg.Done()
	defer close(confChan)

	// // Register with the ChainNotifier for a notification once the funding
	// // transaction reaches `numConfs` confirmations.
	// fundingScript, err := makeFundingScript(completeChan)
	// if err != nil {
	// 	fndgLog.Errorf("unable to create funding script for "+
	// 		"ChannelPoint(%v): %v", completeChan.FundingOutpoint,
	// 		err)
	// 	return
	// }

	// Print escrowTxid and pkScript
	zkchLog.Debugf("Waiting for confirmations for escrow txid: %v with pkScript %x", escrowTxid, pkScript)

	var txid chainhash.Hash
	chainhash.Decode(&txid, escrowTxid)

	NumConfsRequired := 3
	numConfs := uint32(NumConfsRequired)
	FundingBroadcastHeight := uint32(420)

	confNtfn, err := notifier.RegisterConfirmationsNtfn(
		&txid, pkScript, numConfs,
		FundingBroadcastHeight,
	)

	if err != nil {
		zkchLog.Errorf("Unable to register for confirmation of "+
			"ChannelPoint", err)
		return
	}

	zkchLog.Infof("Waiting for funding tx (%v) to reach %v confirmations",
		txid, numConfs)

	var confDetails *chainntnfs.TxConfirmation
	var ok bool

	// Wait until the specified number of confirmations has been reached,
	// we get a cancel signal, or the wallet signals a shutdown.
	select {
	case confDetails, ok = <-confNtfn.Confirmed:
		// fallthrough

	case <-cancelChan:
		zkchLog.Warnf("canceled waiting for funding confirmation, " +
			"stopping funding flow for ChannelPoint")
		return

		// case <-z.quit:
		// 	zkchLog.Warnf("fundingManager shutting down, stopping funding "+
		// 		"flow for ChannelPoint(%v)")
		// return
	}

	if !ok {
		zkchLog.Warnf("ChainNotifier shutting down, cannot complete " +
			"funding flow for ChannelPoint")
		return
	}

	// fundingPoint := completeChan.FundingOutpoint
	// fndgLog.Infof("ChannelPoint(%v) is now active: ChannelID(%v)",
	// 	fundingPoint, lnwire.NewChanIDFromOutPoint(&fundingPoint))

	// // With the block height and the transaction index known, we can
	// // construct the compact chanID which is used on the network to unique
	// // identify channels.
	Index := 0
	shortChanID := lnwire.ShortChannelID{
		BlockHeight: confDetails.BlockHeight,
		TxIndex:     confDetails.TxIndex,
		TxPosition:  uint16(Index),
	}

	select {
	case confChan <- &confirmedChannel{
		shortChanID: shortChanID,
		fundingTx:   confDetails.Tx,
	}:
		// case <-z.quit:
		// return
	}
}

// waitForTimeout will close the timeout channel if maxWaitNumBlocksFundingConf
// has passed from the broadcast height of the given channel. In case of error,
// the error is sent on timeoutChan. The wait can be canceled by closing the
// cancelChan.
//
// NOTE: timeoutChan MUST be buffered.
// NOTE: This MUST be run as a goroutine.
func (z *zkChannelManager) waitForTimeout(notifier chainntnfs.ChainNotifier,
	cancelChan <-chan struct{}, timeoutChan chan<- error, wg sync.WaitGroup) {
	defer wg.Done()

	epochClient, err := notifier.RegisterBlockEpochNtfn(nil)
	if err != nil {
		timeoutChan <- fmt.Errorf("unable to register for epoch "+
			"notification: %v", err)
		return
	}

	defer epochClient.Cancel()

	// // On block maxHeight we will cancel the funding confirmation wait.
	// maxHeight := completeChan.FundingBroadcastHeight + maxWaitNumBlocksFundingConf
	maxHeight := uint32(10)
	for {
		select {
		case epoch, ok := <-epochClient.Epochs:
			if !ok {
				timeoutChan <- fmt.Errorf("epoch client " +
					"shutting down")
				return
			}

			// Close the timeout channel and exit if the block is
			// aboce the max height.
			if uint32(epoch.Height) >= maxHeight {
				zkchLog.Warnf("Waited for %v blocks without "+
					"seeing funding transaction confirmed,"+
					" cancelling.",
					maxWaitNumBlocksFundingConf)

				// Notify the caller of the timeout.
				close(timeoutChan)
				return
			}

			// TODO: If we are the channel initiator implement
			// a method for recovering the funds from the funding
			// transaction

		case <-cancelChan:
			return

			// case <-z.quit:
			// 	// The fundingManager is shutting down, will resume
			// 	// waiting for the funding transaction on startup.
			// 	return
		}
	}
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

func (z *zkChannelManager) processZkEstablishFundingConfirmed(msg *lnwire.ZkEstablishFundingConfirmed, p lnpeer.Peer, zkChannelName string) {

	zkchLog.Debugf("Just received FundingConfirmed for %v", zkChannelName)

	// open the zkchanneldb to load custState
	zkCustDB, err := zkchanneldb.OpenZkChannelBucket(zkChannelName)
	if err != nil {
		zkchLog.Error(err)
	}

	var custState libzkchannels.CustState
	custStateBytes, err := zkchanneldb.GetCustState(zkCustDB, zkChannelName)
	err = json.Unmarshal(custStateBytes, &custState)

	var channelToken libzkchannels.ChannelToken
	channelTokenBytes, err := zkchanneldb.GetCustField(zkCustDB, zkChannelName, "channelTokenKey")
	err = json.Unmarshal(channelTokenBytes, &channelToken)

	zkchLog.Debug("ActivateCustomer, channelToken =>:", channelToken)

	zkCustDB.Close()

	state, custState, err := libzkchannels.ActivateCustomer(custState)
	zkchLog.Debug("ActivateCustomer, state =>:", state)

	// Add variables to zkchannelsdb
	zkCustDB, err = zkchanneldb.OpenZkChannelBucket(zkChannelName)

	custStateBytes, _ = json.Marshal(custState)
	zkchanneldb.AddCustState(zkCustDB, zkChannelName, custStateBytes)

	stateBytes, _ := json.Marshal(state)
	zkchanneldb.AddCustField(zkCustDB, zkChannelName, stateBytes, "stateKey")

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

	var merchState libzkchannels.MerchState
	merchStateBytes, err := zkchanneldb.GetMerchState(zkMerchDB)
	err = json.Unmarshal(merchStateBytes, &merchState)

	zkMerchDB.Close()

	payToken0, merchState, err := libzkchannels.ActivateMerchant(channelToken, state, merchState)

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

func (z *zkChannelManager) processZkEstablishPayToken(msg *lnwire.ZkEstablishPayToken, p lnpeer.Peer, zkChannelName string) {

	payToken0 := string(msg.PayToken0)
	zkchLog.Debugf("Just received PayToken0 for %v: ", zkChannelName, payToken0)

	// open the zkchanneldb to load custState
	zkCustDB, err := zkchanneldb.OpenZkChannelBucket(zkChannelName)

	var custState libzkchannels.CustState
	custStateBytes, err := zkchanneldb.GetCustState(zkCustDB, zkChannelName)
	err = json.Unmarshal(custStateBytes, &custState)

	zkCustDB.Close()

	custState, err = libzkchannels.ActivateCustomerFinalize(payToken0, custState)
	if err != nil {
		log.Fatal(err)
	}

	// Add variables to zkchannelsdb
	zkCustDB, err = zkchanneldb.OpenZkChannelBucket(zkChannelName)

	custStateBytes, _ = json.Marshal(custState)
	zkchanneldb.AddCustState(zkCustDB, zkChannelName, custStateBytes)

	zkCustDB.Close()
}

func (z *zkChannelManager) InitZkPay(p lnpeer.Peer, zkChannelName string, Amount int64) {

	zkCustDB, err := zkchanneldb.OpenZkChannelBucket(zkChannelName)

	var custState libzkchannels.CustState
	custStateBytes, err := zkchanneldb.GetCustState(zkCustDB, zkChannelName)
	err = json.Unmarshal(custStateBytes, &custState)

	var channelState libzkchannels.ChannelState
	channelStateBytes, err := zkchanneldb.GetCustField(zkCustDB, zkChannelName, "channelStateKey")
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
	zkCustDB, err = zkchanneldb.OpenZkChannelBucket(zkChannelName)

	custStateBytes, _ = json.Marshal(custState)
	zkchanneldb.AddCustState(zkCustDB, zkChannelName, custStateBytes)

	revStateBytes, _ := json.Marshal(revState)
	zkchanneldb.AddCustField(zkCustDB, zkChannelName, revStateBytes, "revStateKey")

	newStateBytes, _ := json.Marshal(newState)
	zkchanneldb.AddCustField(zkCustDB, zkChannelName, newStateBytes, "newStateKey")

	oldStateBytes, _ := json.Marshal(oldState)
	zkchanneldb.AddCustField(zkCustDB, zkChannelName, oldStateBytes, "oldStateKey")

	amountBytes, _ := json.Marshal(Amount)
	zkchanneldb.AddCustField(zkCustDB, zkChannelName, amountBytes, "amountKey")

	zkCustDB.Close()

	oldStateNonce := oldState.Nonce
	oldStateNonceBytes := []byte(oldStateNonce)

	amountBytes = make([]byte, 8)
	binary.LittleEndian.PutUint64(amountBytes, uint64(Amount))

	revLockCom := revState.RevLockCom
	revLockComBytes := []byte(revLockCom)

	// TODO: Add amount
	zkpaynonce := lnwire.ZkPayNonce{
		StateNonce: oldStateNonceBytes,
		Amount:     amountBytes,
		RevLockCom: revLockComBytes,
	}

	p.SendMessage(false, &zkpaynonce)
}

func (z *zkChannelManager) processZkPayNonce(msg *lnwire.ZkPayNonce, p lnpeer.Peer) {

	stateNonce := string(msg.StateNonce)
	amount := int64(binary.LittleEndian.Uint64(msg.Amount))
	revLockCom := string(msg.RevLockCom)

	zkchLog.Debug("Just received ZkPayNonce")

	// open the zkchanneldb to load merchState
	zkMerchDB, err := zkchanneldb.SetupZkMerchDB()

	var merchState libzkchannels.MerchState
	merchStateBytes, err := zkchanneldb.GetMerchState(zkMerchDB)
	err = json.Unmarshal(merchStateBytes, &merchState)

	var channelState libzkchannels.ChannelState
	channelStateBytes, err := zkchanneldb.GetMerchField(zkMerchDB, "channelStateKey")
	err = json.Unmarshal(channelStateBytes, &channelState)

	zkMerchDB.Close()

	payTokenMaskCom, merchState, err := libzkchannels.PreparePaymentMerchant(channelState, stateNonce, revLockCom, amount, merchState)
	if err != nil {
		log.Fatal(err)
	}

	// Add variables to zkchannelsdb
	zkMerchDB, err = zkchanneldb.SetupZkMerchDB()

	merchStateBytes, _ = json.Marshal(merchState)
	zkchanneldb.AddMerchState(zkMerchDB, merchStateBytes)

	zkMerchDB.Close()

	payTokenMaskComBytes := []byte(payTokenMaskCom)

	zkPayMaskCom := lnwire.ZkPayMaskCom{
		PayTokenMaskCom: payTokenMaskComBytes,
	}
	p.SendMessage(false, &zkPayMaskCom)
}

func (z *zkChannelManager) processZkPayMaskCom(msg *lnwire.ZkPayMaskCom, p lnpeer.Peer, zkChannelName string) {

	payTokenMaskCom := string(msg.PayTokenMaskCom)

	zkchLog.Debug("Just received ZkPayMaskCom")

	// open the zkchanneldb to load custState
	zkCustDB, err := zkchanneldb.OpenZkChannelBucket(zkChannelName)

	var custState libzkchannels.CustState
	custStateBytes, err := zkchanneldb.GetCustState(zkCustDB, zkChannelName)
	err = json.Unmarshal(custStateBytes, &custState)

	var channelState libzkchannels.ChannelState
	channelStateBytes, err := zkchanneldb.GetCustField(zkCustDB, zkChannelName, "channelStateKey")
	err = json.Unmarshal(channelStateBytes, &channelState)

	zkchLog.Debug("channelState MerchPayOutPk => ", *channelState.MerchPayOutPk)
	zkchLog.Debug("channelState MerchDisputePk => ", *channelState.MerchDisputePk)

	var newState libzkchannels.State
	newStateBytes, err := zkchanneldb.GetCustField(zkCustDB, zkChannelName, "newStateKey")
	err = json.Unmarshal(newStateBytes, &newState)

	var oldState libzkchannels.State
	oldStateBytes, err := zkchanneldb.GetCustField(zkCustDB, zkChannelName, "oldStateKey")
	err = json.Unmarshal(oldStateBytes, &oldState)

	var channelToken libzkchannels.ChannelToken
	channelTokenBytes, err := zkchanneldb.GetCustField(zkCustDB, zkChannelName, "channelTokenKey")
	err = json.Unmarshal(channelTokenBytes, &channelToken)

	var revState libzkchannels.RevokedState
	revStateBytes, err := zkchanneldb.GetCustField(zkCustDB, zkChannelName, "revStateKey")
	err = json.Unmarshal(revStateBytes, &revState)

	var amount int64
	amountBytes, err := zkchanneldb.GetCustField(zkCustDB, zkChannelName, "amountKey")
	err = json.Unmarshal(amountBytes, &amount)

	zkCustDB.Close()
	revLockCom := revState.RevLockCom

	zkchLog.Debug("channelState MerchPayOutPk => ", *channelState.MerchPayOutPk)
	zkchLog.Debug("channelState MerchDisputePk => ", *channelState.MerchDisputePk)

	revLockComBytes := []byte(revLockCom)

	amountBytes = make([]byte, 8)
	binary.LittleEndian.PutUint64(amountBytes, uint64(amount))

	oldStateNonce := oldState.Nonce
	oldStateNonceBytes := []byte(oldStateNonce)

	ZkPayMPC := lnwire.ZkPayMPC{
		StateNonce:      oldStateNonceBytes,
		Amount:          amountBytes,
		PayTokenMaskCom: msg.PayTokenMaskCom,
		RevLockCom:      revLockComBytes,
	}
	p.SendMessage(false, &ZkPayMPC)

	zkchLog.Debug("channelState channelTokenPkM => ", channelToken.PkM)

	isOk, custState, err := libzkchannels.PayUpdateCustomer(channelState, channelToken, oldState, newState, payTokenMaskCom, revLockCom, amount, custState)
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
	zkCustDB, err = zkchanneldb.OpenZkChannelBucket(zkChannelName)

	custStateBytes, _ = json.Marshal(custState)
	zkchanneldb.AddCustState(zkCustDB, zkChannelName, custStateBytes)

	zkCustDB.Close()
}

func (z *zkChannelManager) processZkPayMPC(msg *lnwire.ZkPayMPC, p lnpeer.Peer) {

	stateNonce := string(msg.StateNonce)
	amount := int64(binary.LittleEndian.Uint64(msg.Amount))
	payTokenMaskCom := string(msg.PayTokenMaskCom)
	revLockCom := string(msg.RevLockCom)

	zkchLog.Debug("Just received ZkPayMPC")

	// open the zkchanneldb to load merchState
	zkMerchDB, err := zkchanneldb.SetupZkMerchDB()
	if err != nil {
		log.Fatal(err)
	}

	var merchState libzkchannels.MerchState
	merchStateBytes, err := zkchanneldb.GetMerchState(zkMerchDB)
	err = json.Unmarshal(merchStateBytes, &merchState)

	var channelState libzkchannels.ChannelState
	channelStateBytes, err := zkchanneldb.GetMerchField(zkMerchDB, "channelStateKey")
	err = json.Unmarshal(channelStateBytes, &channelState)

	var totalReceived int64
	totalReceivedBytes, err := zkchanneldb.GetMerchField(zkMerchDB, "totalReceivedKey")
	err = json.Unmarshal(totalReceivedBytes, &totalReceived)

	zkMerchDB.Close()

	zkchLog.Debug("channelState MerchPayOutPk => ", *channelState.MerchPayOutPk)
	zkchLog.Debug("channelState MerchDisputePk => ", *channelState.MerchDisputePk)
	zkchLog.Debug("channelState MerchStatePkM => ", *merchState.PkM)

	maskedTxInputs, merchState, err := libzkchannels.PayUpdateMerchant(channelState, stateNonce, payTokenMaskCom, revLockCom, amount, merchState)

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

func (z *zkChannelManager) processZkPayMaskedTxInputs(msg *lnwire.ZkPayMaskedTxInputs, p lnpeer.Peer, zkChannelName string) {

	var maskedTxInputs libzkchannels.MaskedTxInputs
	err := json.Unmarshal(msg.MaskedTxInputs, &maskedTxInputs)
	if err != nil {
		log.Fatal(err)
	}

	zkchLog.Debug("Just received ZkPayMaskedTxInputs: ", maskedTxInputs)

	// open the zkchanneldb to load custState
	zkCustDB, err := zkchanneldb.OpenZkChannelBucket(zkChannelName)

	var custState libzkchannels.CustState
	custStateBytes, err := zkchanneldb.GetCustState(zkCustDB, zkChannelName)
	err = json.Unmarshal(custStateBytes, &custState)

	var channelState libzkchannels.ChannelState
	channelStateBytes, err := zkchanneldb.GetCustField(zkCustDB, zkChannelName, "channelStateKey")
	err = json.Unmarshal(channelStateBytes, &channelState)

	var channelToken libzkchannels.ChannelToken
	channelTokenBytes, err := zkchanneldb.GetCustField(zkCustDB, zkChannelName, "channelTokenKey")
	err = json.Unmarshal(channelTokenBytes, &channelToken)

	zkCustDB.Close()

	isOk, custState, err := libzkchannels.PayUnmaskSigsCustomer(channelState, channelToken, maskedTxInputs, custState)

	switch isOk {
	case true:
		zkchLog.Info("PayUnmaskTxCustomer successful")
	case false:
		zkchLog.Info("PayUnmaskTxCustomer failed")
	}

	zkchLog.Debug("After PayUnmaskTxCustomer, custState =>:", *custState.State)

	// Update custState in zkchannelsdb
	zkCustDB, err = zkchanneldb.OpenZkChannelBucket(zkChannelName)

	custStateBytes, _ = json.Marshal(custState)
	zkchanneldb.AddCustState(zkCustDB, zkChannelName, custStateBytes)

	zkCustDB.Close()

	// REVOKE OLD STATE
	// open the zkchanneldb to load custState
	zkCustDB, err = zkchanneldb.OpenZkChannelBucket(zkChannelName)

	var revState libzkchannels.RevokedState
	revStateBytes, err := zkchanneldb.GetCustField(zkCustDB, zkChannelName, "revStateKey")
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

	var merchState libzkchannels.MerchState
	merchStateBytes, err := zkchanneldb.GetMerchState(zkMerchDB)
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

func (z *zkChannelManager) processZkPayTokenMask(msg *lnwire.ZkPayTokenMask, p lnpeer.Peer, zkChannelName string) {

	payTokenMask := string(msg.PayTokenMask)
	payTokenMaskR := string(msg.PayTokenMaskR)

	zkchLog.Info("Just received PayTokenMask and PayTokenMaskR: ", payTokenMask, payTokenMaskR)

	// open the zkchanneldb to load custState
	zkCustDB, err := zkchanneldb.OpenZkChannelBucket(zkChannelName)

	var custState libzkchannels.CustState
	custStateBytes, err := zkchanneldb.GetCustState(zkCustDB, zkChannelName)
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
	zkCustDB, err = zkchanneldb.OpenZkChannelBucket(zkChannelName)

	custStateBytes, _ = json.Marshal(custState)
	zkchanneldb.AddCustState(zkCustDB, zkChannelName, custStateBytes)

	zkCustDB.Close()
}

// CloseZkChannel broadcasts a close transaction
func (z *zkChannelManager) CloseZkChannel(wallet *lnwallet.LightningWallet, notifier chainntnfs.ChainNotifier, zkChannelName string) {

	// Add a flag to zkchannelsdb to say that closeChannel has been initiated.
	// This is used to prevent another payment being made
	closeInitiated := true

	zkCustDB, err := zkchanneldb.OpenZkChannelBucket(zkChannelName)
	if err != nil {
		log.Fatal(err)
	}

	closeInitiatedBytes, _ := json.Marshal(closeInitiated)
	zkchanneldb.AddCustField(zkCustDB, zkChannelName, closeInitiatedBytes, "closeInitiatedKey")
	zkCustDB.Close()

	// open the zkchanneldb to load custState
	zkCustDB, err = zkchanneldb.OpenZkChannelBucket(zkChannelName)

	custStateBytes, err := zkchanneldb.GetCustState(zkCustDB, zkChannelName)
	var custState libzkchannels.CustState
	err = json.Unmarshal(custStateBytes, &custState)

	channelStateBytes, err := zkchanneldb.GetCustField(zkCustDB, zkChannelName, "channelStateKey")
	var channelState libzkchannels.ChannelState
	err = json.Unmarshal(channelStateBytes, &channelState)

	channelTokenBytes, err := zkchanneldb.GetCustField(zkCustDB, zkChannelName, "channelTokenKey")
	var channelToken libzkchannels.ChannelToken
	err = json.Unmarshal(channelTokenBytes, &channelToken)

	zkCustDB.Close()

	fromEscrow := true

	CloseEscrowTx, CloseEscrowTxid, err := libzkchannels.CustomerCloseTx(channelState, channelToken, fromEscrow, custState)

	zkchLog.Debug("Signed CloseEscrowTx =>:", CloseEscrowTx)
	zkchLog.Debug("CloseEscrowTx =>:", CloseEscrowTxid)

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

	// Start watching for on-chain notifications of merchClose
	pkScript := msgTx.TxOut[0].PkScript

	zkchLog.Debugf("\nwaitForFundingWithTimeout\npkScript: %#x\n\n", pkScript)

	// TEMPORARY CODE TO FLIP BYTES
	s := ""
	for i := 0; i < len(CloseEscrowTxid)/2; i++ {
		s = CloseEscrowTxid[i*2:i*2+2] + s
	}
	CloseEscrowTxid = s

	confChannel, err := z.waitForFundingWithTimeout(notifier, CloseEscrowTxid, pkScript)
	if err != nil {
		zkchLog.Infof("error waiting for funding "+
			"confirmation: %v", err)
	}

	zkchLog.Debugf("\n\n%#v\n", confChannel)
	zkchLog.Infof("\n\nCustClose (from escrow) close transaction has 3 confirmations\n\n")

}

// MerchClose broadcasts a close transaction for a given escrow txid
func (z *zkChannelManager) MerchClose(wallet *lnwallet.LightningWallet, notifier chainntnfs.ChainNotifier, EscrowTxid string) {

	// open the zkchanneldb to create signedMerchCloseTx
	zkMerchDB, err := zkchanneldb.SetupZkMerchDB()

	var merchState libzkchannels.MerchState
	merchStateBytes, err := zkchanneldb.GetMerchState(zkMerchDB)
	err = json.Unmarshal(merchStateBytes, &merchState)

	zkMerchDB.Close()

	zkchLog.Debug("EscrowTxid to close =>:", EscrowTxid)

	zkchLog.Debugf("\n\nmerchState =>:%+v", merchState)
	zkchLog.Debugf("\n\nCloseTxMap =>:%+v", merchState.CloseTxMap)

	signedMerchCloseTx, merchTxid2, err := libzkchannels.MerchantCloseTx(EscrowTxid, merchState)
	if err != nil {
		zkchLog.Error(err)
	}

	zkchLog.Debug("signedMerchCloseTx =>:", signedMerchCloseTx)
	zkchLog.Debug("signedMerchCloseTxid =>:", merchTxid2)

	// Broadcast escrow tx on chain
	serializedTx, err := hex.DecodeString(signedMerchCloseTx)
	if err != nil {
		zkchLog.Error(err)
	}

	var msgTx wire.MsgTx
	err = msgTx.Deserialize(bytes.NewReader(serializedTx))
	if err != nil {
		zkchLog.Error(err)
	}

	zkchLog.Info("Broadcasting merch close transaction")
	wallet.PublishTransaction(&msgTx)

	// Start watching for on-chain notifications of merchClose
	pkScript := msgTx.TxOut[0].PkScript

	zkchLog.Debugf("\nwaitForFundingWithTimeout\npkScript: %#x\n\n", pkScript)

	confChannel, err := z.waitForFundingWithTimeout(notifier, merchTxid2, pkScript)
	if err != nil {
		zkchLog.Infof("error waiting for funding "+
			"confirmation: %v", err)
	}

	zkchLog.Debugf("\n%#v\n", confChannel)
	zkchLog.Infof("\nMerch close transaction has 3 confirmations\n")

}

// ZkChannelBalance returns the balance on the customer's zkchannel
func ZkChannelBalance(zkChannelName string) (string, int64, int64, error) {

	// open the zkchanneldb to load custState
	zkCustDB, err := zkchanneldb.OpenZkChannelBucket(zkChannelName)
	if err != nil {
		log.Fatal(err)
	}

	var custState libzkchannels.CustState
	custStateBytes, err := zkchanneldb.GetCustState(zkCustDB, zkChannelName)
	err = json.Unmarshal(custStateBytes, &custState)
	if err != nil {
		log.Fatal(err)
	}

	localBalance := custState.CustBalance
	remoteBalance := custState.MerchBalance

	var escrowTxid string
	escrowTxidBytes, err := zkchanneldb.GetCustField(zkCustDB, zkChannelName, "escrowTxidKey")
	err = json.Unmarshal(escrowTxidBytes, &escrowTxid)

	zkCustDB.Close()

	return escrowTxid, localBalance, remoteBalance, nil
}

// TotalReceived returns the balance on the customer's zkchannel
func TotalReceived() (int64, error) {

	zkMerchDB, err := zkchanneldb.SetupZkMerchDB()
	if err != nil {
		log.Fatal(err)
	}

	totalReceivedBytes, err := zkchanneldb.GetMerchField(zkMerchDB, "totalReceivedKey")
	var totalReceived int64
	err = json.Unmarshal(totalReceivedBytes, &totalReceived)

	zkMerchDB.Close()

	return totalReceived, nil
}

// ZkInfo returns info about this zklnd node
func ZkInfo() (string, error) {

	zkMerchDB, err := zkchanneldb.SetupZkMerchDB()
	if err != nil {
		log.Fatal(err)
	}

	merchStateBytes, err := zkchanneldb.GetMerchState(zkMerchDB)
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

// StringInSlice checks if a string exists in a slice of strings
func StringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// ChannelExists checks if a channel with that name has been established
func ChannelExists(zkChannelName string) bool {
	zkChannelList := zkchanneldb.Buckets("zkcust.db")
	return StringInSlice(zkChannelName, zkChannelList)
}
