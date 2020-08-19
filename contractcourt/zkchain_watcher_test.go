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

func setupTestMerchChainWatcher(t *testing.T, isWatchingMerchClose bool, merchDBPath string) (*zkChainWatcher, *wire.OutPoint, *mockNotifier, error) {

	escrowTx, escrowTxid, err := zkchannels.HardcodedTxs("escrow")
	if err != nil {
		t.Error(err)
	}

	var escrowTxidHash chainhash.Hash
	err = chainhash.Decode(&escrowTxidHash, escrowTxid)
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
		BroadcastHeight: uint32(0), // TODO ZKLND-50: Replace with actual fundingtx confirm height
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

	escrowTx, escrowTxid, err := zkchannels.HardcodedTxs("escrow")
	if err != nil {
		t.Error(err)
	}

	var escrowTxidHash chainhash.Hash
	err = chainhash.Decode(&escrowTxidHash, escrowTxid)
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
		BroadcastHeight: uint32(0), // TODO ZKLND-50: Replace with actual fundingtx confirm height
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

	merchCloseTx, merchCloseTxid, err := zkchannels.HardcodedTxs("merchClose")
	if err != nil {
		t.Error(err)
	}

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

	merchCloseTx, merchCloseTxid, err := zkchannels.HardcodedTxs("merchClose")
	if err != nil {
		t.Error(err)
	}

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

	revokedCustCloseTx, revokedCustCloseTxid, err := zkchannels.HardcodedTxs("revokedCustClose")
	if err != nil {
		t.Error(err)
	}

	var closeTxidHash chainhash.Hash
	err = chainhash.Decode(&closeTxidHash, revokedCustCloseTxid)
	if err != nil {
		t.Error(err)
	}

	serializedCustCTx, err := hex.DecodeString(revokedCustCloseTx)
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

// TestZkChainWatcherRemoteValidCustClose tests that the chain watcher is able
// to properly detect a remote closure initiated by the customer.
// 'Valid' custClose refers to the fact that it is the latest state, i.e. the
// customer is not closing with a revoked state.
// Note that this is used in both custCloseTx from Escrow and from merchClose.
func TestZkChainWatcherRemoteValidCustClose(t *testing.T) {
	custDBPath, merchDBPath, err := zkchannels.SetupTestZkDBs()
	if err != nil {
		t.Fatalf("SetupTestZkDBs: %v", err)
	}
	defer zkchannels.TearDownZkDBs(custDBPath, merchDBPath)

	// This wont work until the zkmerch.db and zkcust.db have been setup
	zkchannels.SetupLibzkChannels("myChannel", custDBPath, merchDBPath)

	merchChainWatcher, fundingOut, merchNotifier, err := setupTestMerchChainWatcher(t, false, merchDBPath)

	if err != nil {
		t.Fatalf("unable to create merchChainWatcher: %v", err)
	}
	err = merchChainWatcher.Start()
	if err != nil {
		t.Fatalf("unable to start merchChainWatcher: %v", err)
	}
	defer merchChainWatcher.Stop()

	// We'll request a new channel event subscription from customer's chain
	// watcher.
	chanEvents := merchChainWatcher.SubscribeChannelEvents()

	latestCustCloseTx, latestCustCloseTxid, err := zkchannels.HardcodedTxs("latestCustClose")
	if err != nil {
		t.Error(err)
	}

	var closeTxidHash chainhash.Hash
	err = chainhash.Decode(&closeTxidHash, latestCustCloseTxid)
	if err != nil {
		t.Error(err)
	}

	serializedCustCTx, err := hex.DecodeString(latestCustCloseTx)
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
	merchNotifier.spendChan <- custSpend

	// We should get a new spend event over the event channel.
	var uniClose *ZkCustCloseInfo
	select {
	case uniClose = <-chanEvents.ZkCustClosure:
		t.Logf("amount: %#v\n", uniClose.amount)
	case <-time.After(time.Second * 5):
		t.Fatalf("didn't receive ZkCustClosure event")
	}

	if uniClose == nil {
		t.Fatalf("Did not receive ZkCustCloseInfo for remote custClose")
	}
}

// TestZkChainWatcherRemoteRevokedCustClose tests that the chain watcher is able
// to properly detect a remote closure initiated by the customer.
// 'Revoked' custClose refers to the fact that it is not the latest state, i.e.
// the customer is closing with a revoked state.
// Note that this is used in both custCloseTx from Escrow and from merchClose.
func TestZkChainWatcherRemoteRevokedCustClose(t *testing.T) {
	custDBPath, merchDBPath, err := zkchannels.SetupTestZkDBs()
	if err != nil {
		t.Fatalf("SetupTestZkDBs: %v", err)
	}
	defer zkchannels.TearDownZkDBs(custDBPath, merchDBPath)

	// This wont work until the zkmerch.db and zkcust.db have been setup
	zkchannels.SetupLibzkChannels("myChannel", custDBPath, merchDBPath)

	merchChainWatcher, fundingOut, merchNotifier, err := setupTestMerchChainWatcher(t, false, merchDBPath)

	if err != nil {
		t.Fatalf("unable to create merchChainWatcher: %v", err)
	}
	err = merchChainWatcher.Start()
	if err != nil {
		t.Fatalf("unable to start merchChainWatcher: %v", err)
	}
	defer merchChainWatcher.Stop()

	// We'll request a new channel event subscription from customer's chain
	// watcher.
	chanEvents := merchChainWatcher.SubscribeChannelEvents()

	revokedCustCloseTx, revokedCustCloseTxid, err := zkchannels.HardcodedTxs("revokedCustClose")
	if err != nil {
		t.Error(err)
	}

	var closeTxidHash chainhash.Hash
	err = chainhash.Decode(&closeTxidHash, revokedCustCloseTxid)
	if err != nil {
		t.Error(err)
	}

	serializedCustCTx, err := hex.DecodeString(revokedCustCloseTx)
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
	merchNotifier.spendChan <- custSpend

	// We should get a new spend event over the remote merchClose
	// event channel.
	var uniClose *ZkBreachInfo
	select {
	case uniClose = <-chanEvents.ZkContractBreach:
		t.Logf("amount: %#v\n", uniClose.Amount)
	case <-time.After(time.Second * 5):
		t.Fatalf("didn't receive ZkContractBreach event for revoked custClose")
	}
	if uniClose == nil {
		t.Fatalf("Did not receive ZkBreachInfo for revoked custClose")
	}
}

// TestZkChainWatcherLocalMerchClaim tests that the chain watcher is
// able to properly detect a local merchClaim tx which is spent from merchClose.
func TestZkChainWatcherLocalMerchClaim(t *testing.T) {
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

	merchClaimTx, merchClaimTxid, err := zkchannels.HardcodedTxs("merchClaim")
	if err != nil {
		t.Error(err)
	}

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

	merchClaimTx, merchClaimTxid, err := zkchannels.HardcodedTxs("merchClaim")
	if err != nil {
		t.Error(err)
	}

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
