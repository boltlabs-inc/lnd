package lnd

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/wire"
	"github.com/lightningnetwork/lnd/chainntnfs"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/contractcourt"
	"github.com/lightningnetwork/lnd/libzkchannels"
	"github.com/lightningnetwork/lnd/lncfg"
	"github.com/lightningnetwork/lnd/lnpeer"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/zkchanneldb"
	"github.com/stretchr/testify/assert"
)

type zkTestNode struct {
	msgChan         chan lnwire.Message
	publTxChan      chan *wire.MsgTx
	disconnectPeer  chan *btcec.PublicKey
	zkChannelMgr    *zkChannelManager
	mockNotifier    *mockNotifier
	testDir         string
	shutdownChannel chan struct{}

	remotePeer  *zkTestNode
	sendMessage func(lnwire.Message) error
}

var _ lnpeer.Peer = (*zkTestNode)(nil)

func (z zkTestNode) SendMessage(_ bool, msgs ...lnwire.Message) error {
	return z.sendMessage(msgs[0])
}

func (z zkTestNode) SendMessageLazy(sync bool, msgs ...lnwire.Message) error {
	return z.SendMessage(sync, msgs...)
}

func (z zkTestNode) AddNewChannel(channel *channeldb.OpenChannel, quit <-chan struct{}) error {
	return nil
}

func (z *zkTestNode) WipeChannel(_ *wire.OutPoint) {}

func (z zkTestNode) PubKey() [33]byte {
	return [33]byte{}
}

func (z zkTestNode) IdentityKey() *btcec.PublicKey {
	return (*btcec.PublicKey)(nil)
}

func (z zkTestNode) Address() net.Addr {
	return (net.Addr)(nil)
}

func (z zkTestNode) QuitSignal() <-chan struct{} {
	return (<-chan struct{})(nil)
}

func (z zkTestNode) LocalFeatures() *lnwire.FeatureVector {
	return (*lnwire.FeatureVector)(nil)
}

func (z zkTestNode) RemoteFeatures() *lnwire.FeatureVector {
	return (*lnwire.FeatureVector)(nil)
}

func createTestZkChannelManager(t *testing.T, isMerchant bool) (*zkTestNode, error) {
	sentMessages := make(chan lnwire.Message)

	zkChainWatcher := func(z contractcourt.ZkChainWatcherConfig) error {
		return nil
	}
	testDir, err := ioutil.TempDir("", "zkchanneldb")
	if err != nil {
		t.Fatalf("unable to create temp directory: %v", err)
	}
	publTxChan := make(chan *wire.MsgTx, 1)
	publishTransaction := func(txn *wire.MsgTx, label string) error {
		publTxChan <- txn
		return nil
	}

	peerDisconnected := make(chan *btcec.PublicKey, 1)
	disconnectPeer := func(pubKey *btcec.PublicKey) error {
		peerDisconnected <- pubKey
		return nil
	}

	chainNotifier := &mockNotifier{
		oneConfChannel: make(chan *chainntnfs.TxConfirmation, 1),
		sixConfChannel: make(chan *chainntnfs.TxConfirmation, 1),
		epochChan:      make(chan *chainntnfs.BlockEpoch, 2),
	}
	manager := newZkChannelManager(isMerchant, zkChainWatcher, testDir, publishTransaction, disconnectPeer, lncfg.SelfDelay, lncfg.MinFee, lncfg.MaxFee, lncfg.ValCpfp, lncfg.BalMinCust, lncfg.BalMinMerch)
	return &zkTestNode{
		zkChannelMgr:   manager,
		msgChan:        sentMessages,
		publTxChan:     publTxChan,
		disconnectPeer: peerDisconnected,
		mockNotifier:   chainNotifier,
		testDir:        testDir,
	}, nil
}

