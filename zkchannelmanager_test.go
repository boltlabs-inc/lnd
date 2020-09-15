package lnd

import (
	"errors"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"github.com/lightningnetwork/lnd/chainntnfs"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/contractcourt"
	"github.com/lightningnetwork/lnd/libzkchannels"
	"github.com/lightningnetwork/lnd/lnpeer"
	"github.com/lightningnetwork/lnd/lnwallet/chainfee"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/zkchanneldb"
	"github.com/lightningnetwork/lnd/zkchannels"
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
	feeEstimator := zkchannels.NewMockFeeEstimator(10000, chainfee.FeePerKwFloor)

	chainIO := &mockChainIO{
		bestHeight: fundingBroadcastHeight,
	}

	netParams := activeNetParams.Params
	estimator := chainfee.NewStaticEstimator(62500, 0)

	wc := &mockWalletController{
		rootKey: alicePrivKey,
	}
	signer := &mockSigner{
		key: alicePrivKey,
	}
	bio := &mockChainIO{
		bestHeight: fundingBroadcastHeight,
	}

	dbDir := filepath.Join(testDir, "lnwallet")
	cdb, err := channeldb.Open(dbDir)
	if err != nil {
		return nil, err
	}

	keyRing := &mockSecretKeyRing{
		rootKey: alicePrivKey,
	}

	lnw, err := createTestWallet(
		cdb, netParams, chainNotifier, wc, signer, keyRing, bio,
		estimator,
	)
	if err != nil {
		t.Fatalf("unable to create test ln wallet: %v", err)
	}

	cfg := DefaultConfig()
	manager := newZkChannelManager(&cfg, zkChainWatcher, testDir, publishTransaction, disconnectPeer, feeEstimator, chainIO, lnw)

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
	childSkM := "5811111111111111111111111111111100000000000000000000000000000000"
	disputeSkM := "5711111111111111111111111111111100000000000000000000000000000000"
	err = merch.zkChannelMgr.initMerchant("Merchant", skM, payoutSkM, childSkM, disputeSkM)
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

	txFeeInfo, custBal, merchBal, custStateSk, custPayoutSk := zkchannels.InitCustConstants()
	feeCC := txFeeInfo.FeeCC
	feeMC := txFeeInfo.FeeMC

	inputSats, cust_utxo_txid, index, custInputSk, _, changePk, _ := zkchannels.InitFundingUTXO()

	go cust.zkChannelMgr.initZkEstablish(inputSats, cust_utxo_txid, index, custInputSk, custStateSk, custPayoutSk, changePk, merchPk, zkChannelName, custBal, merchBal, feeCC, feeMC, merch)

	var msg1 lnwire.Message
	select {
	case msg1 = <-cust.msgChan:
	case <-time.After(time.Second * 5):
		t.Fatalf("peer did not send message")
	}
	msg1Type := lnwire.ZkEstablishOpen{}
	assert.Equal(t, msg1Type.MsgType(), msg1.MsgType())

	ZkEstablishOpenMsg, ok := msg1.(*lnwire.ZkEstablishOpen)
	if !ok {
		errorMsg, gotError := msg1.(*lnwire.Error)
		if gotError {
			t.Fatalf("expected ZkEstablishOpen to be sent "+
				"from cust, instead got error: %v",
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
			"merch, instead got %T", msg2)
	}

	go cust.zkChannelMgr.processZkEstablishAccept(ZkEstablishAcceptMsg, merch)

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
				"from cust, instead got error: %v",
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
			"merch, instead got %T", msg4)
	}

	go cust.zkChannelMgr.processZkEstablishCCloseSigned(ZkEstablishCCloseSignedMsg, merch)

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
				"from cust, instead got error: %v",
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
			"merch, instead got %T", msg6)
	}

	go cust.zkChannelMgr.processZkEstablishStateValidated(ZkEstablishStateValidatedMsg, merch,
		cust.mockNotifier)

	// Get and return the funding transaction cust published to the network.
	var fundingTx *wire.MsgTx
	select {
	case fundingTx = <-cust.publTxChan:
		// Compare weight of publishedTx vs value used for calculating txFee
		expected := int64(escrowTxKW * 1000) // (upper bound)
		actual := blockchain.GetTransactionWeight(btcutil.NewTx(fundingTx))
		// allow for 1 byte variation in scriptSig signature length
		if d := expected - actual; d > 1 || d < 0 {
			t.Errorf("escrow Tx did not have the expected weight. Expected: %v, got: %v", expected, actual)

		}

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
				"from cust, instead got error: %v",
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
			"merch, instead got %T", msg8)
	}

	go cust.zkChannelMgr.processZkEstablishFundingConfirmed(ZkEstablishFundingConfirmedMsg, merch)

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
				"from cust, instead got error: %v",
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
			"merch, instead got %T", msg10)
	}

	go cust.zkChannelMgr.processZkEstablishPayToken(ZkEstablishPayTokenMsg, merch)

	var disconnectMsg *btcec.PublicKey
	select {
	case disconnectMsg = <-cust.disconnectPeer:
		t.Logf("NodeID of disconnected merchant: %v", disconnectMsg)
	case <-time.After(time.Second * 5):
		t.Fatalf("Customer did not disconnect from merchant at the end of establish")
	}

}

