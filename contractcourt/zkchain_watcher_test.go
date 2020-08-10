package contractcourt

import (
	"bytes"
	"encoding/hex"
	"testing"
	"time"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/lightningnetwork/lnd/chainntnfs"
	"github.com/lightningnetwork/lnd/lnwallet/chainfee"
	"github.com/lightningnetwork/lnd/zkchannels"
)

// zkchannel test txs
const (
	escrowTx   = "020000000001018d2744b606be69fd45d3661e1a61b81ec66d1417941ae33c4ac7079f2bd4aee80000000017160014d83e1345c76dc160630937746d2d2693562e9c58ffffffff0280841e0000000000220020b718e637a405ba914aede6bcb3d14dec83deca9b0449a20c7163b94456eb01c3806de7290100000016001461492b43be394b9e6eeb077f17e73665bbfd455b02483045022100cb3289b503a08250e7562f1ed4072065ef951d6bee0c02ea555a542c9650d30f02205eb0a7e8a471074a6f8a632033e484fa4a1e20dd2bdf9a73b160ad7f761fe9ea0121032581c94e62b16c1f5fc36c5ff6ddc5c3e7cc4e7e70e2ec3ab3f663cff9d9b7d000000000"
	escrowTxid = "7481edd312265b6fa916e1cc79cf427f73fe9fe7b44670f247bfd733de1aac80"

	merchCloseTx   = "0200000000010180ac1ade33d7bf47f27046b4e79ffe737f42cf79cce116a96f5b2612d3ed81740000000000ffffffff02b07c1e00000000002200204bbedbfa9d195e8fc160e6a237d0d702fdae3b5a2d9494beb048452c4647095ae80300000000000016001430d0e52d62063f511cf71bdd8ae633bd514503af04004830450221008bd7b4c25dcbaeb624776adc3e9851b5b5c5ee54d845ed6749c2e12d917e0a550220316529e58b8ac6a0eaf0fee04d3c4bd2d67316951cb8d29c4466c17ac3e4a94f01483045022100be920e94fd52426ff8e09e46132211ace0ad3115e97ff3f0f9209b225124c6db02206bc923bfa19bd9db9cc20269881969586f383b8689bb20728790b853109d111301475221038c2add1dc8cf2c57bac6e19d1f963e0c42103554e8b35e425bc2a78f4c22b273210217d55a1e3ecdd220fde4bddbbfd485a1596c0c5cb7ef11dbfcdb2dd9cf4b85af52ae00000000"
	merchCloseTxid = "7e4e51a76aaa9f23d6ada1522bfdb4ccefb5cd8875a4982bbb1bf02b6a99e203"

	custCloseTx   = "0200000000010180ac1ade33d7bf47f27046b4e79ffe737f42cf79cce116a96f5b2612d3ed81740000000000ffffffff04703a0f0000000000220020b69ac2024f8d340320003dbe14fb02cf604680839eb0d6cac70dfce31aefa02f40420f000000000016001403d761347fe4398f4ab398266a830ead7c9c2f300000000000000000436a41e6b488e5fcf772078a79ecfc6a787a4aa13cab1782d317d09c84b013f5be48ff027160fb5e48252f02a00066dfa823d15844ad93e04f9c9b746e1f28ed4a1eaddbe803000000000000160014a496306b960746361e3528534d04b1ac4726655a04004730440220295696291e81970a3db52d124add3c45981fb47970e29e9bc3e2b8d8637af7c80220774a1c04a34b3a35cf05d86f1f0138aa2feda3882ac3ab23158310a3c9c7b4530147304402203f323da32dfd40e7b248236e2f86f264fc6ad3d298867049461066a48d6b5cf302206efae4371cfd6fa20e6ede9a8414d6184b129bf4d972520639c5b9863c4a53a901475221038c2add1dc8cf2c57bac6e19d1f963e0c42103554e8b35e425bc2a78f4c22b273210217d55a1e3ecdd220fde4bddbbfd485a1596c0c5cb7ef11dbfcdb2dd9cf4b85af52ae00000000"
	custCloseTxid = "13b04a68e5bc678d71f7533392d83ec7e6f1dd247e9ed68e4bfd0a28436250f3"

	// custCloseMerchTx = "0200000000010103e2996a2bf01bbb2b98a47588cdb5efccb4fd2b52a1add6239faa6aa7514e7e0000000000ffffffff04703a0f0000000000220020b69ac2024f8d340320003dbe14fb02cf604680839eb0d6cac70dfce31aefa02f703a0f000000000016001403d761347fe4398f4ab398266a830ead7c9c2f300000000000000000436a41e6b488e5fcf772078a79ecfc6a787a4aa13cab1782d317d09c84b013f5be48ff027160fb5e48252f02a00066dfa823d15844ad93e04f9c9b746e1f28ed4a1eaddbe803000000000000160014a496306b960746361e3528534d04b1ac4726655a05004830450221008fa4d30c116228934de9aa9636b0bf7331ecee4a785ae91417acba8df2400b5e02206e9fa60df1f4aa55a86970ad4cdc5b15b70fd98d4694b51af025b738465fed7e01483045022100d537d055947db9f5fc4971c17f65fca754c6caff89bb455c746f803adcb478ab0220451f6e92cea5ab72c672b79964000914aefd8c0c7ae6fd7a3d6e5cd08a8a02e601010172635221038c2add1dc8cf2c57bac6e19d1f963e0c42103554e8b35e425bc2a78f4c22b273210217d55a1e3ecdd220fde4bddbbfd485a1596c0c5cb7ef11dbfcdb2dd9cf4b85af52ae6702cf05b2752103780cd60a7ffeb777ec337e2c177e783625c4de907a4aee0f41269cc612fba457ac6800000000"
	// custCloseMerchTxid = "2259fb14bf4a4a30ca0bbb98e9c1bfc9aac8d887c024f395abae9071cc77f141"

	merchClaimTx   = "020000000001011168a2f0333adc67886e7dc9d83f1617346838988313f63749f7e76ac20c92600000000000cf050000011c841e000000000016001461492b43be394b9e6eeb077f17e73665bbfd455b0347304402204cd495f68921c1f171f07e735d693dcf5e390561561c02f5e1d0568fc457056e02202481cce621eda5737aeefb86b1a591578b7115f6673dff716df86c36787f38fc010072635221038c2add1dc8cf2c57bac6e19d1f963e0c42103554e8b35e425bc2a78f4c22b273210217d55a1e3ecdd220fde4bddbbfd485a1596c0c5cb7ef11dbfcdb2dd9cf4b85af52ae6702cf05b2752103780cd60a7ffeb777ec337e2c177e783625c4de907a4aee0f41269cc612fba457ac6800000000"
	merchClaimTxid = "dd1acbde4ab98446d08fda6aeabd9bd118710e11f3e128661cf26d2c2f7835d5"
)

