package lnd

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net"
	"os"
	"path"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/wire"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/contractcourt"
	"github.com/lightningnetwork/lnd/libzkchannels"
	"github.com/lightningnetwork/lnd/lnpeer"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/zkchanneldb"
	"github.com/stretchr/testify/assert"
)

var (
	// Use hard-coded keys for Alice and Bob, the two FundingManagers that
	// we will test the interaction between.
	custPrivKeyBytes = [32]byte{
		0xb7, 0x94, 0x38, 0x5f, 0x2d, 0x1e, 0xf7, 0xab,
		0x4d, 0x92, 0x73, 0xd1, 0x90, 0x63, 0x81, 0xb4,
		0x4f, 0x2f, 0x6f, 0x25, 0x88, 0xa3, 0xef, 0xb9,
		0x6a, 0x49, 0x18, 0x83, 0x31, 0x98, 0x47, 0x53,
	}

	custPrivKey, custPubKey = btcec.PrivKeyFromBytes(btcec.S256(),
		alicePrivKeyBytes[:])

	custTCPAddr, _ = net.ResolveTCPAddr("tcp", "10.0.0.2:9001")

	custAddr = &lnwire.NetAddress{
		IdentityKey: alicePubKey,
		Address:     aliceTCPAddr,
	}

	merchPrivKeyBytes = [32]byte{
		0x81, 0xb6, 0x37, 0xd8, 0xfc, 0xd2, 0xc6, 0xda,
		0x63, 0x59, 0xe6, 0x96, 0x31, 0x13, 0xa1, 0x17,
		0xd, 0xe7, 0x95, 0xe4, 0xb7, 0x25, 0xb8, 0x4d,
		0x1e, 0xb, 0x4c, 0xfd, 0x9e, 0xc5, 0x8c, 0xe9,
	}

	merchPrivKey, merchPubKey = btcec.PrivKeyFromBytes(btcec.S256(),
		bobPrivKeyBytes[:])

	merchTCPAddr, _ = net.ResolveTCPAddr("tcp", "10.0.0.2:9000")

	merchAddr = &lnwire.NetAddress{
		IdentityKey: bobPubKey,
		Address:     bobTCPAddr,
	}
)

type zkTestNode struct {
	privKey         *btcec.PrivateKey
	addr            *lnwire.NetAddress
	msgChan         chan lnwire.Message
	announceChan    chan lnwire.Message
	publTxChan      chan *wire.MsgTx
	zkChannelMgr    *zkChannelManager
	newChannels     chan *newChannelMsg
	mockNotifier    *mockNotifier
	mockChanEvent   *mockChanEvent
	testDir         string
	shutdownChannel chan struct{}
	remoteFeatures  []lnwire.FeatureBit

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
	errChan := make(chan error)
	msg := &newChannelMsg{
		channel: channel,
		err:     errChan,
	}

	select {
	case z.newChannels <- msg:
	case <-quit:
		return ErrFundingManagerShuttingDown
	}

	select {
	case err := <-errChan:
		return err
	case <-quit:
		return ErrFundingManagerShuttingDown
	}
}

func (z *zkTestNode) WipeChannel(_ *wire.OutPoint) {}

func (z zkTestNode) PubKey() [33]byte {
	return newSerializedKey(z.addr.IdentityKey)
}

func (z zkTestNode) IdentityKey() *btcec.PublicKey {
	return z.addr.IdentityKey
}

func (z zkTestNode) Address() net.Addr {
	return z.addr.Address
}

func (z zkTestNode) QuitSignal() <-chan struct{} {
	return z.shutdownChannel
}

func (z zkTestNode) LocalFeatures() *lnwire.FeatureVector {
	return lnwire.NewFeatureVector(nil, nil)
}