func setupZkChannelManagers(t *testing.T) (*zkTestNode, *zkTestNode) {

	cust, err := createTestZkChannelManager(t, false)
	if err != nil {
		t.Fatalf("failed creating fundingManager: %v", err)
	}
	err = cust.zkChannelMgr.initCustomer()
	if err != nil {
		t.Fatalf("failed creating fundingManager: %v", err)
	}

	merch, err := createTestZkChannelManager(t, true)
	if err != nil {
		t.Fatalf("failed creating fundingManager: %v", err)
	}

	skM := "e6e0c5310bb03809e1b2a1595a349f002125fa557d481e51f401ddaf3287e6ae"
	payoutSkM := "5611111111111111111111111111111100000000000000000000000000000000"
	disputeSkM := "5711111111111111111111111111111100000000000000000000000000000000"
	err = merch.zkChannelMgr.initMerchant("Merchant", skM, payoutSkM, disputeSkM)
	if err != nil {
		t.Fatalf("failed creating fundingManager: %v", err)
	}

	// With the funding manager's created, we'll now attempt to mimic a
	// connection pipe between them. In order to intercept the messages
	// within it, we'll redirect all messages back to the msgChan of the
	// sender. Since the fundingManager now has a reference to peers itself,
	// cust.sendMessage will be triggered when Bob's funding manager
	// attempts to send a message to Alice and vice versa.
	cust.remotePeer = merch
	cust.sendMessage = func(msg lnwire.Message) error {
		select {
		case cust.remotePeer.msgChan <- msg:
		case <-cust.shutdownChannel:
			return errors.New("shutting down")
		}
		return nil
	}

	merch.remotePeer = cust
	merch.sendMessage = func(msg lnwire.Message) error {
		select {
		case merch.remotePeer.msgChan <- msg:
		case <-merch.shutdownChannel:
			return errors.New("shutting down")
		}
		return nil
	}

	return cust, merch
}

func tearDownZkChannelManagers(a, b *zkTestNode) {
	if a.shutdownChannel != nil {
		close(a.shutdownChannel)
	}
	if b.shutdownChannel != nil {
		close(b.shutdownChannel)
	}

	if a.testDir != "" {
		os.RemoveAll(a.testDir)
	}
	if b.testDir != "" {
		os.RemoveAll(b.testDir)
	}
}

