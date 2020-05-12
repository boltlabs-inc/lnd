package lnd

import (
	"errors"
	"fmt"
	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/wire"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/contractcourt"
	"github.com/lightningnetwork/lnd/lnpeer"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/stretchr/testify/assert"
	"net"
	"os"
	"testing"
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

func (z zkTestNode) WipeChannel(_ *wire.OutPoint) error {
	return nil
}

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

func createTestZkChannelManager() (*zkTestNode, error) {
	sentMessages := make(chan lnwire.Message)

	zkChainWatcher := func(z contractcourt.ZkChainWatcherConfig) error {
		return nil
	}
	manager, err := newZkChannelManager(true, zkChainWatcher)
	return &zkTestNode{
		zkChannelMgr: manager,
		msgChan:      sentMessages,
	}, err
}

func setupZkChannelManagers(t *testing.T) (*zkTestNode, *zkTestNode) {

	alice, err := createTestZkChannelManager()
	if err != nil {
		t.Fatalf("failed creating fundingManager: %v", err)
	}

	bob, err := createTestZkChannelManager()
	if err != nil {
		t.Fatalf("failed creating fundingManager: %v", err)
	}

	// With the funding manager's created, we'll now attempt to mimic a
	// connection pipe between them. In order to intercept the messages
	// within it, we'll redirect all messages back to the msgChan of the
	// sender. Since the fundingManager now has a reference to peers itself,
	// alice.sendMessage will be triggered when Bob's funding manager
	// attempts to send a message to Alice and vice versa.
	alice.remotePeer = bob
	alice.sendMessage = func(msg lnwire.Message) error {
		select {
		case alice.remotePeer.msgChan <- msg:
		case <-alice.shutdownChannel:
			return errors.New("shutting down")
		}
		return nil
	}

	bob.remotePeer = alice
	bob.sendMessage = func(msg lnwire.Message) error {
		select {
		case bob.remotePeer.msgChan <- msg:
		case <-bob.shutdownChannel:
			return errors.New("shutting down")
		}
		return nil
	}

	return alice, bob
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

func TestZkChannelManagerTests(t *testing.T) {
	t.Skip()
	cust, merch := setupZkChannelManagers(t)
	inputSats := int64(50 * 100000000)
	custUtxoTxid_LE := "21779e66bdf89e943ae5b16ae63240a41c5e6ab937dde7b5811c64f13729bb03"
	custInputSk := fmt.Sprintf("\"%v\"", "5511111111111111111111111111111100000000000000000000000000000000")

	err := cust.zkChannelMgr.initZkEstablish(inputSats, custUtxoTxid_LE, 0, custInputSk, custInputSk, custInputSk, "", "", "", 10000, 0, 1000, 1000, 0, 10000, merch)
	assert.Nil(t, err)

	tearDownZkChannelManagers(cust, merch)
}