// TestFundingNotConfirmedYet tests that the customer will re-send the
// ZkEstablishFundingLocked message if the merchant has not received
// confirmations of the escrow on chain.
func TestFundingNotConfirmedYet(t *testing.T) {

	cust, merch := setupZkChannelManagers(t)
	defer tearDownZkChannelManagers(cust, merch)

	zkChannelName := "testChannel"
	merchPk := "038c2add1dc8cf2c57bac6e19d1f963e0c42103554e8b35e425bc2a78f4c22b273"

	txFeeInfo, custBal, merchBal, custStateSk, custPayoutSk := zkchannels.InitCustConstants()
	feeCC := txFeeInfo.FeeCC
	feeMC := txFeeInfo.FeeMC

	inputSats, cust_utxo_txid, index, custInputSk, _, changePk, _ := zkchannels.InitFundingUTXO()

	go cust.zkChannelMgr.initZkEstablish(inputSats, cust_utxo_txid, index, custInputSk, custStateSk, custPayoutSk, changePk, merchPk, zkChannelName, custBal, merchBal, feeCC, feeMC, merch)

	var msg1 lnwire.Message
	select {
	case msg1 = <-cust.msgChan:
	case <-time.After(time.Second * 5):
		t.Fatalf("peer did not send message")
	}
	msg1Type := lnwire.ZkEstablishOpen{}
	assert.Equal(t, msg1Type.MsgType(), msg1.MsgType())

	ZkEstablishOpenMsg, ok := msg1.(*lnwire.ZkEstablishOpen)
	if !ok {
		errorMsg, gotError := msg1.(*lnwire.Error)
		if gotError {
			t.Fatalf("expected ZkEstablishOpen to be sent "+
				"from cust, instead got error: %v",
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
			"merch, instead got %T", msg2)
	}

	go cust.zkChannelMgr.processZkEstablishAccept(ZkEstablishAcceptMsg, merch)

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
				"from cust, instead got error: %v",
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
			"merch, instead got %T", msg4)
	}

	go cust.zkChannelMgr.processZkEstablishCCloseSigned(ZkEstablishCCloseSignedMsg, merch)

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
				"from cust, instead got error: %v",
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
			"merch, instead got %T", msg6)
	}

	go cust.zkChannelMgr.processZkEstablishStateValidated(ZkEstablishStateValidatedMsg, merch,
		cust.mockNotifier)

	// Get and return the funding transaction cust published to the network.
	var fundingTx *wire.MsgTx
	select {
	case fundingTx = <-cust.publTxChan:
		// Compare weight of publishedTx vs value used for calculating txFee
		expected := int64(escrowTxKW * 1000) // (upper bound)
		actual := blockchain.GetTransactionWeight(btcutil.NewTx(fundingTx))
		// allow for 1 byte variation in scriptSig signature length
		if d := expected - actual; d > 1 || d < 0 {
			t.Errorf("escrow Tx did not have the expected weight. Expected: %v, got: %v", expected, actual)

		}

	case <-time.After(time.Second * 5):
		t.Fatalf("cust did not publish funding tx")
	}

	// Simulate confirmations on escrow tx for merch and cust
	// 'oneConfChannel' is sufficient unless sixConfChannel is required
	// merch.mockNotifier.oneConfChannel <- &chainntnfs.TxConfirmation{
	// 	Tx: fundingTx,
	// }
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
				"from cust, instead got error: %v",
				errorMsg.Error())
		}
		t.Fatalf("expected ZkEstablishFundingLocked to be sent from "+
			"cust, instead got %T", msg7)
	}

	// Now we will simulate the merchant receiving the 'FundingLocked' message
	// without having confirmed the escrowTx on chain themselves. The merchant
	// should return a 'FundingConfirmed' = false msg, and the customer will
	// attempt to send the 'FundingLocked' message again after a short pause.
	for i := 0; i < 3; i++ {
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
				"merch, instead got %T", msg8)
		}

		go cust.zkChannelMgr.processZkEstablishFundingConfirmed(ZkEstablishFundingConfirmedMsg, merch)

		var msg9 lnwire.Message
		select {
		case msg9 = <-cust.msgChan:
		case <-time.After(time.Second * 50):
			t.Fatalf("peer did not send message")
		}

		msg9Type := lnwire.ZkEstablishFundingLocked{}
		assert.Equal(t, msg9Type.MsgType(), msg9.MsgType())

		ZkEstablishFundingLockedMsg, ok = msg9.(*lnwire.ZkEstablishFundingLocked)
		if !ok {
			errorMsg, gotError := msg9.(*lnwire.Error)
			if gotError {
				t.Fatalf("expected ZkEstablishFundingLocked to be sent "+
					"from cust, instead got error: %v",
					errorMsg.Error())
			}
			t.Fatalf("expected ZkEstablishFundingLocked to be sent from "+
				"cust, instead got %T", msg9)
		}
	}

	// Now we simulate the merchant receiving confirmation of the escrowTx.
	merch.mockNotifier.oneConfChannel <- &chainntnfs.TxConfirmation{
		Tx: fundingTx,
	}
	// Give merchState a moment to update
	time.Sleep(1 * time.Millisecond)

	go merch.zkChannelMgr.processZkEstablishFundingLocked(ZkEstablishFundingLockedMsg, cust)

	var msg12 lnwire.Message
	select {
	case msg12 = <-merch.msgChan:
	case <-time.After(time.Second * 5):
		t.Fatalf("peer did not send message")
	}
	msg12Type := lnwire.ZkEstablishFundingConfirmed{}
	assert.Equal(t, msg12Type.MsgType(), msg12.MsgType())

	ZkEstablishFundingConfirmedMsg, ok := msg12.(*lnwire.ZkEstablishFundingConfirmed)
	if !ok {
		errorMsg, gotError := msg12.(*lnwire.Error)
		if gotError {
			t.Fatalf("expected ZkEstablishFundingConfirmedMsg to be sent "+
				"from merch, instead got error: %v",
				errorMsg.Error())
		}
		t.Fatalf("expected ZkEstablishFundingConfirmedMsg to be sent from "+
			"merch, instead got %T", msg12)
	}

	go cust.zkChannelMgr.processZkEstablishFundingConfirmed(ZkEstablishFundingConfirmedMsg, merch)

	var msg13 lnwire.Message
	select {
	case msg13 = <-cust.msgChan:
	case <-time.After(time.Second * 5):
		t.Fatalf("peer did not send message")
	}
	msg13Type := lnwire.ZkEstablishCustActivated{}
	assert.Equal(t, msg13Type.MsgType(), msg13.MsgType())

	ZkEstablishCustActivatedMsg, ok := msg13.(*lnwire.ZkEstablishCustActivated)
	if !ok {
		errorMsg, gotError := msg13.(*lnwire.Error)
		if gotError {
			t.Fatalf("expected ZkEstablishCustActivated to be sent "+
				"from cust, instead got error: %v",
				errorMsg.Error())
		}
		t.Fatalf("expected ZkEstablishCustActivated to be sent from "+
			"cust, instead got %T", msg13)
	}
	_ = ZkEstablishCustActivatedMsg
}
func TestCloseZkChannel(t *testing.T) {

	cust, merch := setupZkChannelManagers(t)
	defer tearDownZkChannelManagers(cust, merch)

	zkChannelName := "myChannel"
	zkchannels.SetupLibzkChannels(zkChannelName, cust.zkChannelMgr.dbPath, merch.zkChannelMgr.dbPath)
	cust.zkChannelMgr.CloseZkChannel(cust.mockNotifier, zkChannelName, false)

	// Since CloseZkChannel has been called by the customer, we want to make
	// sure that the channel has been marked as 'PendingClose', to prevent
	// the customer making payments on this channel.
	status, err := zkchannels.GetCustChannelState(cust.zkChannelMgr.dbPath, zkChannelName)
	if err != nil {
		t.Errorf("%v", err)
	}
	assert.Equal(t, "PendingClose", status)

	// Get and return the cust-close tx cust published to the network.
	var custCloseTx *wire.MsgTx
	select {
	case custCloseTx = <-cust.publTxChan:
		t.Log("custCloseTx was broadcasted")
		// Compare weight of publishedTx vs value used for calculating txFee
		expected := int64(custCloseTxKW * 1000) // (upper bound)
		actual := blockchain.GetTransactionWeight(btcutil.NewTx(custCloseTx))
		// allow for 2 byte variation in scriptSig signature length (2
		// signatures for multisig).
		if d := expected - actual; d > 2 || d < 0 {
			t.Errorf("custCloseTx did not have the expected weight. Expected: %v, got: %v", expected, actual)
		}
	case <-time.After(time.Second * 5):
		t.Fatalf("cust did not publish custClose tx")
	}

	cust.mockNotifier.oneConfChannel <- &chainntnfs.TxConfirmation{
		Tx: custCloseTx,
	}

	// Give custState a moment to update
	time.Sleep(1 * time.Millisecond)
	// Check that custStateChannelStatus has updated.
	status, err = zkchannels.GetCustChannelState(cust.zkChannelMgr.dbPath, zkChannelName)
	if err != nil {
		t.Errorf("%v", err)
	}
	assert.Equal(t, "ConfirmedClose", status)

}

