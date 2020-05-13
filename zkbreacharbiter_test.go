// +build !rpctest

package lnd

import (
	"encoding/hex"
	"io/ioutil"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/lightningnetwork/lnd/chainntnfs"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/contractcourt"
	"github.com/lightningnetwork/lnd/htlcswitch"
	"github.com/lightningnetwork/lnd/libzkchannels"
	"github.com/lightningnetwork/lnd/lnwallet"
	"github.com/lightningnetwork/lnd/lnwallet/chainfee"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/zkchanneldb"
)

// zkchannel test txs
const (
	escrowTx   = "0200000000010132eb1608d217f821e3bfa16c4ab70dc3e56f1d65a7d84dd4a2aba9b244e50d520000000000ffffffff030627000000000000220020f9f4144b30c03dc6f56e29553755e6d1554c32bdad05f8f043283226e07200b40a00000000000000160014d05ac4b544f30ed4648d25eff1700f8e5e0a03b80000000000000000436a414d8e9a9f606a8210478c1fe86a79a98e43484c825df7b18dc351ada564bebd25027160fb5e48252f02a00066dfa823d15844ad93e04f9c9b746e1f28ed4a1eaddb0400483045022100e76b48106224b2c896e3095794dded09a38160b46cd86439fe83f399cc2ba9c1022062e4cb1e650c79e4ca78ac347cf08a3ea6377f4d1566169f960ec268a6c0099501473044022005bfbf4706cc3af47366b9a19200adbb6e58c8266a72fd3cd19b4a4e8eb4f9a20220254d2627588a0a66cd69267f01e54d14b941895f649b8d5dbf52273aa686545b0147522103742aaaffc1dd1e8016355012dbcbc7145291412078e209c3e8bf0fe323ccbf29210217d55a1e3ecdd220fde4bddbbfd485a1596c0c5cb7ef11dbfcdb2dd9cf4b85af52ae00000000"
	escrowTxid = "9a481f1729ff18783b18e4489411307ec69e67072deeba6c0cfe91e352738220"

	merchCloseTxid = "f7aaea919dfd714948253c26b5f0ce45bbc16cca6d01c916696ac97566ced04f"

	custCloseTxid = "9a481f1729ff18783b18e4489411307ec69e67072deeba6c0cfe91e352738220"
	// custCloseTx   = "0200000000010132eb1608d217f821e3bfa16c4ab70dc3e56f1d65a7d84dd4a2aba9b244e50d520000000000ffffffff030627000000000000220020f9f4144b30c03dc6f56e29553755e6d1554c32bdad05f8f043283226e07200b40a00000000000000160014d05ac4b544f30ed4648d25eff1700f8e5e0a03b80000000000000000436a414d8e9a9f606a8210478c1fe86a79a98e43484c825df7b18dc351ada564bebd25027160fb5e48252f02a00066dfa823d15844ad93e04f9c9b746e1f28ed4a1eaddb0400483045022100e76b48106224b2c896e3095794dded09a38160b46cd86439fe83f399cc2ba9c1022062e4cb1e650c79e4ca78ac347cf08a3ea6377f4d1566169f960ec268a6c0099501473044022005bfbf4706cc3af47366b9a19200adbb6e58c8266a72fd3cd19b4a4e8eb4f9a20220254d2627588a0a66cd69267f01e54d14b941895f649b8d5dbf52273aa686545b0147522103742aaaffc1dd1e8016355012dbcbc7145291412078e209c3e8bf0fe323ccbf29210217d55a1e3ecdd220fde4bddbbfd485a1596c0c5cb7ef11dbfcdb2dd9cf4b85af52ae00000000"

// custCloseMerchTxid = "cc7e73d1f20d69b9bfa858a79a7cfc03529123a12fba57700fad923d4ebe341f"
// custCloseMerchTx   = "020000000001014360aa5440c33ba6ddf6cc1799a49fc46b99b07130544b3134f8cf8a6169fb6b0000000000ffffffff030627000000000000220020eab1b21b1227f90a54c80bd0ac992ff57daca6b89510c68281be97a995bd2ac70a00000000000000160014234f7938f5c82ddbcd99c810a885eb336ce097670000000000000000436a416dad4b8de4ea7765e89a69def72048c6bd1b9f2fb143aec83d0b924c42adc074027160fb5e48252f02a00066dfa823d15844ad93e04f9c9b746e1f28ed4a1eaddb050047304402203a440bde54817e96a8611fb69571304e1503a31da20da8bee2033ba06bb5f4cf02200a2fdba6d89004cbb78f995d1a84251df90fbaf36f0022d08df9828a3cf46c72014730440220137c9d3f83f7e8aeb4235e098f446be72aebb06c6aaf92248c2f33e82f38a7760220020517fb88acfbb5d97131f653d4abfe284866d4f526573181cbd3315264831901010172635221025f6ed070ebe3a585e70f77058e3c4ba0c373c3bedebda19a077a860955b6026b210217d55a1e3ecdd220fde4bddbbfd485a1596c0c5cb7ef11dbfcdb2dd9cf4b85af52ae6702cf05b2752103efac29be9b04fc77d27298e36e3c0eded7ab1a98cbe1aee629827df37ef9b6cfac6800000000"
)