func TestZkChannelManagerNormalWorkflow(t *testing.T) {
	cust, merch := setupZkChannelManagers(t)
	defer tearDownZkChannelManagers(cust, merch)

	zkChannelName := "testChannel"
	merchPk := "038c2add1dc8cf2c57bac6e19d1f963e0c42103554e8b35e425bc2a78f4c22b273"

	inputSats := int64(50 * 100000000)
	custUtxoTxid_LE := "21779e66bdf89e943ae5b16ae63240a41c5e6ab937dde7b5811c64f13729bb03"
	index := uint32(0)
	custInputSk := "5511111111111111111111111111111100000000000000000000000000000000"
	custStateSk := "1a1971e1379beec67178509e25b6772c66cb67bb04d70df2b4bcdb8c08a01827"
	custPayoutSk := "4157697b6428532758a9d0f9a73ce58befe3fd665797427d1c5bb3d33f6a132e"
	changePk := "037bed6ab680a171ef2ab564af25eff15c0659313df0bbfb96414da7c7d1e65882"

	custBal := int64(1000000)
	merchBal := int64(1000000)
	feeCC := int64(1000)
	feeMC := int64(1000)

	go cust.zkChannelMgr.initZkEstablish(inputSats, custUtxoTxid_LE, index, custInputSk, custStateSk, custPayoutSk, changePk, merchPk, zkChannelName, custBal, merchBal, feeCC, feeMC, merch)

	var msg1 lnwire.Message
	select {
	case msg1 = <-cust.msgChan:
	case <-time.After(time.Second * 5):
		t.Fatalf("peer did not send message")
	}
	msg1Type := lnwire.ZkEstablishOpen{}
	assert.Equal(t, msg1Type.MsgType(), msg1.MsgType())

	// TODO: Check what exactly this does
	ZkEstablishOpenMsg, ok := msg1.(*lnwire.ZkEstablishOpen)
	if !ok {
		errorMsg, gotError := msg1.(*lnwire.Error)
		if gotError {
			t.Fatalf("expected ZkEstablishOpen to be sent "+
				"from merch, instead got error: %v",
				errorMsg.Error())
		}
		t.Fatalf("expected ZkEstablishOpen to be sent from "+
			"cust, instead got %T", msg1)
	}

	go merch.zkChannelMgr.processZkEstablishOpen(ZkEstablishOpenMsg, cust)

	var msg2 lnwire.Message
	select {
	case msg2 = <-merch.msgChan:
	case <-time.After(time.Second * 5):
		t.Fatalf("peer did not send message")
	}
	msg2Type := lnwire.ZkEstablishAccept{}
	assert.Equal(t, msg2Type.MsgType(), msg2.MsgType())

	ZkEstablishAcceptMsg, ok := msg2.(*lnwire.ZkEstablishAccept)
	if !ok {
		errorMsg, gotError := msg2.(*lnwire.Error)
		if gotError {
			t.Fatalf("expected ZkEstablishAccept to be sent "+
				"from merch, instead got error: %v",
				errorMsg.Error())
		}
		t.Fatalf("expected ZkEstablishAccept to be sent from "+
			"cust, instead got %T", msg2)
	}

	go cust.zkChannelMgr.processZkEstablishAccept(ZkEstablishAcceptMsg, merch, zkChannelName)

	var msg3 lnwire.Message
	select {
	case msg3 = <-cust.msgChan:
	case <-time.After(time.Second * 5):
		t.Fatalf("peer did not send message")
	}
	msg3Type := lnwire.ZkEstablishMCloseSigned{}
	assert.Equal(t, msg3Type.MsgType(), msg3.MsgType())

	ZkEstablishMCloseSignedMsg, ok := msg3.(*lnwire.ZkEstablishMCloseSigned)
	if !ok {
		errorMsg, gotError := msg3.(*lnwire.Error)
		if gotError {
			t.Fatalf("expected ZkEstablishMCloseSigned to be sent "+
				"from merch, instead got error: %v",
				errorMsg.Error())
		}
		t.Fatalf("expected ZkEstablishMCloseSigned to be sent from "+
			"cust, instead got %T", msg3)
	}

	go merch.zkChannelMgr.processZkEstablishMCloseSigned(ZkEstablishMCloseSignedMsg, cust)

	var msg4 lnwire.Message
	select {
	case msg4 = <-merch.msgChan:
	case <-time.After(time.Second * 5):
		t.Fatalf("peer did not send message")
	}
	msg4Type := lnwire.ZkEstablishCCloseSigned{}
	assert.Equal(t, msg4Type.MsgType(), msg4.MsgType())

	ZkEstablishCCloseSignedMsg, ok := msg4.(*lnwire.ZkEstablishCCloseSigned)
	if !ok {
		errorMsg, gotError := msg4.(*lnwire.Error)
		if gotError {
			t.Fatalf("expected ZkEstablishCCloseSigned to be sent "+
				"from merch, instead got error: %v",
				errorMsg.Error())
		}
		t.Fatalf("expected ZkEstablishCCloseSigned to be sent from "+
			"cust, instead got %T", msg4)
	}

	go cust.zkChannelMgr.processZkEstablishCCloseSigned(ZkEstablishCCloseSignedMsg, merch, zkChannelName)

	var msg5 lnwire.Message
	select {
	case msg5 = <-cust.msgChan:
	case <-time.After(time.Second * 5):
		t.Fatalf("peer did not send message")
	}
	msg5Type := lnwire.ZkEstablishInitialState{}
	assert.Equal(t, msg5Type.MsgType(), msg5.MsgType())

	ZkEstablishInitialStateMsg, ok := msg5.(*lnwire.ZkEstablishInitialState)
	if !ok {
		errorMsg, gotError := msg5.(*lnwire.Error)
		if gotError {
			t.Fatalf("expected ZkEstablishInitialState to be sent "+
				"from merch, instead got error: %v",
				errorMsg.Error())
		}
		t.Fatalf("expected ZkEstablishInitialState to be sent from "+
			"cust, instead got %T", msg5)
	}

	go merch.zkChannelMgr.processZkEstablishInitialState(ZkEstablishInitialStateMsg, cust, merch.mockNotifier)

	var msg6 lnwire.Message
	select {
	case msg6 = <-merch.msgChan:
	case <-time.After(time.Second * 5):
		t.Fatalf("peer did not send message")
	}
	msg6Type := lnwire.ZkEstablishStateValidated{}
	assert.Equal(t, msg6Type.MsgType(), msg6.MsgType())

	ZkEstablishStateValidatedMsg, ok := msg6.(*lnwire.ZkEstablishStateValidated)
	if !ok {
		errorMsg, gotError := msg6.(*lnwire.Error)
		if gotError {
			t.Fatalf("expected ZkEstablishStateValidatedMsg to be sent "+
				"from merch, instead got error: %v",
				errorMsg.Error())
		}
		t.Fatalf("expected ZkEstablishStateValidatedMsg to be sent from "+
			"cust, instead got %T", msg6)
	}

	go cust.zkChannelMgr.processZkEstablishStateValidated(ZkEstablishStateValidatedMsg, merch,
		zkChannelName, cust.mockNotifier)

	// Get and return the funding transaction cust published to the network.
	var fundingTx *wire.MsgTx
	select {
	case fundingTx = <-cust.publTxChan:
	case <-time.After(time.Second * 5):
		t.Fatalf("cust did not publish funding tx")
	}

	// Simulate confirmations on escrow tx for merch and cust
	// 'oneConfChannel' is sufficient unless sixConfChannel is required
	merch.mockNotifier.oneConfChannel <- &chainntnfs.TxConfirmation{
		Tx: fundingTx,
	}
	cust.mockNotifier.oneConfChannel <- &chainntnfs.TxConfirmation{
		Tx: fundingTx,
	}

	var msg7 lnwire.Message
	select {
	case msg7 = <-cust.msgChan:
	case <-time.After(time.Second * 5):
		t.Fatalf("peer did not send message")
	}
	msg7Type := lnwire.ZkEstablishFundingLocked{}
	assert.Equal(t, msg7Type.MsgType(), msg7.MsgType())

	ZkEstablishFundingLockedMsg, ok := msg7.(*lnwire.ZkEstablishFundingLocked)
	if !ok {
		errorMsg, gotError := msg7.(*lnwire.Error)
		if gotError {
			t.Fatalf("expected ZkEstablishFundingLocked to be sent "+
				"from merch, instead got error: %v",
				errorMsg.Error())
		}
		t.Fatalf("expected ZkEstablishFundingLocked to be sent from "+
			"cust, instead got %T", msg7)
	}

	go merch.zkChannelMgr.processZkEstablishFundingLocked(ZkEstablishFundingLockedMsg, cust)

	var msg8 lnwire.Message
	select {
	case msg8 = <-merch.msgChan:
	case <-time.After(time.Second * 5):
		t.Fatalf("peer did not send message")
	}
	msg8Type := lnwire.ZkEstablishFundingConfirmed{}
	assert.Equal(t, msg8Type.MsgType(), msg8.MsgType())

	ZkEstablishFundingConfirmedMsg, ok := msg8.(*lnwire.ZkEstablishFundingConfirmed)
	if !ok {
		errorMsg, gotError := msg8.(*lnwire.Error)
		if gotError {
			t.Fatalf("expected ZkEstablishFundingConfirmedMsg to be sent "+
				"from merch, instead got error: %v",
				errorMsg.Error())
		}
		t.Fatalf("expected ZkEstablishFundingConfirmedMsg to be sent from "+
			"cust, instead got %T", msg8)
	}

	go cust.zkChannelMgr.processZkEstablishFundingConfirmed(ZkEstablishFundingConfirmedMsg, merch, zkChannelName)

	var msg9 lnwire.Message
	select {
	case msg9 = <-cust.msgChan:
	case <-time.After(time.Second * 5):
		t.Fatalf("peer did not send message")
	}
	msg9Type := lnwire.ZkEstablishCustActivated{}
	assert.Equal(t, msg9Type.MsgType(), msg9.MsgType())

	ZkEstablishCustActivatedMsg, ok := msg9.(*lnwire.ZkEstablishCustActivated)
	if !ok {
		errorMsg, gotError := msg9.(*lnwire.Error)
		if gotError {
			t.Fatalf("expected ZkEstablishCustActivated to be sent "+
				"from merch, instead got error: %v",
				errorMsg.Error())
		}
		t.Fatalf("expected ZkEstablishCustActivated to be sent from "+
			"cust, instead got %T", msg9)
	}

	go merch.zkChannelMgr.processZkEstablishCustActivated(ZkEstablishCustActivatedMsg, cust)

	var msg10 lnwire.Message
	select {
	case msg10 = <-merch.msgChan:
	case <-time.After(time.Second * 5):
		t.Fatalf("peer did not send message")
	}
	msg10Type := lnwire.ZkEstablishPayToken{}
	assert.Equal(t, msg10Type.MsgType(), msg10.MsgType())

	ZkEstablishPayTokenMsg, ok := msg10.(*lnwire.ZkEstablishPayToken)
	if !ok {
		errorMsg, gotError := msg10.(*lnwire.Error)
		if gotError {
			t.Fatalf("expected ZkEstablishPayTokenMsg to be sent "+
				"from merch, instead got error: %v",
				errorMsg.Error())
		}
		t.Fatalf("expected ZkEstablishPayTokenMsg to be sent from "+
			"cust, instead got %T", msg10)
	}

	go cust.zkChannelMgr.processZkEstablishPayToken(ZkEstablishPayTokenMsg, merch, zkChannelName)

	var disconnectMsg *btcec.PublicKey
	select {
	case disconnectMsg = <-cust.disconnectPeer:
		t.Logf("NodeID of disconnected merchant: %v", disconnectMsg)
	case <-time.After(time.Second * 5):
		t.Fatalf("Customer did not disconnect from merchant at the end of establish")
	}

}