func TestMerchClose(t *testing.T) {

	cust, merch := setupZkChannelManagers(t)
	defer tearDownZkChannelManagers(cust, merch)

	zkchannels.SetupLibzkChannels("myChannel", cust.zkChannelMgr.dbPath, merch.zkChannelMgr.dbPath)

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

	// Check that MerchChannelState has updated.
	status, err := zkchannels.GetMerchChannelState(merch.zkChannelMgr.dbPath, escrowTxid)
	if err != nil {
		t.Errorf("%v", err)
	}
	assert.Equal(t, "Open", status)

	merch.zkChannelMgr.MerchClose(merch.mockNotifier, escrowTxid)

	// Get and return the merch-close tx merch published to the network.
	var merchCloseTx *wire.MsgTx
	select {
	case merchCloseTx = <-merch.publTxChan:
		// Compare weight of publishedTx vs value used for calculating txFee
		expected := int64(merchCloseTxKW * 1000) // (upper bound)
		actual := blockchain.GetTransactionWeight(btcutil.NewTx(merchCloseTx))
		// allow for 2 byte variation in scriptSig signature length (2
		// signatures for multisig).
		if d := expected - actual; d > 2 || d < 0 {
			t.Errorf("merchCloseTx did not have the expected weight. Expected: %v, got: %v", expected, actual)
		}
	case <-time.After(time.Second * 5):
		t.Fatalf("merch did not publish merchClose tx")
	}

	merch.mockNotifier.oneConfChannel <- &chainntnfs.TxConfirmation{
		Tx: merchCloseTx,
	}

	// Give custState a moment to update
	time.Sleep(1 * time.Millisecond)
	// Check that MerchChannelState has updated.
	status, err = zkchannels.GetMerchChannelState(merch.zkChannelMgr.dbPath, escrowTxid)
	if err != nil {
		t.Errorf("%v", err)
	}
	assert.Equal(t, "PendingClose", status)
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

func TestZkInfo(t *testing.T) {

	cust, merch := setupZkChannelManagers(t)
	defer tearDownZkChannelManagers(cust, merch)

	zkchannels.SetupLibzkChannels("myChannel", cust.zkChannelMgr.dbPath, merch.zkChannelMgr.dbPath)

	// Test that ZkInfo returns pubkey (hex) of len 66
	merchPk, err := merch.zkChannelMgr.ZkInfo()
	if err != nil {
		t.Errorf("%v", err)
	}

	re := regexp.MustCompile(`^[0-9a-fA-F]+$`)
	slice := (re.FindAllString(merchPk, -1))
	assert.Equal(t, 66, len(slice[0]), "ZkInfo returned an invalid pubkey (not 33 byte hex string)")
}

func TestTotalReceived(t *testing.T) {

	cust, merch := setupZkChannelManagers(t)
	defer tearDownZkChannelManagers(cust, merch)

	zkchannels.SetupLibzkChannels("myChannel", cust.zkChannelMgr.dbPath, merch.zkChannelMgr.dbPath)

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
}

func addDummyChannelToken(t *testing.T, dbPath string) (libzkchannels.ChannelToken, error) {
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
	return channelToken, nil
}

func addDummyChannelStatus(t *testing.T, dbPath string, escrowTxid string) (newStatus string, err error) {

	newChanStatus := "Open"

	zkMerchDB, err := zkchanneldb.OpenMerchBucket(dbPath)
	if err != nil {
		t.Fatalf("%v", err)
	}

	merchState, err := zkchanneldb.GetMerchState(zkMerchDB)
	if err != nil {
		zkchLog.Error(err)
		return "", err
	}

	// Flip escrowTxid to Big Endian to match how it is stored in
	// merchState.ChannelStatusMap
	//This works because hex strings are of even size
	s := ""
	for i := 0; i < len(escrowTxid); i += 2 {
		s = escrowTxid[i:i+2] + s
	}
	escrowTxidBigEn := s

	(*merchState.ChannelStatusMap)[escrowTxidBigEn] = newChanStatus

	err = zkchanneldb.AddMerchState(zkMerchDB, merchState)
	if err != nil {
		t.Errorf("%v", err)
		return newChanStatus, err
	}

	err = zkMerchDB.Close()
	if err != nil {
		t.Fatalf("%v", err)
	}

	return newChanStatus, nil
}

func TestListZkChannels(t *testing.T) {

	cust, merch := setupZkChannelManagers(t)
	defer tearDownZkChannelManagers(cust, merch)

	zkchannels.SetupLibzkChannels("myChannel", cust.zkChannelMgr.dbPath, merch.zkChannelMgr.dbPath)

	// Test ListZkChannels
	ListOfZkChannels, err := merch.zkChannelMgr.ListZkChannels()
	if err != nil {
		t.Errorf("%v", err)
	}
	numChannels0 := len(ListOfZkChannels.ChannelID)
	assert.Equal(t, 0, numChannels0, "Non-empty list of zkchannels when Merchant is initialized")

	addedChannelToken, err := addDummyChannelToken(t, merch.zkChannelMgr.dbPath)
	addedStatus, err := addDummyChannelStatus(t, merch.zkChannelMgr.dbPath, addedChannelToken.EscrowTxId)

	ListOfZkChannels, err = merch.zkChannelMgr.ListZkChannels()
	if err != nil {
		t.Errorf("%v", err)
	}

	assert.Equal(t, addedChannelToken, ListOfZkChannels.ChannelToken[0], "ListZkChannels did not return the added channel token")
	assert.Equal(t, addedStatus, ListOfZkChannels.ChannelStatus[0], "ListZkChannels did not return the added channel status")
}

func TestZkChannelBalance(t *testing.T) {

	cust, merch := setupZkChannelManagers(t)
	defer tearDownZkChannelManagers(cust, merch)

	zkchannels.SetupLibzkChannels("myChannel", cust.zkChannelMgr.dbPath, merch.zkChannelMgr.dbPath)

	// Test ZkChannelBalance
	escrowTxid, localBalance, remoteBalance, status, err := cust.zkChannelMgr.ZkChannelBalance("myChannel")
	if err != nil {
		t.Errorf("%v", err)
	}
	t.Log("escrowTxid", escrowTxid)

	// Check that escrowTxid is a 32 byte hex string
	re := regexp.MustCompile(`^[0-9a-fA-F]+$`)
	slice := (re.FindAllString(escrowTxid, -1))
	assert.Equal(t, 64, len(slice[0]), "ZkInfo returned an invalid pubkey (not 33 byte hex string)")

	assert.Equal(t, int64(1000000), localBalance, "ZkChannelBalance returned incorrect local balance")
	assert.Equal(t, int64(5000), remoteBalance, "ZkChannelBalance returned incorrect remote balance")
	assert.Equal(t, "Open", status, "ZkChannelBalance returned incorrect channel status")
}