func initZkBreachedState(t *testing.T) (*zkBreachArbiter,
	*lnwallet.LightningChannel, *lnwallet.LightningChannel,
	*lnwallet.LocalForceCloseSummary, chan *ZkContractBreachEvent,
	func(), func()) {
	// Create a pair of channels using a notifier that allows us to signal
	// a spend of the funding transaction. Alice's channel will be the on
	// observing a breach.
	alice, bob, cleanUpChans, err := createInitChannels(1)
	if err != nil {
		t.Fatalf("unable to create test channels: %v", err)
	}

	// Instantiate a breach arbiter to handle the breach of alice's channel.
	custContractBreaches := make(chan *ZkContractBreachEvent)

	brar, cleanUpArb, err := createTestZkArbiter(
		t, custContractBreaches, alice.State().Db,
	)
	if err != nil {
		t.Fatalf("unable to initialize test breach arbiter: %v", err)
	}

	// Send one HTLC to Bob and perform a state transition to lock it in.
	htlcAmount := lnwire.NewMSatFromSatoshis(20000)
	htlc, _ := createHTLC(0, htlcAmount)
	if _, err := alice.AddHTLC(htlc, nil); err != nil {
		t.Fatalf("alice unable to add htlc: %v", err)
	}
	if _, err := bob.ReceiveHTLC(htlc); err != nil {
		t.Fatalf("bob unable to recv add htlc: %v", err)
	}
	if err := forceStateTransition(alice, bob); err != nil {
		t.Fatalf("Can't update the channel state: %v", err)
	}

	// Generate the force close summary at this point in time, this will
	// serve as the old state bob will broadcast.
	bobClose, err := bob.ForceClose()
	if err != nil {
		t.Fatalf("unable to force close bob's channel: %v", err)
	}

	// Now send another HTLC and perform a state transition, this ensures
	// Alice is ahead of the state Bob will broadcast.
	htlc2, _ := createHTLC(1, htlcAmount)
	if _, err := alice.AddHTLC(htlc2, nil); err != nil {
		t.Fatalf("alice unable to add htlc: %v", err)
	}
	if _, err := bob.ReceiveHTLC(htlc2); err != nil {
		t.Fatalf("bob unable to recv add htlc: %v", err)
	}
	if err := forceStateTransition(alice, bob); err != nil {
		t.Fatalf("Can't update the channel state: %v", err)
	}

	return brar, alice, bob, bobClose, custContractBreaches, cleanUpChans,
		cleanUpArb
}

var zkBreachTests = []breachTest{
	{
		name:          "all spends",
		spend2ndLevel: true,
		whenNonZeroInputs: func(t *testing.T,
			inputs map[wire.OutPoint]*wire.MsgTx,
			publTx chan *wire.MsgTx) {

			var tx *wire.MsgTx
			select {
			case tx = <-publTx:
			case <-time.After(5 * time.Second):
				t.Fatalf("tx was not published")
			}

			// The justice transaction should have thee same number
			// of inputs as we are tracking in the test.
			if len(tx.TxIn) != len(inputs) {
				t.Fatalf("expected justice txn to have %d "+
					"inputs, found %d", len(inputs),
					len(tx.TxIn))
			}

			// Ensure that each input exists on the justice
			// transaction.
			for in := range inputs {
				findInputIndex(t, in, tx)
			}

		},
		whenZeroInputs: func(t *testing.T,
			inputs map[wire.OutPoint]*wire.MsgTx,
			publTx chan *wire.MsgTx) {

			// Sanity check to ensure the brar doesn't try to
			// broadcast another sweep, since all outputs have been
			// spent externally.
			select {
			case <-publTx:
				t.Fatalf("tx published unexpectedly")
			case <-time.After(50 * time.Millisecond):
			}
		},
	},
}

// TestBreachSpends checks the behavior of the breach arbiter in response to
// spend events on a channels outputs by asserting that it properly removes or
// modifies the inputs from the justice txn.
func TestZkBreachSpends(t *testing.T) {
	for _, test := range zkBreachTests {
		tc := test
		t.Run(tc.name, func(t *testing.T) {
			testZkBreachSpends(t, tc)
		})
	}
}