func setupLibzkChannels(t *testing.T, zkChannelName string, custDBPath string, merchDBPath string) {

	// Load MerchState and ChannelState
	zkMerchDB, err := zkchanneldb.SetupMerchDB(merchDBPath)
	if err != nil {
		t.Fatalf("%v", err)
	}

	merchState, err := zkchanneldb.GetMerchState(zkMerchDB)
	if err != nil {
		t.Fatalf("%v", err)
	}

	var channelState libzkchannels.ChannelState
	err = zkchanneldb.GetMerchField(zkMerchDB, channelStateKey, &channelState)
	if err != nil {
		t.Fatalf("%v", err)
	}

	err = zkMerchDB.Close()
	if err != nil {
		t.Fatalf("%v", err)
	}

	txFeeInfo := libzkchannels.TransactionFeeInfo{
		BalMinCust:  lncfg.BalMinCust,
		BalMinMerch: lncfg.BalMinMerch,
		ValCpFp:     lncfg.ValCpfp,
		FeeCC:       1000,
		FeeMC:       1000,
		MinFee:      lncfg.MinFee,
		MaxFee:      lncfg.MaxFee,
	}
	feeCC := txFeeInfo.FeeCC
	feeMC := txFeeInfo.FeeMC

	custBal := int64(1000000)
	merchBal := int64(1000000)

	merchPKM := fmt.Sprintf("%v", *merchState.PkM)

	skC := "1a1971e1379beec67178509e25b6772c66cb67bb04d70df2b4bcdb8c08a01827"
	payoutSk := "4157697b6428532758a9d0f9a73ce58befe3fd665797427d1c5bb3d33f6a132e"

	channelToken, custState, err := libzkchannels.InitCustomer(merchPKM, custBal, merchBal, txFeeInfo, "cust")
	if err != nil {
		t.Fatalf("%v", err)
	}

	channelToken, custState, err = libzkchannels.LoadCustomerWallet(custState, channelToken, skC, payoutSk)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// details about input utxo
	inputSats := int64(100000000)
	cust_utxo_txid := "e8aed42b9f07c74a3ce31a9417146dc61eb8611a1e66d345fd69be06b644278d"
	cust_utxo_index := uint32(0)
	custInputSk := "5511111111111111111111111111111100000000000000000000000000000000"

	custSk := fmt.Sprintf("%v", custState.SkC)
	custPk := fmt.Sprintf("%v", custState.PkC)
	// merchSk := fmt.Sprintf("%v", *merchState.SkM)
	merchPk := fmt.Sprintf("%v", *merchState.PkM)
	t.Log("merchPk", merchPk)
	// changeSk := "4157697b6428532758a9d0f9a73ce58befe3fd665797427d1c5bb3d33f6a132e"
	changePk := "037bed6ab680a171ef2ab564af25eff15c0659313df0bbfb96414da7c7d1e65882"

	merchClosePk := fmt.Sprintf("%v", *merchState.PayoutPk)
	toSelfDelay, err := libzkchannels.GetSelfDelayBE(channelState)
	t.Log("toSelfDelay", toSelfDelay)
	if err != nil {
		t.Fatalf("%v", err)
	}

	outputSats := custBal + merchBal
	// escrowTxid_BE, escrowTxid_LE, escrowPrevout, err := FormEscrowTx(cust_utxo_txid, 0, custSk, inputSats, outputSats, custPk, merchPk, changePk, false)
	txFee := int64(500)
	signedEscrowTx, escrowTxid_BE, escrowTxid_LE, escrowPrevout, err := libzkchannels.SignEscrowTx(cust_utxo_txid, cust_utxo_index, custInputSk, inputSats, outputSats, custPk, merchPk, changePk, false, txFee)
	if err != nil {
		t.Fatalf("%v", err)
	}
	_ = signedEscrowTx

	merchTxPreimage, err := libzkchannels.FormMerchCloseTx(escrowTxid_LE, custPk, merchPk, merchClosePk, custBal, merchBal, feeMC, txFeeInfo.ValCpFp, toSelfDelay)
	if err != nil {
		t.Fatalf("%v", err)
	}

	custSig, err := libzkchannels.CustomerSignMerchCloseTx(custSk, merchTxPreimage)
	if err != nil {
		t.Fatalf("%v", err)
	}

	isOk, merchTxid_BE, merchTxid_LE, merchPrevout, merchState, err := libzkchannels.MerchantVerifyMerchCloseTx(escrowTxid_LE, custPk, custBal, merchBal, feeMC, txFeeInfo.ValCpFp, toSelfDelay, custSig, merchState)
	if err != nil {
		t.Fatalf("%v", err)
	}
	_ = merchTxid_LE

	// initiate merch-close-tx
	signedMerchCloseTx, merchTxid2_BE, merchTxid2_LE, merchState, err := libzkchannels.ForceMerchantCloseTx(escrowTxid_LE, merchState, txFeeInfo.ValCpFp)
	if err != nil {
		t.Fatalf("%v", err)
	}
	_ = merchState
	_ = signedMerchCloseTx
	_ = merchTxid2_BE
	_ = merchTxid2_LE

	txInfo := libzkchannels.FundingTxInfo{
		EscrowTxId:    escrowTxid_BE, // big-endian
		EscrowPrevout: escrowPrevout, // big-endian
		MerchTxId:     merchTxid_BE,  // big-endian
		MerchPrevout:  merchPrevout,  // big-endian
		InitCustBal:   custBal,
		InitMerchBal:  merchBal,
	}

	custClosePk := custState.PayoutPk
	escrowSig, merchSig, err := libzkchannels.MerchantSignInitCustCloseTx(txInfo, custState.RevLock, custState.PkC, custClosePk, toSelfDelay, merchState, feeCC, feeMC, txFeeInfo.ValCpFp)
	if err != nil {
		t.Fatalf("%v", err)
	}

	isOk, channelToken, custState, err = libzkchannels.CustomerVerifyInitCustCloseTx(txInfo, txFeeInfo, channelState, channelToken, escrowSig, merchSig, custState)
	if err != nil {
		t.Fatalf("%v", err)
	}

	initCustState, initHash, err := libzkchannels.CustomerGetInitialState(custState)
	if err != nil {
		t.Fatalf("%v", err)
	}

	isOk, merchState, err = libzkchannels.MerchantValidateInitialState(channelToken, initCustState, initHash, merchState)
	if !isOk {
		fmt.Println("error: ", err)
	}

	CloseEscrowTx, CloseEscrowTxId_LE, custState, err := libzkchannels.ForceCustomerCloseTx(channelState, channelToken, true, custState)
	if err != nil {
		t.Fatalf("%v", err)
	}
	_ = CloseEscrowTx
	_ = CloseEscrowTxId_LE

	CloseMerchTx, CloseMerchTxId_LE, custState, err := libzkchannels.ForceCustomerCloseTx(channelState, channelToken, false, custState)
	if err != nil {
		t.Fatalf("%v", err)
	}
	_ = CloseMerchTx
	_ = CloseMerchTxId_LE
	// End of libzkchannels_test.go

	// Save variables needed to create cust close in zkcust.db
	zkCustDB, err := zkchanneldb.CreateZkChannelBucket(zkChannelName, custDBPath)
	if err != nil {
		t.Fatalf("%v", err)
	}

	err = zkchanneldb.AddCustState(zkCustDB, zkChannelName, custState)
	if err != nil {
		t.Fatalf("%v", err)
	}

	err = zkchanneldb.AddField(zkCustDB, zkChannelName, channelState, channelStateKey)
	if err != nil {
		t.Fatalf("%v", err)
	}

	err = zkchanneldb.AddField(zkCustDB, zkChannelName, channelToken, channelTokenKey)
	if err != nil {
		t.Fatalf("%v", err)
	}

	err = zkchanneldb.AddField(zkCustDB, zkChannelName, escrowTxid, escrowTxidKey)
	if err != nil {
		t.Fatalf("%v", err)
	}

	err = zkCustDB.Close()
	if err != nil {
		zkchLog.Error(err)
	}

	// Save variables needed to create merch close in zkmerch.db
	zkMerchDB, err = zkchanneldb.OpenMerchBucket(merchDBPath)
	if err != nil {
		t.Fatalf("%v", err)
	}

	err = zkchanneldb.AddMerchState(zkMerchDB, merchState)
	if err != nil {
		t.Fatalf("%v", err)
	}

	err = zkchanneldb.AddMerchField(zkMerchDB, escrowTxid_LE, escrowTxidKey)
	if err != nil {
		t.Fatalf("%v", err)
	}

	err = zkchanneldb.AddMerchField(zkMerchDB, channelState, channelStateKey)
	if err != nil {
		t.Fatalf("%v", err)
	}

	err = zkchanneldb.AddMerchField(zkMerchDB, channelToken, channelTokenKey)
	if err != nil {
		t.Fatalf("%v", err)
	}

	err = zkMerchDB.Close()
	if err != nil {
		zkchLog.Error(err)
	}

}

