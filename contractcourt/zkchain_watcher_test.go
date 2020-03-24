package contractcourt

import (
	"bytes"
	"encoding/hex"
	"testing"
	"time"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/lightningnetwork/lnd/chainntnfs"
	"github.com/lightningnetwork/lnd/lnwallet"
)

// type mockNotifier struct {
// 	spendChan chan *chainntnfs.SpendDetail
// 	epochChan chan *chainntnfs.BlockEpoch
// 	confChan  chan *chainntnfs.TxConfirmation
// }

// func (m *mockNotifier) RegisterConfirmationsNtfn(txid *chainhash.Hash, _ []byte, numConfs,
// 	heightHint uint32) (*chainntnfs.ConfirmationEvent, error) {
// 	return &chainntnfs.ConfirmationEvent{
// 		Confirmed: m.confChan,
// 		Cancel:    func() {},
// 	}, nil
// }

// func (m *mockNotifier) RegisterBlockEpochNtfn(
// 	bestBlock *chainntnfs.BlockEpoch) (*chainntnfs.BlockEpochEvent, error) {

// 	return &chainntnfs.BlockEpochEvent{
// 		Epochs: m.epochChan,
// 		Cancel: func() {},
// 	}, nil
// }

// func (m *mockNotifier) Start() error {
// 	return nil
// }

// func (m *mockNotifier) Stop() error {
// 	return nil
// }
// func (m *mockNotifier) RegisterSpendNtfn(outpoint *wire.OutPoint, _ []byte,
// 	heightHint uint32) (*chainntnfs.SpendEvent, error) {

// 	return &chainntnfs.SpendEvent{
// 		Spend:  m.spendChan,
// 		Cancel: func() {},
// 	}, nil
// }