func testZkBreachSpends(t *testing.T, test breachTest) {
	brar, alice, _, _, custContractBreaches,
		cleanUpChans, cleanUpArb := initZkBreachedState(t)
	defer cleanUpChans()
	defer cleanUpArb()

	// var (
	// 	height       = bobClose.ChanSnapshot.CommitHeight
	// 	forceCloseTx = bobClose.CloseTx
	// 	chanPoint    = alice.ChanPoint
	// 	publTx       = make(chan *wire.MsgTx)
	// 	publErr      error
	// 	publMtx      sync.Mutex
	// )

	// Use hardcoded zkchannel fundingOutpoint
	var escrowTxidHash chainhash.Hash
	err := chainhash.Decode(&escrowTxidHash, escrowTxid)
	if err != nil {
		t.Fatal(err)
	}

	fundingOut := &wire.OutPoint{
		Hash:  escrowTxidHash,
		Index: uint32(0),
	}

	var (
		// height = uint64(0)
		// forceCloseTx   = custCloseTx
		forceCloseTxid = custCloseTxid
		chanPoint      = fundingOut
		publTx         = make(chan *wire.MsgTx)
		publErr        error
		publMtx        sync.Mutex
	)

	// Make PublishTransaction always return ErrDoubleSpend to begin with.
	publErr = lnwallet.ErrDoubleSpend
	brar.cfg.PublishTransaction = func(tx *wire.MsgTx) error {
		publTx <- tx

		publMtx.Lock()
		defer publMtx.Unlock()
		return publErr
	}

	// // Notify the breach arbiter about the breach.
	// retribution, err := lnwallet.NewBreachRetribution(
	// 	alice.State(), height, 1,
	// )
	// if err != nil {
	// 	t.Fatalf("unable to create breach retribution: %v", err)
	// }

	// Hard coded example
	// TODO: Move to test list
	ZkCustBreachInfo := contractcourt.ZkBreachInfo{
		EscrowTxid:    chainhash.Hash{0xb5, 0x7f, 0x93, 0x7a, 0x5e, 0xb2, 0xd3, 0x3b, 0x27, 0x17, 0xc, 0x0, 0x9d, 0xc2, 0xbe, 0x3d, 0x4d, 0x89, 0xe3, 0x7f, 0xdf, 0x47, 0xa7, 0xd, 0x75, 0x32, 0x1d, 0xde, 0xf5, 0xe5, 0xe, 0x57},
		CloseTxid:     chainhash.Hash{0x13, 0x4b, 0x35, 0x6, 0x1c, 0x7e, 0xfa, 0xf0, 0x3d, 0x85, 0xe2, 0xed, 0x69, 0xe7, 0xed, 0xd7, 0xb6, 0x10, 0xa, 0x18, 0xd1, 0xff, 0x46, 0xfd, 0x25, 0x58, 0x38, 0x3a, 0xcb, 0xd0, 0x52, 0x2e},
		ClosePkScript: []uint8{0x0, 0x20, 0xea, 0xb1, 0xb2, 0x1b, 0x12, 0x27, 0xf9, 0xa, 0x54, 0xc8, 0xb, 0xd0, 0xac, 0x99, 0x2f, 0xf5, 0x7d, 0xac, 0xa6, 0xb8, 0x95, 0x10, 0xc6, 0x82, 0x81, 0xbe, 0x97, 0xa9, 0x95, 0xbd, 0x2a, 0xc7},
		RevLock:       "6dad4b8de4ea7765e89a69def72048c6bd1b9f2fb143aec83d0b924c42adc074",
		RevSecret:     "6dad4b8de4ea7765e89a69def72048c6bd1b9f2fb143aec83d0b924c42adc074",
		CustClosePk:   "027160fb5e48252f02a00066dfa823d15844ad93e04f9c9b746e1f28ed4a1eaddb",
		Amount:        9990,
	}

	disputeKeyPriv, disputeKeyPub := btcec.PrivKeyFromBytes(btcec.S256(),
		alicesPrivKey)

	disputePk := hex.EncodeToString(disputeKeyPub.SerializeCompressed())
	disputeSk := hex.EncodeToString(disputeKeyPriv.Serialize())

	s := ""
	//var m map[string]interface{}
	var mm = map[string]interface{}{}

	merchState := libzkchannels.MerchState{
		Id:          &s,
		PkM:         &disputePk,
		SkM:         &disputeSk,
		HmacKey:     "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
		HmacKeyR:    "00000000000000000000000000000000",
		PayoutSk:    &disputeSk,
		PayoutPk:    &disputePk,
		DisputeSk:   &disputeSk,
		DisputePk:   &disputePk,
		ActivateMap: &mm,
		CloseTxMap:  &mm,
		NetConfig:   nil,
		DbUrl:       s,
	}

	channelState := libzkchannels.ChannelState{
		DustLimit:  330,
		KeyCom:     disputeSk,
		Name:       "name",
		ThirdParty: false,
		SelfDelay:  uint16(1487),
	}

	testDB, _ := zkchanneldb.SetupDB(brar.cfg.DBPath)

	_ = zkchanneldb.AddMerchState(testDB, merchState)
	_ = zkchanneldb.AddMerchField(testDB, channelState, "channelStateKey")

	testDB.Close()

	breach := &ZkContractBreachEvent{
		ChanPoint:    *chanPoint,
		ProcessACK:   make(chan error, 1),
		ZkBreachInfo: ZkCustBreachInfo,
	}

	custContractBreaches <- breach

	// We'll also wait to consume the ACK back from the breach arbiter.
	select {
	case err := <-breach.ProcessACK:
		if err != nil {
			t.Fatalf("handoff failed: %v", err)
		}
	case <-time.After(time.Second * 15):
		t.Fatalf("breach arbiter didn't send ack back")
	}

	state := alice.State()
	err = state.CloseChannel(&channeldb.ChannelCloseSummary{
		ChanPoint:               state.FundingOutpoint,
		ChainHash:               state.ChainHash,
		RemotePub:               state.IdentityPub,
		CloseType:               channeldb.BreachClose,
		Capacity:                state.Capacity,
		IsPending:               true,
		ShortChanID:             state.ShortChanID(),
		RemoteCurrentRevocation: state.RemoteCurrentRevocation,
		RemoteNextRevocation:    state.RemoteNextRevocation,
		LocalChanConfig:         state.LocalChanCfg,
	})
	if err != nil {
		t.Fatalf("unable to close channel: %v", err)
	}

	// // After exiting, the breach arbiter should have persisted the
	// // retribution information and the channel should be shown as pending
	// // force closed.
	// assertArbiterBreach(t, brar, chanPoint)

	// // Assert that the database sees the channel as pending close, otherwise
	// // the breach arbiter won't be able to fully close it.
	// assertPendingClosed(t, alice)

	// Notify that the breaching transaction is confirmed, to trigger the
	// retribution logic.

	notifier := brar.cfg.Notifier.(*mockSpendNotifier)
	notifier.confChannel <- &chainntnfs.TxConfirmation{}

	// The breach arbiter should attempt to sweep all outputs on the
	// breached commitment. We'll pretend that the HTLC output has been
	// spent by the channel counter party's second level tx already.
	var tx *wire.MsgTx
	select {
	case tx = <-publTx:
	case <-time.After(5 * time.Second):
		t.Fatalf("tx was not published")
	}

	// All outputs should initially spend from the force closed txn.
	forceTxID := forceCloseTxid
	for _, txIn := range tx.TxIn {
		if txIn.PreviousOutPoint.Hash.String() != forceTxID {
			t.Fatalf("og justice tx not spending commitment %v", forceTxID)
		}
	}

	// Deliver confirmation of sweep if the test expects it.
	if test.sendFinalConf {
		notifier.confChannel <- &chainntnfs.TxConfirmation{}
	}

	// // Assert that the channel is fully resolved.
	// assertBrarCleanup(t, brar, alice.ChanPoint, alice.State().Db)
}