func TestCloseZkChannel(t *testing.T) {

	cust, merch := setupZkChannelManagers(t)
	defer tearDownZkChannelManagers(cust, merch)

	setupLibzkChannels(t, "myChannel", cust.zkChannelMgr.dbPath, merch.zkChannelMgr.dbPath)
	cust.zkChannelMgr.CloseZkChannel(cust.mockNotifier, "myChannel", false)

	// Get and return the cust-close tx cust published to the network.
	var custCloseTx *wire.MsgTx
	select {
	case custCloseTx = <-cust.publTxChan:
	case <-time.After(time.Second * 5):
		t.Fatalf("cust did not publish custClose tx")
	}
	_ = custCloseTx

	// // TODO ZKLND-11 - make sure channel is removed after channel closure
	// // has been confirmed on chain
	// cust.mockNotifier.oneConfChannel <- &chainntnfs.TxConfirmation{
	// 	Tx: custCloseTx,
	// }
}

func TestMerchClose(t *testing.T) {

	cust, merch := setupZkChannelManagers(t)
	defer tearDownZkChannelManagers(cust, merch)

	setupLibzkChannels(t, "myChannel", cust.zkChannelMgr.dbPath, merch.zkChannelMgr.dbPath)

	// open the zkchanneldb to load merchState
	zkMerchDB, err := zkchanneldb.OpenMerchBucket(merch.zkChannelMgr.dbPath)
	if err != nil {
		t.Fatalf("%v", err)
	}

	var escrowTxid string
	err = zkchanneldb.GetMerchField(zkMerchDB, escrowTxidKey, &escrowTxid)
	if err != nil {
		t.Fatalf("%v", err)
	}

	err = zkMerchDB.Close()
	if err != nil {
		t.Fatalf("%v", err)
	}

	merch.zkChannelMgr.MerchClose(merch.mockNotifier, escrowTxid)

	// Get and return the merch-close tx merch published to the network.
	var merchCloseTx *wire.MsgTx
	select {
	case merchCloseTx = <-merch.publTxChan:
	case <-time.After(time.Second * 5):
		t.Fatalf("merch did not publish merchClose tx")
	}
	_ = merchCloseTx

	// // TODO ZKLND-11 - make sure channel is removed after channel closure
	// // has been confirmed on chain
	// merch.mockNotifier.oneConfChannel <- &chainntnfs.TxConfirmation{
	// 	Tx: merchCloseTx,
	// }
}