// ////////////////////////// zkchannel test txs
const (
	escrowTx   = "02000000000101e162d4625d3a6bc72f2c938b1e29068a00f42796aacc323896c235971416dff40000000017160014d83e1345c76dc160630937746d2d2693562e9c58ffffffff0210270000000000002200202d518b7ba4094d76e65af92750833f0a66307135564f1ad4afe5ce7efc46653df0ca052a01000000160014578dd1183845e18d42f90b1a9f3a464675ad2440024830450221009bb5949bb2dd0458cdcaed9d9c7cf3dd5aeb27f5ce4735418b2c365c1254c75b0220796704cb14275a19a574c0f7c0c7ce5066a6bd0aadced1db4769c57a2651b0c80121032581c94e62b16c1f5fc36c5ff6ddc5c3e7cc4e7e70e2ec3ab3f663cff9d9b7d000000000"
	escrowTxid = "570ee5f5de1d32750da747df7fe3894d3dbec29d000c17273bd3b25e7a937fb5"

	merchCloseTx   = "02000000000101570ee5f5de1d32750da747df7fe3894d3dbec29d000c17273bd3b25e7a937fb50000000000ffffffff0110270000000000002200208484e1b620d7c463c0e255166ab7b2e7f01b534bc56e62f2fdb07da816e1fc880400473044022029d628f32c1b95073f3494afe4080749d129a0404d887962143e0410a2df0b8d0220739a8abbe343663637313447242242af00768299b8b99d58cfe31343ba01a93901483045022100fcb5b0e5729cec81e768d7a90e0ff4623be0c611b880beb9b21710cda34b30560220461b9841cef598766034c860dbcc3f6eb6cfa03245087426674cfe465397b4db01475221025f6ed070ebe3a585e70f77058e3c4ba0c373c3bedebda19a077a860955b6026b210217d55a1e3ecdd220fde4bddbbfd485a1596c0c5cb7ef11dbfcdb2dd9cf4b85af52ae00000000"
	merchCloseTxid = "4360aa5440c33ba6ddf6cc1799a49fc46b99b07130544b3134f8cf8a6169fb6b"

	custCloseTxid = "2e52d0cb3a385825fd46ffd1180a10b6d7ede769ede2853df0fa7e1c06354b13"
	custCloseTx   = "02000000000101570ee5f5de1d32750da747df7fe3894d3dbec29d000c17273bd3b25e7a937fb50000000000ffffffff030627000000000000220020eab1b21b1227f90a54c80bd0ac992ff57daca6b89510c68281be97a995bd2ac70a00000000000000160014234f7938f5c82ddbcd99c810a885eb336ce097670000000000000000436a416dad4b8de4ea7765e89a69def72048c6bd1b9f2fb143aec83d0b924c42adc074027160fb5e48252f02a00066dfa823d15844ad93e04f9c9b746e1f28ed4a1eaddb0400483045022100bc07fce05baa349089434480cb64fa4d760b7ee5eb0c6b37efdb79d74cc19fae02206909b331b1d05c5db79e7aa5aaec2d370f84858dabd2b5e26aea24a22b072a970147304402207e6a8e6f64547b6ccc0a33701ae038fd6c2e7e563dbed5cd4dc1a541debdb606022028ca7ec8b256dae4ac1de0d960c8e49b30683eb3f4750c89c670029e05ab8d7001475221025f6ed070ebe3a585e70f77058e3c4ba0c373c3bedebda19a077a860955b6026b210217d55a1e3ecdd220fde4bddbbfd485a1596c0c5cb7ef11dbfcdb2dd9cf4b85af52ae00000000"

// custCloseMerchTxid = "cc7e73d1f20d69b9bfa858a79a7cfc03529123a12fba57700fad923d4ebe341f"
// custCloseMerchTx = "020000000001014360aa5440c33ba6ddf6cc1799a49fc46b99b07130544b3134f8cf8a6169fb6b0000000000ffffffff030627000000000000220020eab1b21b1227f90a54c80bd0ac992ff57daca6b89510c68281be97a995bd2ac70a00000000000000160014234f7938f5c82ddbcd99c810a885eb336ce097670000000000000000436a416dad4b8de4ea7765e89a69def72048c6bd1b9f2fb143aec83d0b924c42adc074027160fb5e48252f02a00066dfa823d15844ad93e04f9c9b746e1f28ed4a1eaddb050047304402203a440bde54817e96a8611fb69571304e1503a31da20da8bee2033ba06bb5f4cf02200a2fdba6d89004cbb78f995d1a84251df90fbaf36f0022d08df9828a3cf46c72014730440220137c9d3f83f7e8aeb4235e098f446be72aebb06c6aaf92248c2f33e82f38a7760220020517fb88acfbb5d97131f653d4abfe284866d4f526573181cbd3315264831901010172635221025f6ed070ebe3a585e70f77058e3c4ba0c373c3bedebda19a077a860955b6026b210217d55a1e3ecdd220fde4bddbbfd485a1596c0c5cb7ef11dbfcdb2dd9cf4b85af52ae6702cf05b2752103efac29be9b04fc77d27298e36e3c0eded7ab1a98cbe1aee629827df37ef9b6cfac6800000000"
)

// //////////////////////////////////////////////

