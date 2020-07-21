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

	// escrowTx   = "020000000001018d2744b606be69fd45d3661e1a61b81ec66d1417941ae33c4ac7079f2bd4aee80000000017160014d83e1345c76dc160630937746d2d2693562e9c58ffffffff0280841e000000000022002092eeef2d000b5585d8809fb56bd0d9df7d6f6a337acbb2261a08f1f38ba958328c5ad7050000000016001461492b43be394b9e6eeb077f17e73665bbfd455b02483045022100a7577e466b875da137bd1d5503dedf91b5833ed47002026bafacf4b8632dedce0220276ca6b5dadb7a08d5d53830c81109a3da9059cb4f80f7ceeda15c2ac3cd0e660121032581c94e62b16c1f5fc36c5ff6ddc5c3e7cc4e7e70e2ec3ab3f663cff9d9b7d000000000"
	escrowTxid = "43e3075ffc665e085040cfbc0a0f63b4cb43dd95830f6d2c22a78112787c3dfe"

	custCloseTxid = "b5af75a0d34c54e26dae1ef1290623b9208b11e1a368fa71f414d061bf3dab3c"
	// custCloseTx   = "02000000000101fe3d7c781281a7222c6d0f8395dd43cbb4630f0abccf4050085e66fc5f07e3430000000000ffffffff04703a0f000000000022002048e63675919d6590fc2277a9364b21dcaa93c9b8b2e193e0c96dd4667958c30440420f000000000016001403d761347fe4398f4ab398266a830ead7c9c2f300000000000000000436a41ff83f645eb906ca56957c8af2fa8445c5999427ef3849cf36d729e8968d2178102861c8266f6e0188866f53a3dbb5efeccd8c94d648802bf53bacae9b8fc524bbee803000000000000160014d3450dd48043928d769ec3ef4f249498009bc064040047304402202bbd63c1d1d07480a45a4d5584712bd61fb384dd99b265b6f72cf71fcc419f67022064647b1c198766f1d91c058271a03eeb27efa190e9184bb26954585729cb3e7a0147304402201fda4dce371e10d582c0e3c6ea64c006c613b49fd76f4d3e241411a0d4ec4c5802201dbefbc1cf87dcdbac3564b0e20d7ddf87540cbb17a5b302898f8f8535c33ed501475221038c2add1dc8cf2c57bac6e19d1f963e0c42103554e8b35e425bc2a78f4c22b273210344147c5d54f894340b151072f80039cea708ecec802aa8cca247c48fce862a9e52ae00000000"

	closePkScript = "002072e2c63c7d43aa00de70c445e915b4d9157e270129a4852e0c89c44f644a9757"

	// merchCloseTxid = "df2c79fc0122aa587d1e080ea324ff67b244f9d00f3b5e45e0053a8a2a448343"
	// merchCloseTx = "02000000000101fe3d7c781281a7222c6d0f8395dd43cbb4630f0abccf4050085e66fc5f07e3430000000000ffffffff02b07c1e0000000000220020a9b24b8eeef1d0eb1c55eeb11009ea9ce9b490fc742943ab8cfa314e1d15b177e80300000000000016001430d0e52d62063f511cf71bdd8ae633bd514503af0400483045022100c19205df0ffe46139bda9d7fd76e5b302cf4c72d0c2fb88b1885693c11c9a64902204bd3ec8552867b09f7644e2115f434f55cb4d87aaa1cb8b3113990977c8cffb7014830450221009f1252e3698a0856168fedccb895d622ece300c6a99b6d9aca969a7ddf7f6169022001285b9cbd0f04d6b33a1706c8efe0e2dc7d4f59fe32b77f408b3e0e42c15f8401475221038c2add1dc8cf2c57bac6e19d1f963e0c42103554e8b35e425bc2a78f4c22b273210344147c5d54f894340b151072f80039cea708ecec802aa8cca247c48fce862a9e52ae00000000"

	custCloseMerchTxid = "7b6aa0ebc1a9df8ae77f1a63a04cb8a4cd61621823330af5c7e8c4038d72ad1a"
	// custCloseMerchTx   = "020000000001014383442a8a3a05e0455e3b0fd0f944b267ff24a30e081e7d58aa2201fc792cdf0000000000ffffffff04703a0f000000000022002048e63675919d6590fc2277a9364b21dcaa93c9b8b2e193e0c96dd4667958c304703a0f000000000016001403d761347fe4398f4ab398266a830ead7c9c2f300000000000000000436a41ff83f645eb906ca56957c8af2fa8445c5999427ef3849cf36d729e8968d2178102861c8266f6e0188866f53a3dbb5efeccd8c94d648802bf53bacae9b8fc524bbee803000000000000160014d3450dd48043928d769ec3ef4f249498009bc0640500473044022019e8f472c4710ca0d49674660df8012680431c9e1416c0ecb90802b9bfe9886f02206d21e5e4ba35f533fe95cc13fe64df06cab8f2ca07c71ad349acdc1c83145ba601483045022100f18efbf0d254dfde2e34944696b6498bf1d2db251a5cbcdb0b634e96134ffa83022062cb4691d89c648d630306b33520d98b399357b8448dc60b8afbed75ebc9539401010172635221038c2add1dc8cf2c57bac6e19d1f963e0c42103554e8b35e425bc2a78f4c22b273210344147c5d54f894340b151072f80039cea708ecec802aa8cca247c48fce862a9e52ae6702cf05b2752103780cd60a7ffeb777ec337e2c177e783625c4de907a4aee0f41269cc612fba457ac6800000000"

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

	channelState, merchState, err = libzkchannels.LoadMerchantWallet(merchState, channelState, skM, payoutSkM, disputeSkM)
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
	for i := 0; i < len(escrowTxid)/2; i++ {
		s = escrowTxid[i*2:i*2+2] + s
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

	// notifier := brar.cfg.Notifier.(*mockSpendNotifier)
	// notifier.confChannel <- &chainntnfs.TxConfirmation{}

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

	// // Deliver confirmation of sweep if the test expects it.
	// if test.sendFinalConf {
	// 	notifier.confChannel <- &chainntnfs.TxConfirmation{}
	// }
}