// createTestZkArbiter instantiates a breach arbiter with a failing retribution
// store, so that controlled failures can be tested.
func createTestZkArbiter(t *testing.T, custContractBreaches chan *ZkContractBreachEvent,
	db *channeldb.DB) (*zkBreachArbiter, func(), error) {

	// // Create a failing retribution store, that wraps a normal one.
	// store := newFailingRetributionStore(func() RetributionStore {
	// 	return newRetributionStore(db)
	// })

	aliceKeyPriv, _ := btcec.PrivKeyFromBytes(btcec.S256(),
		alicesPrivKey)
	signer := &mockSigner{key: aliceKeyPriv}

	// Assemble our test arbiter.
	notifier := makeMockSpendNotifier()

	testDir, err := ioutil.TempDir("", "zkchanneldb")
	if err != nil {
		t.Fatalf("unable to create temp directory: %v", err)
	}

	ba := newZkBreachArbiter(&ZkBreachConfig{
		CloseLink:            func(_ *wire.OutPoint, _ htlcswitch.ChannelCloseType) {},
		DBPath:               path.Join(testDir, "zkmerch.db"),
		DB:                   db,
		Estimator:            chainfee.NewStaticEstimator(12500, 0),
		GenSweepScript:       func() ([]byte, error) { return nil, nil },
		CustContractBreaches: custContractBreaches,
		Signer:               signer,
		Notifier:             notifier,
		PublishTransaction:   func(_ *wire.MsgTx) error { return nil },
		// Store:              store,
	})

	if err := ba.Start(); err != nil {
		return nil, nil, err
	}

	// The caller is responsible for closing the database.
	cleanUp := func() {
		ba.Stop()
	}

	return ba, cleanUp, nil
}