func (z zkTestNode) RemoteFeatures() *lnwire.FeatureVector {
	return lnwire.NewFeatureVector(
		lnwire.NewRawFeatureVector(z.remoteFeatures...), nil,
	)
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
	manager := newZkChannelManager(isMerchant, zkChainWatcher, testDir)
	return &zkTestNode{
		zkChannelMgr: manager,
		msgChan:      sentMessages,
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
	err = merch.zkChannelMgr.initMerchant("Merchant", "4e98ffa9b14dfb89826c53191445f17ab40aa1187228ae22bf7546f0a0b88951", "5c9bd788107445519053a6bf212c32ffe4fcae585fbdfdfaf048df96187609ef", "c3d60931fee23ab136014a40fc329f4f90f40f20a0455b8a1cdfad81d9018b97")
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

func initTestCustState(DBPath string, zkChannelName string) error {

	merchPk := "038c2add1dc8cf2c57bac6e19d1f963e0c42103554e8b35e425bc2a78f4c22b273"
	custBal := int64(1000000)
	merchBal := int64(1000000)
	feeCC := int64(1000)
	minFee := int64(0)
	maxFee := int64(10000)
	feeMC := int64(1000)
	custStateSk := "1a1971e1379beec67178509e25b6772c66cb67bb04d70df2b4bcdb8c08a01827"
	custPayoutSk := "4157697b6428532758a9d0f9a73ce58befe3fd665797427d1c5bb3d33f6a132e"
	channelToken, custState, err := libzkchannels.InitCustomer(merchPk, custBal, merchBal, feeCC, minFee, maxFee, feeMC, "cust")
	channelToken, custState, err = libzkchannels.LoadCustomerWallet(custState, channelToken, custStateSk, custPayoutSk)

	escrowPrevout := "0299620f12ebfe5e940c87c8bf1c914d70f72e0f678f9a623f98e6f86b4fb410"
	signedEscrowTx := "020000000001018d2744b606be69fd45d3661e1a61b81ec66d1417941ae33c4ac7079f2bd4aee80000000017160014d83e1345c76dc160630937746d2d2693562e9c58ffffffff0280841e0000000000220020b718e637a405ba914aede6bcb3d14dec83deca9b0449a20c7163b94456eb01c3806de7290100000016001461492b43be394b9e6eeb077f17e73665bbfd455b02483045022100cb3289b503a08250e7562f1ed4072065ef951d6bee0c02ea555a542c9650d30f02205eb0a7e8a471074a6f8a632033e484fa4a1e20dd2bdf9a73b160ad7f761fe9ea0121032581c94e62b16c1f5fc36c5ff6ddc5c3e7cc4e7e70e2ec3ab3f663cff9d9b7d000000000"

	zkCustDB, err := zkchanneldb.OpenZkChannelBucket(zkChannelName, DBPath)

	err = zkchanneldb.AddCustState(zkCustDB, zkChannelName, custState)
	if err != nil {
		zkchLog.Error(err)
		return err
	}

	err = zkchanneldb.AddField(zkCustDB, zkChannelName, channelToken, "channelTokenKey")
	if err != nil {
		zkchLog.Error(err)
		return err
	}

	err = zkchanneldb.AddField(zkCustDB, zkChannelName, merchPk, "merchPkKey")
	if err != nil {
		zkchLog.Error(err)
		return err
	}

	err = zkchanneldb.AddField(zkCustDB, zkChannelName, custState.CustBalance, "custBalKey")
	if err != nil {
		zkchLog.Error(err)
		return err
	}

	err = zkchanneldb.AddField(zkCustDB, zkChannelName, custState.MerchBalance, "merchBalKey")
	if err != nil {
		zkchLog.Error(err)
		return err
	}

	err = zkchanneldb.AddField(zkCustDB, zkChannelName, feeCC, "feeCCKey")
	if err != nil {
		zkchLog.Error(err)
		return err
	}

	err = zkchanneldb.AddField(zkCustDB, zkChannelName, feeMC, "feeMCKey")
	if err != nil {
		zkchLog.Error(err)
		return err
	}

	err = zkchanneldb.AddField(zkCustDB, zkChannelName, minFee, "minFeeKey")
	if err != nil {
		zkchLog.Error(err)
		return err
	}

	err = zkchanneldb.AddField(zkCustDB, zkChannelName, maxFee, "maxFeeKey")
	if err != nil {
		zkchLog.Error(err)
		return err
	}

	err = zkchanneldb.AddField(zkCustDB, zkChannelName, escrowTxid, "escrowTxidKey")
	if err != nil {
		zkchLog.Error(err)
		return err
	}

	err = zkchanneldb.AddField(zkCustDB, zkChannelName, escrowPrevout, "escrowPrevoutKey")
	if err != nil {
		zkchLog.Error(err)
		return err
	}

	err = zkchanneldb.AddField(zkCustDB, zkChannelName, signedEscrowTx, "signedEscrowTxKey")
	if err != nil {
		zkchLog.Error(err)
		return err
	}

	err = zkCustDB.Close()
	if err != nil {
		zkchLog.Error(err)
	}

	return err
}

func TestInitZkEstablish(t *testing.T) {
	cust, merch := setupZkChannelManagers(t)
	defer tearDownZkChannelManagers(cust, merch)
	inputSats := int64(50 * 100000000)
	custUtxoTxid_LE := "21779e66bdf89e943ae5b16ae63240a41c5e6ab937dde7b5811c64f13729bb03"

	go cust.zkChannelMgr.initZkEstablish(inputSats, custUtxoTxid_LE, 0, "5511111111111111111111111111111100000000000000000000000000000000", "5511111111111111111111111111111100000000000000000000000000000000", "5511111111111111111111111111111100000000000000000000000000000000", "03fb5e01cef3d314bcf87ca2aee05b7f5d308b5d61ea04e99f595c3761375ebbbe", "03fe612a21357ab98377f442ab80383bae3fea772e31745bd33e3ae1339f38ff86", "channel", 100000, 0, 1000, 1000, 0, 10000, merch)
	var msg lnwire.Message
	select {
	case msg = <-cust.msgChan:
	case <-time.After(time.Second * 5):
		t.Fatalf("peer did not send message")
	}
	msgType := lnwire.ZkEstablishOpen{}
	assert.Equal(t, msgType.MsgType(), msg.MsgType())
}

func TestProcessZkEstablishOpen(t *testing.T) {
	cust, merch := setupZkChannelManagers(t)
	defer tearDownZkChannelManagers(cust, merch)

	zkEstablishOpen := lnwire.ZkEstablishOpen{
		EscrowTxid:    nil,
		CustPk:        nil,
		EscrowPrevout: nil,
		RevLock:       nil,
		CustBal:       nil,
		MerchBal:      nil,
	}

	go merch.zkChannelMgr.processZkEstablishOpen(&zkEstablishOpen, cust)

	var msg lnwire.Message
	select {
	case msg = <-merch.msgChan:
	case <-time.After(time.Second * 5):
		t.Fatalf("peer did not send message")
	}
	msgType := lnwire.ZkEstablishAccept{}
	assert.Equal(t, msgType.MsgType(), msg.MsgType())
}

func TestProcessZkEstablishAccept(t *testing.T) {

	cust, merch := setupZkChannelManagers(t)
	defer tearDownZkChannelManagers(cust, merch)
	zkChannelName := "testChannel"

	channelState := libzkchannels.ChannelState{
		MinThreshold:   546,
		KeyCom:         "49ef201d874fa22441086eb6b2bf0f87fe1920fdf2d4a869e2e768e6b94c24d5",
		Name:           "channel",
		ThirdParty:     false,
		MerchPayOutPk:  nil,
		MerchDisputePk: nil,
		SelfDelay:      1487,
	}
	toSelfDelay, err := libzkchannels.GetSelfDelayBE(channelState)

	custDBPath := path.Join(cust.testDir, "zkcust.db")
	err = initTestCustState(custDBPath, zkChannelName)
	if err != nil {
		t.Fatal(err)
	}

	// Create ZkEstablishAccept message that merch sends to cust
	merchClosePk := "03fe612a21357ab98377f442ab80383bae3fea772e31745bd33e3ae1339f38ff86"
	merchClosePkBytes := []byte(merchClosePk)
	toSelfDelayBytes := []byte(toSelfDelay)
	channelStateBytes, err := json.Marshal(channelState)
	if err != nil {
		cust.zkChannelMgr.failEstablishFlow(merch, err)
		return
	}

	zkEstablishAccept := lnwire.ZkEstablishAccept{
		ToSelfDelay:   toSelfDelayBytes,
		MerchPayoutPk: merchClosePkBytes,
		ChannelState:  channelStateBytes,
	}

	go cust.zkChannelMgr.processZkEstablishAccept(&zkEstablishAccept, merch, zkChannelName)

	var msg lnwire.Message
	select {
	case msg = <-cust.msgChan:
	case <-time.After(time.Second * 5):
		t.Fatalf("peer did not send message")
	}
	msgType := lnwire.ZkEstablishMCloseSigned{}
	assert.Equal(t, msgType.MsgType(), msg.MsgType())
}