// TestChainWatcherRemoteUnilateralClose tests that the chain watcher is able
// to properly detect a normal unilateral close by the remote node using their
// lowest commitment.
func TestZkChainWatcherRemoteUnilateralClose(t *testing.T) {
	t.Parallel()

	// Populate fields of zkFundingInfo.
	var escrowTxidHash chainhash.Hash
	chainhash.Decode(&escrowTxidHash, escrowTxid)

	fundingOut := &wire.OutPoint{
		Hash:  escrowTxidHash,
		Index: uint32(0),
	}

	serializedTx, err := hex.DecodeString(escrowTx)
	if err != nil {
		t.Error(err)
	}

	var msgTx wire.MsgTx
	err = msgTx.Deserialize(bytes.NewReader(serializedTx))
	if err != nil {
		t.Error(err)
	}

	// in zkChannels, the funding outpoint is always the first index of the output.
	pkScript := msgTx.TxOut[0].PkScript

	// t.Logf("pkScript: %#v", pkScript)

	zkFundingInfo := zkFundingInfo{
		fundingOut:      *fundingOut,
		pkScript:        pkScript,
		broadcastHeight: uint32(0), // TODO: Replace with actual fundingtx confirm height
	}

	// With the channels created, we'll now create a chain watcher instance
	// which will be watching for any closes of Alice's channel.
	custNotifier := &mockNotifier{
		spendChan: make(chan *chainntnfs.SpendDetail),
	}

	custChainWatcher, err := newZkChainWatcher(zkChainWatcherConfig{
		zkFundingInfo:       zkFundingInfo,
		notifier:            custNotifier,
		extractStateNumHint: lnwallet.GetStateNumHint,
	})
	if err != nil {
		t.Fatalf("unable to create chain watcher: %v", err)
	}
	err = custChainWatcher.Start()
	if err != nil {
		t.Fatalf("unable to start chain watcher: %v", err)
	}
	defer custChainWatcher.Stop()

	// We'll request a new channel event subscription from Alice's chain
	// watcher.
	chanEvents := custChainWatcher.SubscribeChannelEvents()

	var merchCloseTxidHash chainhash.Hash
	chainhash.Decode(&escrowTxidHash, custCloseTxid)

	serializedMerchCTx, err := hex.DecodeString(custCloseTx)
	if err != nil {
		t.Error(err)
	}

	// var merchCloseTxidHash chainhash.Hash
	// chainhash.Decode(&escrowTxidHash, merchCloseTxid)

	// serializedMerchCTx, err := hex.DecodeString(merchCloseTx)
	// if err != nil {
	// 	t.Error(err)
	// }

	var msgMerchCTx wire.MsgTx
	err = msgMerchCTx.Deserialize(bytes.NewReader(serializedMerchCTx))
	if err != nil {
		t.Error(err)
	}

	merchSpend := &chainntnfs.SpendDetail{
		SpenderTxHash: &merchCloseTxidHash,
		SpendingTx:    &msgMerchCTx,
	}
	custNotifier.spendChan <- merchSpend

	// We should get a new spend event over the remote unilateral close
	// event channel.
	var uniClose *ZkRemoteUnilateralCloseInfo
	select {
	case uniClose = <-chanEvents.ZkRemoteUnilateralClosure:
		t.Log(uniClose)
	case <-time.After(time.Second * 5):
		t.Fatalf("didn't receive unilateral close event")
	}

	// The unilateral close should have properly located Alice's output in
	// the commitment transaction.
	if uniClose == nil {
		t.Fatalf("unable to find alice's commit resolution")
	}
}

// func addFakeHTLC(t *testing.T, htlcAmount lnwire.MilliSatoshi, id uint64,
// 	aliceChannel, bobChannel *lnwallet.LightningChannel) {

// 	preimage := bytes.Repeat([]byte{byte(id)}, 32)
// 	paymentHash := sha256.Sum256(preimage)
// 	var returnPreimage [32]byte
// 	copy(returnPreimage[:], preimage)
// 	htlc := &lnwire.UpdateAddHTLC{
// 		ID:          uint64(id),
// 		PaymentHash: paymentHash,
// 		Amount:      htlcAmount,
// 		Expiry:      uint32(5),
// 	}

// 	if _, err := aliceChannel.AddHTLC(htlc, nil); err != nil {
// 		t.Fatalf("alice unable to add htlc: %v", err)
// 	}
// 	if _, err := bobChannel.ReceiveHTLC(htlc); err != nil {
// 		t.Fatalf("bob unable to recv add htlc: %v", err)
// 	}
// }

// // TestChainWatcherRemoteUnilateralClosePendingCommit tests that the chain
// // watcher is able to properly detect a unilateral close wherein the remote
// // node broadcasts their newly received commitment, without first revoking the
// // old one.
// func TestChainWatcherRemoteUnilateralClosePendingCommit(t *testing.T) {
// 	t.Parallel()

// 	// First, we'll create two channels which already have established a
// 	// commitment contract between themselves.
// 	aliceChannel, bobChannel, cleanUp, err := lnwallet.CreateTestChannels(true)
// 	if err != nil {
// 		t.Fatalf("unable to create test channels: %v", err)
// 	}
// 	defer cleanUp()

