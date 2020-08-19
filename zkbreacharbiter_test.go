// +build !rpctest

package lnd

import (
	"encoding/hex"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"github.com/lightningnetwork/lnd/chainntnfs"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/contractcourt"
	"github.com/lightningnetwork/lnd/htlcswitch"
	"github.com/lightningnetwork/lnd/libzkchannels"
	"github.com/lightningnetwork/lnd/lnwallet"
	"github.com/lightningnetwork/lnd/lnwallet/chainfee"
	"github.com/lightningnetwork/lnd/zkchanneldb"
	"github.com/lightningnetwork/lnd/zkchannels"
)

// createTestZkArbiter instantiates a breach arbiter with a failing retribution
// store, so that controlled failures can be tested.
func createTestZkArbiter(t *testing.T, custContractBreaches chan *ZkContractBreachEvent,
	db *channeldb.DB) (*zkBreachArbiter, func(), error) {

	aliceKeyPriv, _ := btcec.PrivKeyFromBytes(btcec.S256(),
		alicesPrivKey)
	signer := &mockSigner{key: aliceKeyPriv}

	// Assemble our test arbiter.
	notifier := makeMockSpendNotifier()

	testDir, err := ioutil.TempDir("", "zkchanneldb")
	if err != nil {
		t.Fatalf("unable to create temp directory: %v", err)
	}

	chainNotifier := &mockNotifier{
		oneConfChannel: make(chan *chainntnfs.TxConfirmation, 1),
		sixConfChannel: make(chan *chainntnfs.TxConfirmation, 1),
		epochChan:      make(chan *chainntnfs.BlockEpoch, 2),
	}

	netParams := activeNetParams.Params
	estimator := chainfee.NewStaticEstimator(62500, 0)

	wc := &mockWalletController{
		rootKey: alicePrivKey,
	}

	bio := &mockChainIO{
		bestHeight: fundingBroadcastHeight,
	}

	dbDir := filepath.Join(testDir, "lnwallet")
	cdb, err := channeldb.Open(dbDir)
	if err != nil {
		return nil, nil, err
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

	ba := newZkBreachArbiter(&ZkBreachConfig{
		CloseLink:            func(_ *wire.OutPoint, _ htlcswitch.ChannelCloseType) {},
		DBPath:               path.Join(testDir, "zkmerch.db"),
		DB:                   db,
		Estimator:            chainfee.NewStaticEstimator(12500, 0),
		GenSweepScript:       func() ([]byte, error) { return nil, nil },
		CustContractBreaches: custContractBreaches,
		Signer:               signer,
		Notifier:             notifier,
		PublishTransaction:   func(_ *wire.MsgTx, _ string) error { return nil },
		Wallet:               lnw,
		// Store:              store,
	})

	if err := ba.Start(); err != nil {
		return nil, nil, err
	}

	// The caller is responsible for closing the database.
	cleanUp := func() {
		if testDir != "" {
			os.RemoveAll(testDir)
		}
		ba.Stop()
	}

	return ba, cleanUp, nil
}

func initZkBreachedState(t *testing.T) (*zkBreachArbiter,
	*lnwallet.LightningChannel, chan *ZkContractBreachEvent,
	func(), func()) {
	// Create a pair of channels using a notifier that allows us to signal
	// a spend of the funding transaction. Alice's channel will be the on
	// observing a breach.
	alice, _, cleanUpChans, err := createInitChannels(1)
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

	return brar, alice, custContractBreaches, cleanUpChans,
		cleanUpArb
}

func initTestMerchState(DBPath, skM, payoutSkM, childSkM, disputeSkM string) error {
	dbURL := "redis://127.0.0.1/"
	selfDelay := int16(1487)
	channelState, err := libzkchannels.ChannelSetup("channel", selfDelay, 546, 546, 1000, false)
	if err != nil {
		return err
	}

	channelState, merchState, err := libzkchannels.InitMerchant(dbURL, channelState, "merch")
	if err != nil {
		return err
	}

	channelState, merchState, err = libzkchannels.LoadMerchantWallet(merchState, channelState, skM, payoutSkM, childSkM, disputeSkM)
	if err != nil {
		return err
	}
	zkchLog.Infof("merchState: %v", merchState)

	testDB, err := zkchanneldb.SetupMerchDB(DBPath)
	if err != nil {
		return err
	}

	err = zkchanneldb.AddMerchState(testDB, merchState)
	if err != nil {
		return err
	}

	err = zkchanneldb.AddMerchField(testDB, channelState, channelStateKey)
	if err != nil {
		return err
	}

	err = testDB.Close()
	if err != nil {
		return err
	}

	return err
}

func setupMerchChannelState(escrowTxid string, DBPath string, newStatus string) error {
	zkMerchDB, err := zkchanneldb.OpenMerchBucket(DBPath)
	if err != nil {
		zkchLog.Error(err)
		return err
	}

	merchState, err := zkchanneldb.GetMerchState(zkMerchDB)
	if err != nil {
		zkchLog.Error(err)
		return err
	}

	// Flip escrowTxid to Big Endian to match how it is stored in
	// merchState.ChannelStatusMap
	//This works because hex strings are of even size
	s := ""
	for i := 0; i < len(escrowTxid); i += 2 {
		s = escrowTxid[i:i+2] + s
	}
	escrowTxidBigEn := s

	(*merchState.ChannelStatusMap)[escrowTxidBigEn] = "CustomerInitClose"

	err = zkchanneldb.AddMerchState(zkMerchDB, merchState)
	if err != nil {
		zkchLog.Error(err)
		return err
	}

	err = zkMerchDB.Close()
	if err != nil {
		zkchLog.Error(err)
		return err
	}

	return nil
}

var zkBreachTests = []breachTest{
	{
		name: "revokedCustClose",
	},
}

// TestBreachSpends checks the behavior of the breach arbiter in response to
// spend events on a channels outputs by asserting that it properly removes or
// modifies the inputs from the justice txn.
func TestZkBreachSpends(t *testing.T) {
	tc := zkBreachTests[0]
	t.Run(tc.name, func(t *testing.T) {
		testZkBreachSpends(t, tc)
	})
}

func testZkBreachSpends(t *testing.T, test breachTest) {
	brar, _, custContractBreaches,
		cleanUpChans, cleanUpArb := initZkBreachedState(t)
	defer cleanUpChans()
	defer cleanUpArb()

	_, escrowTxid, err := zkchannels.HardcodedTxs("escrow")
	if err != nil {
		t.Error(err)
	}
	_, custCloseTxid, err := zkchannels.HardcodedTxs("revokedCustClose")
	if err != nil {
		t.Error(err)
	}

	// Use hardcoded zkchannel fundingOutpoint
	var escrowTxidHash chainhash.Hash
	err = chainhash.Decode(&escrowTxidHash, escrowTxid)
	if err != nil {
		t.Fatal(err)
	}

	// Use hardcoded zkchannel fundingOutpoint
	var custCloseTxidHash chainhash.Hash
	err = chainhash.Decode(&custCloseTxidHash, custCloseTxid)
	if err != nil {
		t.Fatal(err)
	}

	closePkScript, revLock, revSecret, custClosePk, amount := zkchannels.InitBreachConstants()

	ClosePkScript, err := hex.DecodeString(closePkScript)
	if err != nil {
		log.Fatal(err)
	}

	fundingOut := &wire.OutPoint{
		Hash:  escrowTxidHash,
		Index: uint32(0),
	}

	var (
		chanPoint = fundingOut
		publTx    = make(chan *wire.MsgTx)
		publErr   error
		publMtx   sync.Mutex
	)

	// Make PublishTransaction always return ErrDoubleSpend to begin with.
	publErr = lnwallet.ErrDoubleSpend
	brar.cfg.PublishTransaction = func(tx *wire.MsgTx, label string) error {
		publTx <- tx

		publMtx.Lock()
		defer publMtx.Unlock()
		return publErr
	}

	// Load hard coded example
	ZkCustBreachInfo := contractcourt.ZkBreachInfo{
		IsMerchClose:  false,
		EscrowTxid:    escrowTxidHash,
		CloseTxid:     custCloseTxidHash,
		ClosePkScript: ClosePkScript,
		RevLock:       revLock,
		RevSecret:     revSecret,
		CustClosePk:   custClosePk,
		Amount:        amount,
	}

	// Set up merchState and channelState in a temporary db
	_, skM, payoutSkM, childSkM, disputeSkM := zkchannels.InitMerchConstants()
	err = initTestMerchState(brar.cfg.DBPath, skM, payoutSkM, childSkM, disputeSkM)
	if err != nil {
		log.Fatal(err)
	}
	err = setupMerchChannelState(escrowTxid, brar.cfg.DBPath, "CustomerInitClose")
	if err != nil {
		log.Fatal(err)
	}

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

	notifier := brar.cfg.Notifier.(*mockSpendNotifier)
	notifier.confChannel <- &chainntnfs.TxConfirmation{}

	var disputeTx *wire.MsgTx
	select {
	case disputeTx = <-publTx:
	case <-time.After(5 * time.Second):
		t.Fatalf("disputeTx was not published")
	}

	// All outputs should initially spend from the force closed txn.
	forceTxID := custCloseTxid
	for _, txIn := range disputeTx.TxIn {
		if txIn.PreviousOutPoint.Hash.String() != forceTxID {
			t.Fatalf("dispute tx not spending commitment %v", forceTxID)
		}
	}

	if len(disputeTx.TxOut) != 1 {
		t.Fatalf("dispute tx should only have 1 output, but it has %v", len(disputeTx.TxOut))
	}
	// Compare weight of publishedTx vs value used for calculating txFee
	expected := int64(disputeTxKW * 1000) // (upper bound)
	actual := blockchain.GetTransactionWeight(btcutil.NewTx(disputeTx))
	// allow for 1 byte variation in scriptSig signature length.
	if d := expected - actual; d > 1 || d < 0 {
		t.Errorf("disputeTx did not have the expected weight. Expected: %v, got: %v", expected, actual)
	}

}
