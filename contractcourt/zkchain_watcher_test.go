package contractcourt

import (
	"bytes"
	"encoding/hex"
	"testing"
	"time"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/lightningnetwork/lnd/chainntnfs"
)

// zkchannel test txs
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

// TestChainWatcherMerchClose tests that the chain watcher is able
// to properly detect a unilateral closure initiated by the merchant.
func TestZkChainWatcherMerchClose(t *testing.T) {
	t.Parallel()

	// Populate fields of ZkFundingInfo.
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

	ZkFundingInfo := ZkFundingInfo{
		FundingOut:      *fundingOut,
		PkScript:        pkScript,
		BroadcastHeight: uint32(0), // TODO: Replace with actual fundingtx confirm height
	}

	// With the channels created, we'll now create a chain watcher instance
	// which will be watching for any closes of Alice's channel.
	custNotifier := &mockNotifier{
		spendChan: make(chan *chainntnfs.SpendDetail),
	}

	custChainWatcher, err := newZkChainWatcher(ZkChainWatcherConfig{
		IsMerch:       true,
		ZkFundingInfo: ZkFundingInfo,
		Notifier:      custNotifier,
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
	chainhash.Decode(&merchCloseTxidHash, merchCloseTxid)

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

	// We should get a new spend event over the remote unilateral close
	// event channel.
	var uniClose *ZkMerchCloseInfo
	select {
	case uniClose = <-chanEvents.ZkMerchClosure:
		t.Logf("amount: %#v\n", uniClose.amount)
	case <-time.After(time.Second * 5):
		t.Fatalf("didn't receive unilateral close event")
	}

	// The unilateral close should have properly located Alice's output in
	// the commitment transaction.
	if uniClose == nil {
		t.Fatalf("unable to find alice's commit resolution")
	}
}

// TestChainWatcherCustClose tests that the chain watcher is able
// to properly detect a unilateral closure initiated by the customer.
func TestZkChainWatcherCustClose(t *testing.T) {
	t.Parallel()

	// Populate fields of ZkFundingInfo.
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

	ZkFundingInfo := ZkFundingInfo{
		FundingOut:      *fundingOut,
		PkScript:        pkScript,
		BroadcastHeight: uint32(0), // TODO: Replace with actual fundingtx confirm height
	}

	// With the channels created, we'll now create a chain watcher instance
	// which will be watching for any closes of Alice's channel.
	custNotifier := &mockNotifier{
		spendChan: make(chan *chainntnfs.SpendDetail),
	}

	custChainWatcher, err := newZkChainWatcher(ZkChainWatcherConfig{
		IsMerch:       true,
		ZkFundingInfo: ZkFundingInfo,
		Notifier:      custNotifier,
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

	var custCloseTxidHash chainhash.Hash
	chainhash.Decode(&custCloseTxidHash, custCloseTxid)

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
		SpenderTxHash: &custCloseTxidHash,
		SpendingTx:    &msgCustCTx,
	}
	custNotifier.spendChan <- custSpend

	// We should get a new spend event over the remote unilateral close
	// event channel.
	var uniClose *ZkCustCloseInfo
	select {
	case uniClose = <-chanEvents.ZkCustClosure:
		// t.Logf("ZkCustBreachInfo: %#v", *uniClose)
	case <-time.After(time.Second * 5):
		t.Fatalf("didn't receive unilateral close event")
	}

	// The unilateral close should have properly located Alice's output in
	// the commitment transaction.
	if uniClose == nil {
		t.Fatalf("unable to find alice's commit resolution")
	}
}