// 	// With the channels created, we'll now create a chain watcher instance
// 	// which will be watching for any closes of Alice's channel.
// 	aliceNotifier := &mockNotifier{
// 		spendChan: make(chan *chainntnfs.SpendDetail),
// 	}
// 	aliceChainWatcher, err := newChainWatcher(chainWatcherConfig{
// 		chanState:           aliceChannel.State(),
// 		notifier:            aliceNotifier,
// 		signer:              aliceChannel.Signer,
// 		extractStateNumHint: lnwallet.GetStateNumHint,
// 	})
// 	if err != nil {
// 		t.Fatalf("unable to create chain watcher: %v", err)
// 	}
// 	if err := aliceChainWatcher.Start(); err != nil {
// 		t.Fatalf("unable to start chain watcher: %v", err)
// 	}
// 	defer aliceChainWatcher.Stop()

// 	// We'll request a new channel event subscription from Alice's chain
// 	// watcher.
// 	chanEvents := aliceChainWatcher.SubscribeChannelEvents()

// 	// Next, we'll create a fake HTLC just so we can advance Alice's
// 	// channel state to a new pending commitment on her remote commit chain
// 	// for Bob.
// 	htlcAmount := lnwire.NewMSatFromSatoshis(20000)
// 	addFakeHTLC(t, htlcAmount, 0, aliceChannel, bobChannel)

// 	// With the HTLC added, we'll now manually initiate a state transition
// 	// from Alice to Bob.
// 	_, _, _, err = aliceChannel.SignNextCommitment()
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	// At this point, we'll now Bob broadcasting this new pending unrevoked
// 	// commitment.
// 	bobPendingCommit, err := aliceChannel.State().RemoteCommitChainTip()
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	// We'll craft a fake spend notification with Bob's actual commitment.
// 	// The chain watcher should be able to detect that this is a pending
// 	// commit broadcast based on the state hints in the commitment.
// 	bobCommit := bobPendingCommit.Commitment.CommitTx
// 	bobTxHash := bobCommit.TxHash()
// 	bobSpend := &chainntnfs.SpendDetail{
// 		SpenderTxHash: &bobTxHash,
// 		SpendingTx:    bobCommit,
// 	}
// 	aliceNotifier.spendChan <- bobSpend

// 	// We should get a new spend event over the remote unilateral close
// 	// event channel.
// 	var uniClose *RemoteUnilateralCloseInfo
// 	select {
// 	case uniClose = <-chanEvents.RemoteUnilateralClosure:
// 	case <-time.After(time.Second * 15):
// 		t.Fatalf("didn't receive unilateral close event")
// 	}

// 	// The unilateral close should have properly located Alice's output in
// 	// the commitment transaction.
// 	if uniClose.CommitResolution == nil {
// 		t.Fatalf("unable to find alice's commit resolution")
// 	}
// }

// // dlpTestCase is a special struct that we'll use to generate randomized test
// // cases for the main TestChainWatcherDataLossProtect test. This struct has a
// // special Generate method that will generate a random state number, and a
// // broadcast state number which is greater than that state number.
// type dlpTestCase struct {
// 	BroadcastStateNum uint8
// 	NumUpdates        uint8
// }

// func executeStateTransitions(t *testing.T, htlcAmount lnwire.MilliSatoshi,
// 	aliceChannel, bobChannel *lnwallet.LightningChannel,
// 	numUpdates uint8) error {

// 	for i := 0; i < int(numUpdates); i++ {
// 		addFakeHTLC(
// 			t, htlcAmount, uint64(i), aliceChannel, bobChannel,
// 		)

// 		err := lnwallet.ForceStateTransition(aliceChannel, bobChannel)
// 		if err != nil {
// 			return err
// 		}
// 	}

// 	return nil
// }

// // TestChainWatcherDataLossProtect tests that if we've lost data (and are
// // behind the remote node), then we'll properly detect this case and dispatch a
// // remote force close using the obtained data loss commitment point.
// func TestChainWatcherDataLossProtect(t *testing.T) {
// 	t.Parallel()