func setupTestMerchChainWatcher(t *testing.T, isWatchingMerchClose bool, merchDBPath string) (*zkChainWatcher, *wire.OutPoint, *mockNotifier, error) {
	var escrowTxidHash chainhash.Hash
	err := chainhash.Decode(&escrowTxidHash, escrowTxid)
	if err != nil {
		t.Error(err)
	}

	outPoint := &wire.OutPoint{
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

	ZkFundingInfo := ZkFundingInfo{
		FundingOut:      *outPoint,
		PkScript:        pkScript,
		BroadcastHeight: uint32(0),
	}

	// With the channels created, we'll now create a chain watcher instance
	// which will be watching for any closes of Alice's channel.
	merchNotifier := &mockNotifier{
		spendChan: make(chan *chainntnfs.SpendDetail),
	}

	feeEstimator := zkchannels.NewMockFeeEstimator(10000, chainfee.FeePerKwFloor)
	merchChainWatcher, err := newZkChainWatcher(ZkChainWatcherConfig{
		IsMerch:            true,
		DBPath:             merchDBPath,
		Estimator:          feeEstimator,
		ZkFundingInfo:      ZkFundingInfo,
		Notifier:           merchNotifier,
		WatchingMerchClose: isWatchingMerchClose,
		zkContractBreach: func(zkInfo *ZkBreachInfo) error {
			return nil
		},
	})

	return merchChainWatcher, outPoint, merchNotifier, err
}

func setupTestCustChainWatcher(t *testing.T, isWatchingMerchClose bool, custDBPath string) (*zkChainWatcher, *wire.OutPoint, *mockNotifier, error) {
	var escrowTxidHash chainhash.Hash
	err := chainhash.Decode(&escrowTxidHash, escrowTxid)
	if err != nil {
		t.Error(err)
	}

	outPoint := &wire.OutPoint{
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

	ZkFundingInfo := ZkFundingInfo{
		FundingOut:      *outPoint,
		PkScript:        pkScript,
		BroadcastHeight: uint32(0),
	}

	// With the channels created, we'll now create a chain watcher instance
	// which will be watching for any closes of Alice's channel.
	custNotifier := &mockNotifier{
		spendChan: make(chan *chainntnfs.SpendDetail),
	}

	feeEstimator := zkchannels.NewMockFeeEstimator(10000, chainfee.FeePerKwFloor)

	custChainWatcher, err := newZkChainWatcher(ZkChainWatcherConfig{
		IsMerch:            false,
		DBPath:             custDBPath,
		Estimator:          feeEstimator,
		ZkFundingInfo:      ZkFundingInfo,
		Notifier:           custNotifier,
		WatchingMerchClose: isWatchingMerchClose,
		zkContractBreach: func(zkInfo *ZkBreachInfo) error {
			return nil
		},
	})

	return custChainWatcher, outPoint, custNotifier, err
}

// TestChainWatcherLocalMerchClose tests that the chain watcher is able
// to properly detect a local closure initiated by the merchant.
func TestChainWatcherLocalMerchClose(t *testing.T) {
	// t.Parallel()

	_, merchDBPath, err := zkchannels.SetupTempDBPaths()
	if err != nil {
		t.Fatalf("SetupTempDBPaths: %v", err)
	}

	merchChainWatcher, fundingOut, merchNotifier, err := setupTestMerchChainWatcher(t, false, merchDBPath)
	if err != nil {
		t.Fatalf("unable to create merchChainWatcher: %v", err)
	}
	err = merchChainWatcher.Start()
	if err != nil {
		t.Fatalf("unable to start merchChainWatcher: %v", err)
	}
	defer merchChainWatcher.Stop()

	// We'll request a new channel event subscription from merchant's chain
	// watcher.
	chanEvents := merchChainWatcher.SubscribeChannelEvents()

	var merchCloseTxidHash chainhash.Hash
	err = chainhash.Decode(&merchCloseTxidHash, merchCloseTxid)
	if err != nil {
		t.Error(err)
	}

	serializedMerchCTx, err := hex.DecodeString(merchCloseTx)
	if err != nil {
		t.Error(err)
	}

	var msgMerchCTx wire.MsgTx
	err = msgMerchCTx.Deserialize(bytes.NewReader(serializedMerchCTx))
	if err != nil {
		t.Error(err)
	}

	merchSpend := &chainntnfs.SpendDetail{
		SpentOutPoint: fundingOut,
		SpenderTxHash: &merchCloseTxidHash,
		SpendingTx:    &msgMerchCTx,
	}
	merchNotifier.spendChan <- merchSpend

	// We should get a new spend event over the local unilateral close
	// event channel.
	var uniClose *ZkMerchCloseInfo
	select {
	case uniClose = <-chanEvents.ZkMerchClosure:
		t.Logf("amount: %#v\n", uniClose.amount)
	case <-time.After(time.Second * 5):
		t.Fatalf("didn't receive ZkMerchClosure event")
	}

	if uniClose == nil {
		t.Fatalf("Did not receive ZkMerchCloseInfo for local closure")
	}
}

// TestChainWatcherRemoteMerchClose tests that the chain watcher is able
// to properly detect a remote closure initiated by the merchant.
func TestChainWatcherRemoteMerchClose(t *testing.T) {
	// t.Parallel()
	custDBPath, _, err := zkchannels.SetupTempDBPaths()
	if err != nil {
		t.Fatalf("SetupTempDBPaths: %v", err)
	}
	custChainWatcher, fundingOut, custNotifier, err := setupTestCustChainWatcher(t, false, custDBPath)
	if err != nil {
		t.Fatalf("unable to create custChainWatcher: %v", err)
	}
	err = custChainWatcher.Start()
	if err != nil {
		t.Fatalf("unable to start custChainWatcher: %v", err)
	}
	defer custChainWatcher.Stop()

	// We'll request a new channel event subscription from customer's chain
	// watcher.
	chanEvents := custChainWatcher.SubscribeChannelEvents()

	var merchCloseTxidHash chainhash.Hash
	err = chainhash.Decode(&merchCloseTxidHash, merchCloseTxid)
	if err != nil {
		t.Error(err)
	}

	serializedMerchCTx, err := hex.DecodeString(merchCloseTx)
	if err != nil {
		t.Error(err)
	}

	var msgMerchCTx wire.MsgTx
	err = msgMerchCTx.Deserialize(bytes.NewReader(serializedMerchCTx))
	if err != nil {
		t.Error(err)
	}

	merchSpend := &chainntnfs.SpendDetail{
		SpentOutPoint: fundingOut,
		SpenderTxHash: &merchCloseTxidHash,
		SpendingTx:    &msgMerchCTx,
	}
	custNotifier.spendChan <- merchSpend

	// We should get a new spend event over the remote merchClose
	// event channel.
	var uniClose *ZkBreachInfo
	select {
	case uniClose = <-chanEvents.ZkContractBreach:
		t.Logf("amount: %#v\n", uniClose.Amount)
	case <-time.After(time.Second * 5):
		t.Fatalf("didn't receive ZkContractBreach event")
	}

	if uniClose == nil {
		t.Fatalf("Did not receive ZkBreachInfo for remote merchClose")
	}
}

// TestZkChainWatcherLocalCustClose tests that the chain watcher is able
// to properly detect a local closure initiated by the customer.
// Note that this is used in both custCloseTx from Escrow and from merchClose.
func TestZkChainWatcherLocalCustClose(t *testing.T) {
	// t.Parallel()/
	custDBPath, _, err := zkchannels.SetupTempDBPaths()
	if err != nil {
		t.Fatalf("SetupTempDBPaths: %v", err)
	}
	custChainWatcher, fundingOut, custNotifier, err := setupTestCustChainWatcher(t, false, custDBPath)
	if err != nil {
		t.Fatalf("unable to create custChainWatcher: %v", err)
	}
	err = custChainWatcher.Start()
	if err != nil {
		t.Fatalf("unable to start custChainWatcher: %v", err)
	}
	defer custChainWatcher.Stop()

	// We'll request a new channel event subscription from customer's chain
	// watcher.
	chanEvents := custChainWatcher.SubscribeChannelEvents()

	var closeTxidHash chainhash.Hash
	err = chainhash.Decode(&closeTxidHash, custCloseTxid)
	if err != nil {
		t.Error(err)
	}

	serializedCustCTx, err := hex.DecodeString(custCloseTx)
	if err != nil {
		t.Error(err)
	}

	var msgCustCTx wire.MsgTx
	err = msgCustCTx.Deserialize(bytes.NewReader(serializedCustCTx))
	if err != nil {
		t.Error(err)
	}

	custSpend := &chainntnfs.SpendDetail{
		SpentOutPoint: fundingOut,
		SpenderTxHash: &closeTxidHash,
		SpendingTx:    &msgCustCTx,
	}
	custNotifier.spendChan <- custSpend

	// We should get a new spend event over the local unilateral close
	// event channel.
	var uniClose *ZkCustCloseInfo
	select {
	case uniClose = <-chanEvents.ZkCustClosure:
		t.Logf("ZkBreachInfo: %#v", *uniClose)
	case <-time.After(time.Second * 5):
		t.Fatalf("didn't receive ZkCustClosure event")
	}

	if uniClose == nil {
		t.Fatalf("unable to find ZkCustCloseInfo")
	}
}

// // TestZkChainWatcherRemoteValidCustClose tests that the chain watcher is able
// // to properly detect a local closure initiated by the customer.
// // 'Valid' custClose refers to the fact that it is the latest state, i.e. the
// // customer is not closing with a revoked state.
// // Note that this is used in both custCloseTx from Escrow and from merchClose.
// func TestZkChainWatcherRemoteValidCustClose(t *testing.T) {
// 	// t.Parallel()

// 	// This wont work until the zkmerch.db and zkcust.db have been setup
// 	zkchannels.SetupLibzkChannels("myChannel", custDBPath, merchDBPath)

// 	merchChainWatcher, fundingOut, merchNotifier, err := setupTestMerchChainWatcher(t, false, merchDBPath)

// 	if err != nil {
// 		t.Fatalf("unable to create merchChainWatcher: %v", err)
// 	}
// 	err = merchChainWatcher.Start()
// 	if err != nil {
// 		t.Fatalf("unable to start merchChainWatcher: %v", err)
// 	}
// 	defer merchChainWatcher.Stop()

// 	// We'll request a new channel event subscription from customer's chain
// 	// watcher.
// 	chanEvents := merchChainWatcher.SubscribeChannelEvents()

// 	var closeTxidHash chainhash.Hash
// 	err = chainhash.Decode(&closeTxidHash, custCloseTxid)
// 	if err != nil {
// 		t.Error(err)
// 	}

// 	serializedCustCTx, err := hex.DecodeString(custCloseTx)
// 	if err != nil {
// 		t.Error(err)
// 	}

// 	var msgCustCTx wire.MsgTx
// 	err = msgCustCTx.Deserialize(bytes.NewReader(serializedCustCTx))
// 	if err != nil {
// 		t.Error(err)
// 	}

// 	custSpend := &chainntnfs.SpendDetail{
// 		SpentOutPoint: fundingOut,
// 		SpenderTxHash: &closeTxidHash,
// 		SpendingTx:    &msgCustCTx,
// 	}
// 	merchNotifier.spendChan <- custSpend

// 	// We should get a new spend event over the event channel.
// 	var uniClose *ZkCustCloseInfo
// 	select {
// 	case uniClose = <-chanEvents.ZkCustClosure:
// 		t.Logf("amount: %#v\n", uniClose.amount)
// 	case <-time.After(time.Second * 5):
// 		t.Fatalf("didn't receive ZkCustClosure event")
// 	}

// 	if uniClose == nil {
// 		t.Fatalf("Did not receive ZkCustCloseInfo for remote custClose")
// 	}
// }

// TestZkChainWatcherLocalMerchClaim tests that the chain watcher is
// able to properly detect a local merchClaim tx which is spent from merchClose.
func TestZkChainWatcherLocalMerchClaim(t *testing.T) {
	// t.Parallel()

	_, merchDBPath, err := zkchannels.SetupTempDBPaths()
	if err != nil {
		t.Fatalf("SetupTempDBPaths: %v", err)
	}

	merchChainWatcher, outPoint, merchNotifier, err := setupTestMerchChainWatcher(t, true, merchDBPath)
	if err != nil {
		t.Fatalf("unable to create merchChainWatcher: %v", err)
	}
	err = merchChainWatcher.Start()
	if err != nil {
		t.Fatalf("unable to start merchChainWatcher: %v", err)
	}
	defer merchChainWatcher.Stop()

	// We'll request a new channel event subscription from merchant's chain
	// watcher.
	chanEvents := merchChainWatcher.SubscribeChannelEvents()

	var claimTxidHash chainhash.Hash
	err = chainhash.Decode(&claimTxidHash, merchClaimTxid)
	if err != nil {
		t.Error(err)
	}

	serializedMerchClaimTx, err := hex.DecodeString(merchClaimTx)
	if err != nil {
		t.Error(err)
	}

	var msgMerchClaimTx wire.MsgTx
	err = msgMerchClaimTx.Deserialize(bytes.NewReader(serializedMerchClaimTx))
	if err != nil {
		t.Error(err)
	}

	merchClaimSpend := &chainntnfs.SpendDetail{
		SpentOutPoint: outPoint,
		SpenderTxHash: &claimTxidHash,
		SpendingTx:    &msgMerchClaimTx,
	}
	merchNotifier.spendChan <- merchClaimSpend

	// We should get a new spend event for merchClaim
	var uniClose *ZkMerchClaimInfo
	select {
	case uniClose = <-chanEvents.ZkMerchClaim:
		t.Logf("ZkMerchClaimInfo: %#v", *uniClose)
	case <-time.After(time.Second * 5):
		t.Fatalf("didn't receive ZkMerchClaim event")
	}

	if uniClose == nil {
		t.Fatalf("unable to find ZkMerchClaimInfo")
	}
}

// TestZkChainWatcherRemoteMerchClaim tests that the chain watcher is
// able to properly detect a remote merchClaim tx which is spent from merchClose.
func TestZkChainWatcherRemoteMerchClaim(t *testing.T) {
	// t.Parallel()
	custDBPath, _, err := zkchannels.SetupTempDBPaths()
	if err != nil {
		t.Fatalf("SetupTempDBPaths: %v", err)
	}

	custChainWatcher, outPoint, merchNotifier, err := setupTestCustChainWatcher(t, true, custDBPath)
	if err != nil {
		t.Fatalf("unable to create custChainWatcher: %v", err)
	}
	err = custChainWatcher.Start()
	if err != nil {
		t.Fatalf("unable to start custChainWatcher: %v", err)
	}
	defer custChainWatcher.Stop()

	// We'll request a new channel event subscription from merchant's chain
	// watcher.
	chanEvents := custChainWatcher.SubscribeChannelEvents()

	var claimTxidHash chainhash.Hash
	err = chainhash.Decode(&claimTxidHash, merchClaimTxid)
	if err != nil {
		t.Error(err)
	}

	serializedMerchClaimTx, err := hex.DecodeString(merchClaimTx)
	if err != nil {
		t.Error(err)
	}

	var msgMerchClaimTx wire.MsgTx
	err = msgMerchClaimTx.Deserialize(bytes.NewReader(serializedMerchClaimTx))
	if err != nil {
		t.Error(err)
	}

	merchClaimSpend := &chainntnfs.SpendDetail{
		SpentOutPoint: outPoint,
		SpenderTxHash: &claimTxidHash,
		SpendingTx:    &msgMerchClaimTx,
	}
	merchNotifier.spendChan <- merchClaimSpend

	// We should get a new spend event for merchClaim
	var uniClose *ZkMerchClaimInfo
	select {
	case uniClose = <-chanEvents.ZkMerchClaim:
		t.Logf("ZkMerchClaimInfo: %#v", *uniClose)
	case <-time.After(time.Second * 5):
		t.Fatalf("didn't receive ZkMerchClaim event")
	}

	// The unilateral close should have properly located Alice's output in
	// the commitment transaction.
	if uniClose == nil {
		t.Fatalf("unable to find ZkMerchClaimInfo")
	}
}

// // TestZkChainWatcherCustClose tests that the chain watcher is able
// // to properly detect a local unilateral closure initiated by the customer.
// // Note that this should cover custCloseTx from Escrow and from merchClose.
// func TestZkChainWatcherLocalCustClose(t *testing.T) {
// 	t.Parallel()

// 	// Populate fields of ZkFundingInfo.
// 	var escrowTxidHash chainhash.Hash
// 	err := chainhash.Decode(&escrowTxidHash, escrowTxid)
// 	if err != nil {
// 		t.Error(err)
// 	}

// 	fundingOut := &wire.OutPoint{
// 		Hash:  escrowTxidHash,
// 		Index: uint32(0),
// 	}

// 	serializedTx, err := hex.DecodeString(escrowTx)
// 	if err != nil {
// 		t.Error(err)
// 	}

// 	var msgTx wire.MsgTx
// 	err = msgTx.Deserialize(bytes.NewReader(serializedTx))
// 	if err != nil {
// 		t.Error(err)
// 	}

// 	// in zkChannels, the funding outpoint is always the first index of the output.
// 	pkScript := msgTx.TxOut[0].PkScript

// 	ZkFundingInfo := ZkFundingInfo{
// 		FundingOut:      *fundingOut,
// 		PkScript:        pkScript,
// 		BroadcastHeight: uint32(0), // TODO ZKLND-50 Replace with actual height
// 	}

// 	// With the channels created, we'll now create a chain watcher instance
// 	// which will be watching for any closes of Alice's channel.
// 	custNotifier := &mockNotifier{
// 		spendChan: make(chan *chainntnfs.SpendDetail),
// 	}

// 	feeEstimator := zkchannels.NewMockFeeEstimator(10000, chainfee.FeePerKwFloor)
// 	custChainWatcher, err := newZkChainWatcher(ZkChainWatcherConfig{
// 		IsMerch:       false,
// 		ZkFundingInfo: ZkFundingInfo,
// 		Estimator:     feeEstimator,
// 		Notifier:      custNotifier,
// 	})
// 	if err != nil {
// 		t.Fatalf("unable to create chain watcher: %v", err)
// 	}
// 	err = custChainWatcher.Start()
// 	if err != nil {
// 		t.Fatalf("unable to start chain watcher: %v", err)
// 	}
// 	defer custChainWatcher.Stop()

// 	// We'll request a new channel event subscription from Alice's chain
// 	// watcher.
// 	chanEvents := custChainWatcher.SubscribeChannelEvents()

// 	var closeTxidHash chainhash.Hash
// 	err = chainhash.Decode(&closeTxidHash, custCloseTxid)
// 	if err != nil {
// 		t.Error(err)
// 	}

// 	serializedCustCTx, err := hex.DecodeString(custCloseTx)
// 	if err != nil {
// 		t.Error(err)
// 	}

// 	var msgCustCTx wire.MsgTx
// 	err = msgCustCTx.Deserialize(bytes.NewReader(serializedCustCTx))
// 	if err != nil {
// 		t.Error(err)
// 	}

// 	custSpend := &chainntnfs.SpendDetail{
// 		SpentOutPoint: fundingOut,
// 		SpenderTxHash: &closeTxidHash,
// 		SpendingTx:    &msgCustCTx,
// 	}
// 	custNotifier.spendChan <- custSpend

// 	// We should get a new spend event over the remote unilateral close
// 	// event channel.
// 	var uniClose *ZkCustCloseInfo
// 	select {
// 	case uniClose = <-chanEvents.ZkCustClosure:
// 		t.Logf("ZkBreachInfo: %#v", *uniClose)
// 	case <-time.After(time.Second * 5):
// 		t.Fatalf("didn't receive unilateral close event")
// 	}

// 	// The unilateral close should have properly located Alice's output in
// 	// the commitment transaction.
// 	if uniClose == nil {
// 		t.Fatalf("unable to find alice's commit resolution")
// 	}
// 	t.Log("END", uniClose)
// }