func addAmountToTotalReceieved(t *testing.T, dbPath string, amount int64) error {
	zkMerchDB, err := zkchanneldb.OpenMerchBucket(dbPath)
	if err != nil {
		t.Fatalf("%v", err)
	}

	var totalReceived Total
	err = zkchanneldb.GetMerchField(zkMerchDB, totalReceivedKey, &totalReceived)
	if err != nil {
		t.Fatalf("%v", err)
	}

	totalReceived.Amount += amount

	err = zkchanneldb.AddMerchField(zkMerchDB, totalReceived, totalReceivedKey)
	if err != nil {
		t.Fatalf("%v", err)
	}

	err = zkMerchDB.Close()
	if err != nil {
		t.Fatalf("%v", err)
	}

	return err
}

func addDummyChannelToken(t *testing.T, dbPath string) error {
	zkchannels := make(map[string]libzkchannels.ChannelToken)

	channelToken := libzkchannels.ChannelToken{
		PkC:        "0217d55a1e3ecdd220fde4bddbbfd485a1596c0c5cb7ef11dbfcdb2dd9cf4b85af",
		PkM:        "038c2add1dc8cf2c57bac6e19d1f963e0c42103554e8b35e425bc2a78f4c22b273",
		EscrowTxId: "a9f308e24254b83ed42c23db99dd8cdd4b3f2e47080c46ed897a53418d9788f4",
		MerchTxId:  "f2acaa74600d1d67ec33053a08fce0bca17a769d56092a503772105afa905182",
	}
	channelID, err := libzkchannels.GetChannelId(channelToken)
	if err != nil {
		t.Fatalf("%v", err)
	}
	zkchannels[channelID] = channelToken

	zkMerchDB, err := zkchanneldb.OpenMerchBucket(dbPath)
	if err != nil {
		t.Fatalf("%v", err)
	}

	err = zkchanneldb.AddMerchField(zkMerchDB, zkchannels, zkChannelsKey)
	if err != nil {
		t.Fatalf("%v", err)
	}

	err = zkMerchDB.Close()
	if err != nil {
		t.Fatalf("%v", err)
	}
	return nil
}