// 	// dlpScenario is our primary quick check testing function for this
// 	// test as whole. It ensures that if the remote party broadcasts a
// 	// commitment that is beyond our best known commitment for them, and
// 	// they don't have a pending commitment (one we sent but which hasn't
// 	// been revoked), then we'll properly detect this case, and execute the
// 	// DLP protocol on our end.
// 	//
// 	// broadcastStateNum is the number that we'll trick Alice into thinking
// 	// was broadcast, while numUpdates is the actual number of updates
// 	// we'll execute. Both of these will be random 8-bit values generated
// 	// by testing/quick.
// 	dlpScenario := func(t *testing.T, testCase dlpTestCase) bool {
// 		// First, we'll create two channels which already have
// 		// established a commitment contract between themselves.
// 		aliceChannel, bobChannel, cleanUp, err := lnwallet.CreateTestChannels(
// 			false,
// 		)
// 		if err != nil {
// 			t.Fatalf("unable to create test channels: %v", err)
// 		}
// 		defer cleanUp()

// 		// With the channels created, we'll now create a chain watcher
// 		// instance which will be watching for any closes of Alice's
// 		// channel.
// 		aliceNotifier := &mockNotifier{
// 			spendChan: make(chan *chainntnfs.SpendDetail),
// 		}
// 		aliceChainWatcher, err := newChainWatcher(chainWatcherConfig{
// 			chanState: aliceChannel.State(),
// 			notifier:  aliceNotifier,
// 			signer:    aliceChannel.Signer,
// 			extractStateNumHint: func(*wire.MsgTx,
// 				[lnwallet.StateHintSize]byte) uint64 {

// 				// We'll return the "fake" broadcast commitment
// 				// number so we can simulate broadcast of an
// 				// arbitrary state.
// 				return uint64(testCase.BroadcastStateNum)
// 			},
// 		})
// 		if err != nil {
// 			t.Fatalf("unable to create chain watcher: %v", err)
// 		}
// 		if err := aliceChainWatcher.Start(); err != nil {
// 			t.Fatalf("unable to start chain watcher: %v", err)
// 		}
// 		defer aliceChainWatcher.Stop()

// 		// Based on the number of random updates for this state, make a
// 		// new HTLC to add to the commitment, and then lock in a state
// 		// transition.
// 		const htlcAmt = 1000
// 		err = executeStateTransitions(
// 			t, htlcAmt, aliceChannel, bobChannel, testCase.NumUpdates,
// 		)
// 		if err != nil {
// 			t.Errorf("unable to trigger state "+
// 				"transition: %v", err)
// 			return false
// 		}

// 		// We'll request a new channel event subscription from Alice's
// 		// chain watcher so we can be notified of our fake close below.
// 		chanEvents := aliceChainWatcher.SubscribeChannelEvents()

// 		// Otherwise, we'll feed in this new state number as a response
// 		// to the query, and insert the expected DLP commit point.
// 		dlpPoint := aliceChannel.State().RemoteCurrentRevocation
// 		err = aliceChannel.State().MarkDataLoss(dlpPoint)
// 		if err != nil {
// 			t.Errorf("unable to insert dlp point: %v", err)
// 			return false
// 		}

// 		// Now we'll trigger the channel close event to trigger the
// 		// scenario.
// 		bobCommit := bobChannel.State().LocalCommitment.CommitTx
// 		bobTxHash := bobCommit.TxHash()
// 		bobSpend := &chainntnfs.SpendDetail{
// 			SpenderTxHash: &bobTxHash,
// 			SpendingTx:    bobCommit,
// 		}
// 		aliceNotifier.spendChan <- bobSpend

