// +build !rpctest

package lnd

import (
	"encoding/hex"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path"
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
)

// zkchannel test txs
const (
	skM        = "e6e0c5310bb03809e1b2a1595a349f002125fa557d481e51f401ddaf3287e6ae"
	payoutSkM  = "5611111111111111111111111111111100000000000000000000000000000000"
	disputeSkM = "5711111111111111111111111111111100000000000000000000000000000000"

	// escrowTx   = "020000000001018d2744b606be69fd45d3661e1a61b81ec66d1417941ae33c4ac7079f2bd4aee80000000017160014d83e1345c76dc160630937746d2d2693562e9c58ffffffff0280841e0000000000220020b718e637a405ba914aede6bcb3d14dec83deca9b0449a20c7163b94456eb01c3806de7290100000016001461492b43be394b9e6eeb077f17e73665bbfd455b02483045022100cb3289b503a08250e7562f1ed4072065ef951d6bee0c02ea555a542c9650d30f02205eb0a7e8a471074a6f8a632033e484fa4a1e20dd2bdf9a73b160ad7f761fe9ea0121032581c94e62b16c1f5fc36c5ff6ddc5c3e7cc4e7e70e2ec3ab3f663cff9d9b7d000000000"
	escrowTxid = "7481edd312265b6fa916e1cc79cf427f73fe9fe7b44670f247bfd733de1aac80"

	custCloseTxid = "99c74362e62b3301620274e7a562c2d40a9727e318cfa523b9d58cf6c34a65e8"
	// custCloseTx   = "0200000000010180ac1ade33d7bf47f27046b4e79ffe737f42cf79cce116a96f5b2612d3ed81740000000000ffffffff04703a0f00000000002200201a5b9a0624746b614bf9ca504810ab94a2d26eda4e8db2a2e38c7540884d7c0a40420f000000000016001403d761347fe4398f4ab398266a830ead7c9c2f300000000000000000436a41802143564b3fc76045db83e6919a68675dc02203ea79721507fb0998d2a259fd027160fb5e48252f02a00066dfa823d15844ad93e04f9c9b746e1f28ed4a1eaddbe803000000000000160014a496306b960746361e3528534d04b1ac4726655a040047304402200568a611afce83a37d5d4f82a5ec2ff913983c82f8016156b80d41a226491e3c022072029d950f82200cacff5e078ad553a2fc965c8f2d33422fbaa0a0966add2dc601483045022100e5386abb1d23a97c6176f63fbe9c5272c252814eaa8b83ab3d225e84f0e5dbf702206f45bf78139e60784a6be7d1aff53a056708111f6e863daeee5adaf9bb959e4a01475221038c2add1dc8cf2c57bac6e19d1f963e0c42103554e8b35e425bc2a78f4c22b273210217d55a1e3ecdd220fde4bddbbfd485a1596c0c5cb7ef11dbfcdb2dd9cf4b85af52ae00000000"

	closePkScript = "002072e2c63c7d43aa00de70c445e915b4d9157e270129a4852e0c89c44f644a9757"

	// merchCloseTxid = "7e4e51a76aaa9f23d6ada1522bfdb4ccefb5cd8875a4982bbb1bf02b6a99e203"
	// merchCloseTx = "0200000000010180ac1ade33d7bf47f27046b4e79ffe737f42cf79cce116a96f5b2612d3ed81740000000000ffffffff02b07c1e00000000002200204bbedbfa9d195e8fc160e6a237d0d702fdae3b5a2d9494beb048452c4647095ae80300000000000016001430d0e52d62063f511cf71bdd8ae633bd514503af04004830450221008bd7b4c25dcbaeb624776adc3e9851b5b5c5ee54d845ed6749c2e12d917e0a550220316529e58b8ac6a0eaf0fee04d3c4bd2d67316951cb8d29c4466c17ac3e4a94f01483045022100be920e94fd52426ff8e09e46132211ace0ad3115e97ff3f0f9209b225124c6db02206bc923bfa19bd9db9cc20269881969586f383b8689bb20728790b853109d111301475221038c2add1dc8cf2c57bac6e19d1f963e0c42103554e8b35e425bc2a78f4c22b273210217d55a1e3ecdd220fde4bddbbfd485a1596c0c5cb7ef11dbfcdb2dd9cf4b85af52ae00000000"

	// custCloseMerchTxid = "5deda336c2c1fa0886a3f2e00af54b653ae718043ab5315a4bb704eaf041cf2e"
	// custCloseMerchTx   = "0200000000010103e2996a2bf01bbb2b98a47588cdb5efccb4fd2b52a1add6239faa6aa7514e7e0000000000ffffffff04703a0f00000000002200201a5b9a0624746b614bf9ca504810ab94a2d26eda4e8db2a2e38c7540884d7c0a703a0f000000000016001403d761347fe4398f4ab398266a830ead7c9c2f300000000000000000436a41802143564b3fc76045db83e6919a68675dc02203ea79721507fb0998d2a259fd027160fb5e48252f02a00066dfa823d15844ad93e04f9c9b746e1f28ed4a1eaddbe803000000000000160014a496306b960746361e3528534d04b1ac4726655a0500483045022100c95109bb650ca742c14b53c8aa588806e04ba2e2f3cb5f44398a6207f909e14e022061ebdd6f47f466cb687d82acbaac45187313c11b6ad334a605e7885ee321071901483045022100c0433755dfa2e93a9bfb129637e9886a09c6454b5054b30b89c20591113160eb02207c66cf83fb5bd75b758b578c84cf364e03457ecbc5cec2a49b996e979623938f01010172635221038c2add1dc8cf2c57bac6e19d1f963e0c42103554e8b35e425bc2a78f4c22b273210217d55a1e3ecdd220fde4bddbbfd485a1596c0c5cb7ef11dbfcdb2dd9cf4b85af52ae6702cf05b2752103780cd60a7ffeb777ec337e2c177e783625c4de907a4aee0f41269cc612fba457ac6800000000"

	revLock     = "802143564b3fc76045db83e6919a68675dc02203ea79721507fb0998d2a259fd"
	revSecret   = "92bbe0429d2b7acef6ef5ea1e6afe8b3188d4f31c4b42f635a404365f81726f2"
	custClosePk = "027160fb5e48252f02a00066dfa823d15844ad93e04f9c9b746e1f28ed4a1eaddb"
	amount      = 997990
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

var zkBreachTests = []breachTest{
	{
		name: "revokedCustClose",
	},
}

func initTestMerchState(DBPath string, skM string, payoutSkM string, disputeSkM string) error {
	dbURL := ""
	selfDelay := int16(1487)
	channelState, err := libzkchannels.ChannelSetup("channel", selfDelay, 546, 546, 1000, false)
	if err != nil {
		return err
	}

	channelState, merchState, err := libzkchannels.InitMerchant(dbURL, channelState, "merch")
	if err != nil {
		return err
	}

	channelState, merchState, err = libzkchannels.LoadMerchantWallet(merchState, channelState, skM, payoutSkM, disputeSkM)
	if err != nil {
		return err
	}

	testDB, err := zkchanneldb.SetupDB(DBPath)
	if err != nil {
		return err
	}

	err = zkchanneldb.AddMerchState(testDB, merchState)
	if err != nil {
		return err
	}

	err = zkchanneldb.AddMerchField(testDB, channelState, "channelStateKey")
	if err != nil {
		return err
	}

	err = testDB.Close()
	if err != nil {
		return err
	}

	return err
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

	// Use hardcoded zkchannel fundingOutpoint
	var escrowTxidHash chainhash.Hash
	err := chainhash.Decode(&escrowTxidHash, escrowTxid)
	if err != nil {
		t.Fatal(err)
	}

	// Use hardcoded zkchannel fundingOutpoint
	var custCloseTxidHash chainhash.Hash
	err = chainhash.Decode(&custCloseTxidHash, custCloseTxid)
	if err != nil {
		t.Fatal(err)
	}

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
		EscrowTxid:    escrowTxidHash,
		CloseTxid:     custCloseTxidHash,
		ClosePkScript: ClosePkScript,
		RevLock:       revLock,
		RevSecret:     revSecret,
		CustClosePk:   custClosePk,
		Amount:        amount,
	}

	// Set up merchState and channelState in a temporary db
	err = initTestMerchState(brar.cfg.DBPath, skM, payoutSkM, disputeSkM)
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

	var tx *wire.MsgTx
	select {
	case tx = <-publTx:
	case <-time.After(5 * time.Second):
		t.Fatalf("tx was not published")
	}

	// All outputs should initially spend from the force closed txn.
	forceTxID := custCloseTxid
	for _, txIn := range tx.TxIn {
		if txIn.PreviousOutPoint.Hash.String() != forceTxID {
			t.Fatalf("dispute tx not spending commitment %v", forceTxID)
		}
	}

	if len(tx.TxOut) != 1 {
		t.Fatalf("dispute tx should only have 1 output, but it has %v", len(tx.TxOut))
	}

	// TODO ZKC-15: Add the check below when txFee has been added to MerchantSignDisputeTx
	// Calculate appropriate minimum tx fee and check it is large enough
	expectedWeight := blockchain.GetTransactionWeight(btcutil.NewTx(tx))

	// a minRelayTxFee of 1 satoshi per vbyte is equal to 0.253 satoshi per weight unit
	minTxFee := int64(math.Ceil(float64(expectedWeight) * 0.253))

	disputeTxFee := ZkCustBreachInfo.Amount - tx.TxOut[0].Value
	if disputeTxFee < minTxFee {
		t.Fatalf("disputeTx fee of %v sat is less than minRelayTxFee of %v sat", disputeTxFee, minTxFee)
	}

	// Deliver confirmation of sweep if the test expects it.
	if test.sendFinalConf {
		notifier.confChannel <- &chainntnfs.TxConfirmation{}
	}

}
