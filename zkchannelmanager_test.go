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
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/contractcourt"
	"github.com/lightningnetwork/lnd/lnpeer"
	"github.com/lightningnetwork/lnd/lnwire"
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