// 		// We should get a new uni close resolution that indicates we
// 		// processed the DLP scenario.
// 		var uniClose *RemoteUnilateralCloseInfo
// 		select {
// 		case uniClose = <-chanEvents.RemoteUnilateralClosure:
// 			// If we processed this as a DLP case, then the remote
// 			// party's commitment should be blank, as we don't have
// 			// this up to date state.
// 			blankCommit := channeldb.ChannelCommitment{}
// 			if uniClose.RemoteCommit.FeePerKw != blankCommit.FeePerKw {
// 				t.Errorf("DLP path not executed")
// 				return false
// 			}

// 			// The resolution should have also read the DLP point
// 			// we stored above, and used that to derive their sweep
// 			// key for this output.
// 			sweepTweak := input.SingleTweakBytes(
// 				dlpPoint,
// 				aliceChannel.State().LocalChanCfg.PaymentBasePoint.PubKey,
// 			)
// 			commitResolution := uniClose.CommitResolution
// 			resolutionTweak := commitResolution.SelfOutputSignDesc.SingleTweak
// 			if !bytes.Equal(sweepTweak, resolutionTweak) {
// 				t.Errorf("sweep key mismatch: expected %x got %x",
// 					sweepTweak, resolutionTweak)
// 				return false
// 			}

// 			return true

// 		case <-time.After(time.Second * 5):
// 			t.Errorf("didn't receive unilateral close event")
// 			return false
// 		}
// 	}

// 	testCases := []dlpTestCase{
// 		// For our first scenario, we'll ensure that if we're on state 1,
// 		// and the remote party broadcasts state 2 and we don't have a
// 		// pending commit for them, then we'll properly detect this as a
// 		// DLP scenario.
// 		{
// 			BroadcastStateNum: 2,
// 			NumUpdates:        1,
// 		},

// 		// We've completed a single update, but the remote party broadcasts
// 		// a state that's 5 states byeond our best known state. We've lost
// 		// data, but only partially, so we should enter a DLP secnario.
// 		{
// 			BroadcastStateNum: 6,
// 			NumUpdates:        1,
// 		},

// 		// Similar to the case above, but we've done more than one
// 		// update.
// 		{
// 			BroadcastStateNum: 6,
// 			NumUpdates:        3,
// 		},

// 		// We've done zero updates, but our channel peer broadcasts a
// 		// state beyond our knowledge.
// 		{
// 			BroadcastStateNum: 10,
// 			NumUpdates:        0,
// 		},
// 	}
// 	for _, testCase := range testCases {
// 		testName := fmt.Sprintf("num_updates=%v,broadcast_state_num=%v",
// 			testCase.NumUpdates, testCase.BroadcastStateNum)

// 		testCase := testCase
// 		t.Run(testName, func(t *testing.T) {
// 			t.Parallel()

// 			if !dlpScenario(t, testCase) {
// 				t.Fatalf("test %v failed", testName)
// 			}
// 		})
// 	}
// }

// // TestChainWatcherLocalForceCloseDetect tests we're able to always detect our
// // commitment output based on only the outputs present on the transaction.
// func TestChainWatcherLocalForceCloseDetect(t *testing.T) {
// 	t.Parallel()

// 	// localForceCloseScenario is the primary test we'll use to execute our
// 	// table driven tests. We'll assert that for any number of state
// 	// updates, and if the commitment transaction has our output or not,
// 	// we're able to properly detect a local force close.
// 	localForceCloseScenario := func(t *testing.T, numUpdates uint8,
// 		remoteOutputOnly, localOutputOnly bool) bool {

// 		// First, we'll create two channels which already have
// 		// established a commitment contract between themselves.
// 		aliceChannel, bobChannel, cleanUp, err := lnwallet.CreateTestChannels(
// 			false,
// 		)
// 		if err != nil {
// 			t.Fatalf("unable to create test channels: %v", err)
// 		}
// 		defer cleanUp()