func TestCliCommands(t *testing.T) {

	cust, merch := setupZkChannelManagers(t)
	defer tearDownZkChannelManagers(cust, merch)

	setupLibzkChannels(t, "myChannel", cust.zkChannelMgr.dbPath, merch.zkChannelMgr.dbPath)

	// Test that ZkInfo returns pubkey (hex) of len 66
	merchPk, err := merch.zkChannelMgr.ZkInfo()
	if err != nil {
		t.Errorf("%v", err)
	}

	assert.Equal(t, 66, len(merchPk), "ZkInfo returned a pubkey (hex) with bad length")

	// Test that TotalReceived updates correctly
	paymentAmount := int64(1000)
	amount0, err := merch.zkChannelMgr.TotalReceived()
	if err != nil {
		t.Errorf("%v", err)
	}

	addAmountToTotalReceieved(t, merch.zkChannelMgr.dbPath, paymentAmount)

	amount1, err := merch.zkChannelMgr.TotalReceived()
	if err != nil {
		t.Errorf("%v", err)
	}

	assert.Equal(t, paymentAmount, amount1-amount0, "TotalReceived failed to update amount correctly")

	// Test ListZkChannels
	ListOfZkChannels, err := merch.zkChannelMgr.ListZkChannels()
	if err != nil {
		t.Errorf("%v", err)
	}
	numChannels0 := len(ListOfZkChannels.ChannelID)
	assert.Equal(t, 0, numChannels0, "Non-empty list of zkchannels when Merchant is initialized")

	addDummyChannelToken(t, merch.zkChannelMgr.dbPath)

	ListOfZkChannels, err = merch.zkChannelMgr.ListZkChannels()
	if err != nil {
		t.Errorf("%v", err)
	}
	numChannels1 := len(ListOfZkChannels.ChannelID)

	assert.Equal(t, 1, numChannels1, "ListZkChannels did not return the added channel")

	// Test ZkChannelBalance
	escrowTxid, localBalance, remoteBalance, err := cust.zkChannelMgr.ZkChannelBalance("myChannel")
	if err != nil {
		t.Errorf("%v", err)
	}
	t.Log("escrowTxid", escrowTxid)
	t.Log("localBalance", localBalance)
	t.Log("remoteBalance", remoteBalance)
}
