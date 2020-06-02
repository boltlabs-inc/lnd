package lnd

import (
	"errors"
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
	"github.com/lightningnetwork/lnd/lnpeer"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/stretchr/testify/assert"
)

type zkTestNode struct {
	msgChan         chan lnwire.Message
	publTxChan      chan *wire.MsgTx
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

	chainNotifier := &mockNotifier{
		oneConfChannel: make(chan *chainntnfs.TxConfirmation, 1),
		sixConfChannel: make(chan *chainntnfs.TxConfirmation, 1),
		epochChan:      make(chan *chainntnfs.BlockEpoch, 2),
	}
	manager := newZkChannelManager(isMerchant, zkChainWatcher, testDir, publishTransaction)
	return &zkTestNode{
		zkChannelMgr: manager,
		msgChan:      sentMessages,
		publTxChan:   publTxChan,
		mockNotifier: chainNotifier,
		testDir:      testDir,
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
	// TODO: Make sure tests return useful error messages from libzkchannels
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
	minFee := int64(0)
	maxFee := int64(10000)

	go cust.zkChannelMgr.initZkEstablish(inputSats, custUtxoTxid_LE, index, custInputSk, custStateSk, custPayoutSk, changePk, merchPk, zkChannelName, custBal, merchBal, feeCC, feeMC, minFee, maxFee, merch)

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

	var msg11 lnwire.Message
	select {
	case msg11 = <-merch.msgChan:
		errorMsg, _ := msg11.(*lnwire.Error)
		t.Fatalf("Received an unexpected error message: " + errorMsg.Error())
	case <-time.After(time.Second):
	}
}