// 		// With the channels created, we'll now create a chain watcher
// 		// instance which will be watching for any closes of Alice's
// 		// channel.
// 		aliceNotifier := &mockNotifier{
// 			spendChan: make(chan *chainntnfs.SpendDetail),
// 		}
// 		aliceChainWatcher, err := newChainWatcher(chainWatcherConfig{
// 			chanState:           aliceChannel.State(),
// 			notifier:            aliceNotifier,
// 			signer:              aliceChannel.Signer,
// 			extractStateNumHint: lnwallet.GetStateNumHint,
// 		})
// 		if err != nil {
// 			t.Fatalf("unable to create chain watcher: %v", err)
// 		}
// 		if err := aliceChainWatcher.Start(); err != nil {
// 			t.Fatalf("unable to start chain watcher: %v", err)
// 		}
// 		defer aliceChainWatcher.Stop()

// 		// We'll execute a number of state transitions based on the
// 		// randomly selected number from testing/quick. We do this to
// 		// get more coverage of various state hint encodings beyond 0
// 		// and 1.
// 		const htlcAmt = 1000
// 		err = executeStateTransitions(
// 			t, htlcAmt, aliceChannel, bobChannel, numUpdates,
// 		)
// 		if err != nil {
// 			t.Errorf("unable to trigger state "+
// 				"transition: %v", err)
// 			return false
// 		}

// 		// We'll request a new channel event subscription from Alice's
// 		// chain watcher so we can be notified of our fake close below.
// 		chanEvents := aliceChainWatcher.SubscribeChannelEvents()

// 		// Next, we'll obtain Alice's commitment transaction and
// 		// trigger a force close. This should cause her to detect a
// 		// local force close, and dispatch a local close event.
// 		aliceCommit := aliceChannel.State().LocalCommitment.CommitTx

// 		// Since this is Alice's commitment, her output is always first
// 		// since she's the one creating the HTLCs (lower balance). In
// 		// order to simulate the commitment only having the remote
// 		// party's output, we'll remove Alice's output.
// 		if remoteOutputOnly {
// 			aliceCommit.TxOut = aliceCommit.TxOut[1:]
// 		}
// 		if localOutputOnly {
// 			aliceCommit.TxOut = aliceCommit.TxOut[:1]
// 		}

// 		aliceTxHash := aliceCommit.TxHash()
// 		aliceSpend := &chainntnfs.SpendDetail{
// 			SpenderTxHash: &aliceTxHash,
// 			SpendingTx:    aliceCommit,
// 		}
// 		aliceNotifier.spendChan <- aliceSpend

// 		// We should get a local force close event from Alice as she
// 		// should be able to detect the close based on the commitment
// 		// outputs.
// 		select {
// 		case <-chanEvents.LocalUnilateralClosure:
// 			return true

// 		case <-time.After(time.Second * 5):
// 			t.Errorf("didn't get local for close for state #%v",
// 				numUpdates)
// 			return false
// 		}
// 	}

// 	// For our test cases, we'll ensure that we test having a remote output
// 	// present and absent with non or some number of updates in the channel.
// 	testCases := []struct {
// 		numUpdates       uint8
// 		remoteOutputOnly bool
// 		localOutputOnly  bool
// 	}{
// 		{
// 			numUpdates:       0,
// 			remoteOutputOnly: true,
// 		},
// 		{
// 			numUpdates:       0,
// 			remoteOutputOnly: false,
// 		},
// 		{
// 			numUpdates:      0,
// 			localOutputOnly: true,
// 		},
// 		{
// 			numUpdates:       20,
// 			remoteOutputOnly: false,
// 		},
// 		{
// 			numUpdates:       20,
// 			remoteOutputOnly: true,
// 		},
// 		{
// 			numUpdates:      20,
// 			localOutputOnly: true,
// 		},
// 	}
// 	for _, testCase := range testCases {
// 		testName := fmt.Sprintf(
// 			"num_updates=%v,remote_output=%v,local_output=%v",
// 			testCase.numUpdates, testCase.remoteOutputOnly,
// 			testCase.localOutputOnly,
// 		)

// 		testCase := testCase
// 		t.Run(testName, func(t *testing.T) {
// 			t.Parallel()

// 			localForceCloseScenario(
// 				t, testCase.numUpdates, testCase.remoteOutputOnly,
// 				testCase.localOutputOnly,
// 			)
// 		})
// 	}
// }
