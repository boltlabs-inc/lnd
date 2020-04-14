package contractcourt

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"github.com/davecgh/go-spew/spew"
	"github.com/lightningnetwork/lnd/chainntnfs"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/input"
	"github.com/lightningnetwork/lnd/libzkchannels"
	"github.com/lightningnetwork/lnd/lnwallet"
	"github.com/lightningnetwork/lnd/zkchanneldb"
)

// const (
// 	// minCommitPointPollTimeout is the minimum time we'll wait before
// 	// polling the database for a channel's commitpoint.
// 	minCommitPointPollTimeout = 1 * time.Second

// 	// maxCommitPointPollTimeout is the maximum time we'll wait before
// 	// polling the database for a channel's commitpoint.
// 	maxCommitPointPollTimeout = 10 * time.Minute
// )

// // LocalUnilateralCloseInfo encapsulates all the information we need to act on
// // a local force close that gets confirmed.
// type LocalUnilateralCloseInfo struct {
// 	*chainntnfs.SpendDetail
// 	*lnwallet.LocalForceCloseSummary
// 	*channeldb.ChannelCloseSummary

// 	// CommitSet is the set of known valid commitments at the time the
// 	// remote party's commitment hit the chain.
// 	CommitSet CommitSet
// }

// // CooperativeCloseInfo encapsulates all the information we need to act on a
// // cooperative close that gets confirmed.
// type CooperativeCloseInfo struct {
// 	*channeldb.ChannelCloseSummary
// }

// ZkMerchCloseInfo provides the information needed for the customer to retrieve
// the relevant CloseMerch transaction for that channel.
type ZkMerchCloseInfo struct {
	escrowTxid     chainhash.Hash
	merchCloseTxid chainhash.Hash
	pkScript       []byte
	amount         int64
}

// ZkCustCloseInfo provides details about the customer close tx.
type ZkCustCloseInfo struct {
	escrowTxid  chainhash.Hash
	closeTxid   chainhash.Hash
	pkScript    []byte
	revLock     string
	custClosePk string
	amount      int64
}

// ZkBreachInfo provides information needed to create a dispute transaction.
type ZkBreachInfo struct {
	IsMerchClose    bool
	EscrowTxid      chainhash.Hash
	CloseTxid       chainhash.Hash
	ClosePkScript   []byte
	CustChannelName string
	RevLock         string
	RevSecret       string
	CustClosePk     string
	Amount          int64
}

// // RemoteUnilateralCloseInfo wraps the normal UnilateralCloseSummary to couple
// // the CommitSet at the time of channel closure.
// type RemoteUnilateralCloseInfo struct {
// 	*lnwallet.UnilateralCloseSummary

// 	// CommitSet is the set of known valid commitments at the time the
// 	// remote party's commitment hit the chain.
// 	CommitSet CommitSet
// }

// // CommitSet is a collection of the set of known valid commitments at a given
// // instant. If ConfCommitKey is set, then the commitment identified by the
// // HtlcSetKey has hit the chain. This struct will be used to examine all live
// // HTLCs to determine if any additional actions need to be made based on the
// // remote party's commitments.
// type CommitSet struct {
// 	// ConfCommitKey if non-nil, identifies the commitment that was
// 	// confirmed in the chain.
// 	ConfCommitKey *HtlcSetKey

// 	// HtlcSets stores the set of all known active HTLC for each active
// 	// commitment at the time of channel closure.
// 	HtlcSets map[HtlcSetKey][]channeldb.HTLC
// }

// // IsEmpty returns true if there are no HTLCs at all within all commitments
// // that are a part of this commitment diff.
// func (c *CommitSet) IsEmpty() bool {
// 	if c == nil {
// 		return true
// 	}

// 	for _, htlcs := range c.HtlcSets {
// 		if len(htlcs) != 0 {
// 			return false
// 		}
// 	}

// 	return true
// }

// // toActiveHTLCSets returns the set of all active HTLCs across all commitment
// // transactions.
// func (c *CommitSet) toActiveHTLCSets() map[HtlcSetKey]htlcSet {
// 	htlcSets := make(map[HtlcSetKey]htlcSet)

// 	for htlcSetKey, htlcs := range c.HtlcSets {
// 		htlcSets[htlcSetKey] = newHtlcSet(htlcs)
// 	}

// 	return htlcSets
// }

// ZkChainEventSubscription is a struct that houses a subscription to be notified
// for any on-chain events related to a channel. There are three types of
// possible on-chain events: a cooperative channel closure, a unilateral
// channel closure, and a channel breach. The fourth type: a force close is
// locally initiated, so we don't provide any event stream for said event.
type ZkChainEventSubscription struct {
	// ChanPoint is that channel that chain events will be dispatched for.
	ChanPoint wire.OutPoint

	// RemoteUnilateralClosure is a channel that will be sent upon in the
	// event that the remote party's commitment transaction is confirmed.
	RemoteUnilateralClosure chan *RemoteUnilateralCloseInfo

	// ZkMerchClosure is a channel that will be sent upon in the
	// event that the Merchant's close transaction is confirmed.
	ZkMerchClosure chan *ZkMerchCloseInfo

	// ZkCustClosure is a channel that will be sent upon in the
	// event that the Customer's close transaction is confirmed.
	ZkCustClosure chan *ZkCustCloseInfo

	// ZkContractBreach is a channel that will be sent upon in the
	// event that the Customer's close transaction is confirmed.
	ZkContractBreach chan *ZkBreachInfo

	// LocalUnilateralClosure is a channel that will be sent upon in the
	// event that our commitment transaction is confirmed.
	LocalUnilateralClosure chan *LocalUnilateralCloseInfo

	// CooperativeClosure is a signal that will be sent upon once a
	// cooperative channel closure has been detected confirmed.
	CooperativeClosure chan *CooperativeCloseInfo

	// ContractBreach is a channel that will be sent upon if we detect a
	// contract breach. The struct sent across the channel contains all the
	// material required to bring the cheating channel peer to justice.
	ContractBreach chan *lnwallet.BreachRetribution

	// Cancel cancels the subscription to the event stream for a particular
	// channel. This method should be called once the caller no longer needs to
	// be notified of any on-chain events for a particular channel.
	Cancel func()
}

// ZkChainWatcherConfig encapsulates all the necessary functions and interfaces
// needed to watch and act on on-chain events for a particular channel.
type ZkChainWatcherConfig struct {
	// ZkFundingInfo contains funding outpoint, pkscript and, confirmation blockheight
	ZkFundingInfo

	// isMerch is true if this node is the merchant's
	IsMerch bool

	// CustChannelName is the name the customer gives to the channel.
	// For the merchant, this field will be left blank
	CustChannelName string

	// chanState is a snapshot of the persistent state of the channel that
	// we're watching. In the event of an on-chain event, we'll query the
	// database to ensure that we act using the most up to date state.
	chanState *channeldb.OpenChannel

	// notifier is a reference to the channel notifier that we'll use to be
	// notified of output spends and when transactions are confirmed.
	Notifier chainntnfs.ChainNotifier

	// signer is the main signer instances that will be responsible for
	// signing any HTLC and commitment transaction generated by the state
	// machine.
	signer input.Signer

	// contractBreach is a method that will be called by the watcher if it
	// detects that a contract breach transaction has been confirmed. Only
	// when this method returns with a non-nil error it will be safe to mark
	// the channel as pending close in the database.
	contractBreach func(*lnwallet.BreachRetribution) error

	// contractBreach is a method that will be called by the watcher if it
	// detects that a contract breach transaction has been confirmed. Only
	// when this method returns with a non-nil error it will be safe to mark
	// the channel as pending close in the database.
	zkContractBreach func(*ZkBreachInfo) error

	// isOurAddr is a function that returns true if the passed address is
	// known to us.
	isOurAddr func(btcutil.Address) bool

	// extractStateNumHint extracts the encoded state hint using the passed
	// obfuscater. This is used by the chain watcher to identify which
	// state was broadcast and confirmed on-chain.
	extractStateNumHint func(*wire.MsgTx, [lnwallet.StateHintSize]byte) uint64
}

type ZkFundingInfo struct {
	FundingOut      wire.OutPoint
	PkScript        []byte
	BroadcastHeight uint32
}

// zkChainWatcher is a system that's assigned to every active channel. The duty
// of this system is to watch the chain for spends of the channels chan point.
// If a spend is detected then with chain watcher will notify all subscribers
// that the channel has been closed, and also give them the materials necessary
// to sweep the funds of the channel on chain eventually.
type zkChainWatcher struct {
	started int32 // To be used atomically.
	stopped int32 // To be used atomically.

	quit chan struct{}
	wg   sync.WaitGroup

	cfg ZkChainWatcherConfig

	// stateHintObfuscator is a 48-bit state hint that's used to obfuscate
	// the current state number on the commitment transactions.
	stateHintObfuscator [lnwallet.StateHintSize]byte

	// All the fields below are protected by this mutex.
	sync.Mutex

	// clientID is an ephemeral counter used to keep track of each
	// individual client subscription.
	clientID uint64

	// clientSubscriptions is a map that keeps track of all the active
	// client subscriptions for events related to this channel.
	clientSubscriptions map[uint64]*ZkChainEventSubscription
}

// newZkChainWatcher returns a new instance of a zkChainWatcher for a channel given
// the chan point to watch, and also a notifier instance that will allow us to
// detect on chain events.
func newZkChainWatcher(cfg ZkChainWatcherConfig) (*zkChainWatcher, error) {

	fmt.Println("\nSetting up new zkChainWatcher\n")
	return &zkChainWatcher{
		cfg:                 cfg,
		quit:                make(chan struct{}),
		clientSubscriptions: make(map[uint64]*ZkChainEventSubscription),
	}, nil
}

// Start starts all goroutines that the zkChainWatcher needs to perform its
// duties.
func (c *zkChainWatcher) Start() error {
	fmt.Println("Starting new zkChainWatcher")

	// // zkch TODO: What does this do? It is blocking
	// if !atomic.CompareAndSwapInt32(&c.started, 0, 1) {
	// 	return nil
	// }

	fmt.Printf("\nwatching for escrow: %#v\n", c.cfg.ZkFundingInfo.FundingOut.Hash.String())

	spendNtfn, err := c.cfg.Notifier.RegisterSpendNtfn(
		&c.cfg.ZkFundingInfo.FundingOut,
		c.cfg.ZkFundingInfo.PkScript,
		c.cfg.ZkFundingInfo.BroadcastHeight,
	)
	if err != nil {
		fmt.Printf("\n\n zkchainwatcher err:", err)
		return err
	}
	fmt.Printf("\nspend notifier finished\n")

	// With the spend notification obtained, we'll now dispatch the
	// closeObserver which will properly react to any changes.
	c.wg.Add(1)
	go c.zkCloseObserver(spendNtfn)

	return nil
}

// Stop signals the close observer to gracefully exit.
func (c *zkChainWatcher) Stop() error {
	if !atomic.CompareAndSwapInt32(&c.stopped, 0, 1) {
		return nil
	}

	close(c.quit)

	c.wg.Wait()

	return nil
}

// SubscribeChannelEvents returns an active subscription to the set of channel
// events for the channel watched by this chain watcher. Once clients no longer
// require the subscription, they should call the Cancel() method to allow the
// watcher to regain those committed resources.
func (c *zkChainWatcher) SubscribeChannelEvents() *ZkChainEventSubscription {

	c.Lock()
	clientID := c.clientID
	c.clientID++
	c.Unlock()

	sub := &ZkChainEventSubscription{
		ChanPoint:               c.cfg.ZkFundingInfo.FundingOut,
		RemoteUnilateralClosure: make(chan *RemoteUnilateralCloseInfo, 1),
		ZkMerchClosure:          make(chan *ZkMerchCloseInfo, 1),
		ZkCustClosure:           make(chan *ZkCustCloseInfo, 1),
		ZkContractBreach:        make(chan *ZkBreachInfo, 1),
		LocalUnilateralClosure:  make(chan *LocalUnilateralCloseInfo, 1),
		CooperativeClosure:      make(chan *CooperativeCloseInfo, 1),
		ContractBreach:          make(chan *lnwallet.BreachRetribution, 1),
		Cancel: func() {
			c.Lock()
			delete(c.clientSubscriptions, clientID)
			c.Unlock()
			return
		},
	}

	c.Lock()
	c.clientSubscriptions[clientID] = sub
	c.Unlock()

	return sub
}

// // isOurCommitment returns true if the passed commitSpend is a spend of the
// // funding transaction using our commitment transaction (a local force close).
// // In order to do this in a state agnostic manner, we'll make our decisions
// // based off of only the set of outputs included.
// func isOurCommitment(localChanCfg, remoteChanCfg channeldb.ChannelConfig,
// 	commitSpend *chainntnfs.SpendDetail, broadcastStateNum uint64,
// 	revocationProducer shachain.Producer,
// 	chanType channeldb.ChannelType) (bool, error) {

// 	// First, we'll re-derive our commitment point for this state since
// 	// this is what we use to randomize each of the keys for this state.
// 	commitSecret, err := revocationProducer.AtIndex(broadcastStateNum)
// 	if err != nil {
// 		return false, err
// 	}
// 	commitPoint := input.ComputeCommitmentPoint(commitSecret[:])

// 	// Now that we have the commit point, we'll derive the tweaked local
// 	// and remote keys for this state. We use our point as only we can
// 	// revoke our own commitment.
// 	commitKeyRing := lnwallet.DeriveCommitmentKeys(
// 		commitPoint, true, chanType, &localChanCfg, &remoteChanCfg,
// 	)

// 	// With the keys derived, we'll construct the remote script that'll be
// 	// present if they have a non-dust balance on the commitment.
// 	remoteDelay := uint32(remoteChanCfg.CsvDelay)
// 	remoteScript, err := lnwallet.CommitScriptToRemote(
// 		chanType, remoteDelay, commitKeyRing.ToRemoteKey,
// 	)
// 	if err != nil {
// 		return false, err
// 	}

// 	// Next, we'll derive our script that includes the revocation base for
// 	// the remote party allowing them to claim this output before the CSV
// 	// delay if we breach.
// 	localScript, err := input.CommitScriptToSelf(
// 		uint32(localChanCfg.CsvDelay), commitKeyRing.ToLocalKey,
// 		commitKeyRing.RevocationKey,
// 	)
// 	if err != nil {
// 		return false, err
// 	}
// 	localPkScript, err := input.WitnessScriptHash(localScript)
// 	if err != nil {
// 		return false, err
// 	}

// 	// With all our scripts assembled, we'll examine the outputs of the
// 	// commitment transaction to determine if this is a local force close
// 	// or not.
// 	for _, output := range commitSpend.SpendingTx.TxOut {
// 		pkScript := output.PkScript

// 		switch {
// 		case bytes.Equal(localPkScript, pkScript):
// 			return true, nil

// 		case bytes.Equal(remoteScript.PkScript, pkScript):
// 			return true, nil
// 		}
// 	}

// 	// If neither of these scripts are present, then it isn't a local force
// 	// close.
// 	return false, nil
// }

// zkCloseObserver is a dedicated goroutine that will watch for any closes of the
// channel that it's watching on chain. In the event of an on-chain event, the
// close observer will assembled the proper materials required to claim the
// funds of the channel on-chain (if required), then dispatch these as
// notifications to all subscribers.
func (c *zkChainWatcher) zkCloseObserver(spendNtfn *chainntnfs.SpendEvent) {
	defer c.wg.Done()

	fmt.Printf("zkCloseObserver is running\n")
	// determine if this node is a merchant
	isMerch := c.cfg.IsMerch

	select {
	// We've detected a spend of the channel onchain! Depending on the type
	// of spend, we'll act accordingly , so we'll examine the spending
	// transaction to determine what we should do.

	case commitSpend, ok := <-spendNtfn.Spend:
		// If the channel was closed, then this means that the notifier
		// exited, so we will as well.
		if !ok {
			return
		}

		log.Debug("Spend from escrow detected")

		escrowTxid := commitSpend.SpentOutPoint.Hash
		commitTxBroadcast := commitSpend.SpendingTx
		closeTxid := *commitSpend.SpenderTxHash
		spendHeight := commitSpend.SpendingHeight

		numOutputs := len(commitTxBroadcast.TxOut)

		amount := commitTxBroadcast.TxOut[0].Value

		switch {

		// merchCloseTx has one output.
		case numOutputs < 2 && isMerch:
			log.Debug("Merch close detected")

			pkScript := commitTxBroadcast.TxOut[0].PkScript

			err := c.zkDispatchMerchClose(escrowTxid, closeTxid, pkScript, amount)

			if err != nil {
				log.Errorf("unable to handle remote "+
					"close for channel=%v",
					escrowTxid, err)
			}

		// merchCloseTx has one output.
		case numOutputs < 2 && !isMerch:
			fmt.Printf("Merch close detected\n")

			pkScript := commitTxBroadcast.TxOut[0].PkScript

			zkBreachInfo := ZkBreachInfo{
				IsMerchClose:    true,
				EscrowTxid:      escrowTxid,
				CloseTxid:       closeTxid,
				ClosePkScript:   pkScript,
				CustChannelName: c.cfg.CustChannelName,
				// RevLock:       revLock,
				// RevSecret:     revSecret,
				// CustClosePk:   custClosePk,
				Amount: amount,
			}

			err := c.zkDispatchBreach(zkBreachInfo)

			if err != nil {
				log.Errorf("unable to handle remote "+
					"close for channel=%v",
					escrowTxid, err)
			}

		// custCloseTx has 3 outputs.
		case numOutputs > 2:

			ClosePkScript := commitTxBroadcast.TxOut[0].PkScript

			opreturnScript := commitTxBroadcast.TxOut[2].PkScript
			revLockBytes := opreturnScript[2:34]
			revLock := hex.EncodeToString(revLockBytes)

			custClosePkBytes := opreturnScript[34:67]
			custClosePk := hex.EncodeToString(custClosePkBytes)

			// If the user is a customer and they broadcasted custClose,
			// there is nothing more to do.
			if !isMerch {

				log.Debug("signCustClaimTx")
				// save custClose details so that it can be claimed manually with cli command
				err := c.storeCustClaimTx(escrowTxid.String(), closeTxid.String(), ClosePkScript, revLock, custClosePk, amount, spendHeight)
				if err != nil {
					log.Errorf("Unable to store CustClaimTx for channel %v ",
						escrowTxid, err)
				}

				log.Debug("zkDispatchCustClose")
				err = c.zkDispatchCustClose(escrowTxid, closeTxid, ClosePkScript, revLock, custClosePk, amount)
				// custClose is not the latest state. The customer has attempted to
				// close on a previous state, possibly a double spend.

				if err != nil {
					log.Errorf("unable to handle remote "+
						"custClose for channel=%v",
						escrowTxid, err)
				}
			}
			// if the closeTx has more than 2 outputs, it is a custCloseTx. Now we
			// check to see if custCloseTx corresponds to a revoked state by
			// checking if we have seen the revocation lock before.
			if isMerch {

				// open the zkchanneldb to load merchState and channelState
				zkMerchDB, err := zkchanneldb.SetupZkMerchDB()
				var merchState libzkchannels.MerchState
				merchStateBytes, err := zkchanneldb.GetMerchState(zkMerchDB)
				err = json.Unmarshal(merchStateBytes, &merchState)
				zkMerchDB.Close()

				isOldRevLock, revSecret, err := libzkchannels.MerchantCheckRevLock(revLock, merchState)

				// The Revocation Lock in the custCloseTx corresponds to an old
				// state. We must send the relevant transaction information
				// to the breachArbiter to broadcast the dispute/justice tx.
				if isOldRevLock {
					fmt.Printf("Revoked Cust close detected\n")

					zkBreachInfo := ZkBreachInfo{
						IsMerchClose:  false,
						EscrowTxid:    escrowTxid,
						CloseTxid:     closeTxid,
						ClosePkScript: ClosePkScript,
						RevLock:       revLock,
						RevSecret:     revSecret,
						CustClosePk:   custClosePk,
						Amount:        amount,
					}

					err := c.zkDispatchBreach(zkBreachInfo)

					if err != nil {
						log.Errorf("unable to handle remote breach"+
							" custClose for channel=%v",
							escrowTxid, err)
					}
					// The Revocation Lock has not been seen before, therefore it
					// corresponds to the customer's latest state. We should
					// store the Revocation Lock to prevent a customer making
					// a payment with it.
				} else {
					fmt.Printf("Latest Cust close detected\n")

					// custClose is not the latest state. The customer has attempted to
					// close on a previous state, possibly a double spend.
					err := c.zkDispatchCustClose(escrowTxid, closeTxid, ClosePkScript, revLock, custClosePk, amount)

					if err != nil {
						log.Errorf("unable to handle remote "+
							"custClose for channel=%v",
							escrowTxid, err)
					}
				}
				if err != nil {
					log.Errorf("unable to handle remote "+
						"close for channel=%v",
						escrowTxid, err)
				}

			}
		}

		// Now that a spend has been detected, we've done our job, so
		// we'll exit immediately.
		return

	// The zkChainWatcher has been signalled to exit, so we'll do so now.
	case <-c.quit:
		return
	}
}

// storeCustClaimTx creates a signed custClaimTx for the given closeTx
func (c *zkChainWatcher) storeCustClaimTx(escrowTxidLittleEn string, closeTxid string, closePkScript []byte,
	revLock string, custClosePk string, amount int64, spendHeight int32) error {

	// Start watching the channel in order to respond to breach Txs
	// TEMPORARY CODE TO FLIP BYTES
	escrowTxidBigEn := ""
	for i := 0; i < len(escrowTxidLittleEn)/2; i++ {
		escrowTxidBigEn = escrowTxidLittleEn[i*2:i*2+2] + escrowTxidBigEn
	}

	log.Debugf("storeCustClaimTx inputs: ", escrowTxidBigEn, closeTxid, closePkScript,
		revLock, custClosePk, amount, spendHeight)

	channelName := c.cfg.CustChannelName

	// Load the current custState and channelState so that it can be retrieved
	// later when it is needed to sign the claim tx.
	zkCustDB, err := zkchanneldb.OpenZkChannelBucket(channelName)
	if err != nil {
		log.Error(err)
		return err
	}

	var custState libzkchannels.CustState
	custStateBytes, err := zkchanneldb.GetCustState(zkCustDB, channelName)
	if err != err {
		log.Error(err)
		return err
	}

	err = json.Unmarshal(custStateBytes, &custState)
	if err != err {
		log.Error(err)
		return err
	}

	var channelState libzkchannels.ChannelState
	channelStateBytes, err := zkchanneldb.GetCustField(zkCustDB, channelName, "channelStateKey")
	if err != err {
		log.Error(err)
		return err
	}
	err = json.Unmarshal(channelStateBytes, &channelState)
	if err != nil {
		log.Error(err)
		return err
	}

	err = zkCustDB.Close()

	toSelfDelay, err := libzkchannels.GetSelfDelayBE(channelState)

	// TODO: Generate a fresh outputPk for the claimed outputs. For now this is just
	// reusing the custClosePk
	outputPk := custClosePk

	signedCustClaimTx, err := libzkchannels.CustomerSignClaimTx(channelState, closeTxid, uint32(0), custState.CustBalance, toSelfDelay, outputPk, custState.RevLock, custClosePk, custState)
	if err != nil {
		log.Error(err)
		return err
	}
	log.Debugf("signedCustClaimTx: %#v", signedCustClaimTx)

	// open the zkchanneldb to load custState.
	// use escrowTxid as the bucket name
	bucketEscrowTxid := escrowTxidBigEn

	zkCustClaimDB, err := zkchanneldb.OpenZkClaimBucket(bucketEscrowTxid)
	if err != nil {
		log.Error(err)
		return err
	}
	signedCustClaimTxBytes, err := json.Marshal(signedCustClaimTx)
	if err != nil {
		log.Error(err)
		return err
	}
	err = zkchanneldb.AddCustField(zkCustClaimDB, bucketEscrowTxid, signedCustClaimTxBytes, "signedCustClaimTxKey")
	if err != nil {
		log.Error(err)
		return err
	}
	spendHeightBytes, err := json.Marshal(spendHeight)
	if err != nil {
		log.Error(err)
		return err
	}
	err = zkchanneldb.AddCustField(zkCustClaimDB, bucketEscrowTxid, spendHeightBytes, "spendHeightKey")
	if err != nil {
		log.Error(err)
		return err
	}

	zkCustClaimDB.Close()

	return nil
}

// toSelfAmount takes a transaction and returns the sum of all outputs that pay
// to a script that the wallet controls. If no outputs pay to us, then we
// return zero. This is possible as our output may have been trimmed due to
// being dust.
func (c *zkChainWatcher) toSelfAmount(tx *wire.MsgTx) btcutil.Amount {
	var selfAmt btcutil.Amount
	for _, txOut := range tx.TxOut {
		_, addrs, _, err := txscript.ExtractPkScriptAddrs(
			// Doesn't matter what net we actually pass in.
			txOut.PkScript, &chaincfg.TestNet3Params,
		)
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if c.cfg.isOurAddr(addr) {
				selfAmt += btcutil.Amount(txOut.Value)
			}
		}
	}

	return selfAmt
}

// dispatchCooperativeClose processed a detect cooperative channel closure.
// We'll use the spending transaction to locate our output within the
// transaction, then clean up the database state. We'll also dispatch a
// notification to all subscribers that the channel has been closed in this
// manner.
func (c *zkChainWatcher) dispatchCooperativeClose(commitSpend *chainntnfs.SpendDetail) error {
	broadcastTx := commitSpend.SpendingTx

	log.Infof("Cooperative closure for ChannelPoint(%v): %v",
		c.cfg.chanState.FundingOutpoint, spew.Sdump(broadcastTx))

	// If the input *is* final, then we'll check to see which output is
	// ours.
	localAmt := c.toSelfAmount(broadcastTx)

	// Once this is known, we'll mark the state as fully closed in the
	// database. We can do this as a cooperatively closed channel has all
	// its outputs resolved after only one confirmation.
	closeSummary := &channeldb.ChannelCloseSummary{
		ChanPoint:               c.cfg.chanState.FundingOutpoint,
		ChainHash:               c.cfg.chanState.ChainHash,
		ClosingTXID:             *commitSpend.SpenderTxHash,
		RemotePub:               c.cfg.chanState.IdentityPub,
		Capacity:                c.cfg.chanState.Capacity,
		CloseHeight:             uint32(commitSpend.SpendingHeight),
		SettledBalance:          localAmt,
		CloseType:               channeldb.CooperativeClose,
		ShortChanID:             c.cfg.chanState.ShortChanID(),
		IsPending:               true,
		RemoteCurrentRevocation: c.cfg.chanState.RemoteCurrentRevocation,
		RemoteNextRevocation:    c.cfg.chanState.RemoteNextRevocation,
		LocalChanConfig:         c.cfg.chanState.LocalChanCfg,
	}

	// Attempt to add a channel sync message to the close summary.
	chanSync, err := c.cfg.chanState.ChanSyncMsg()
	if err != nil {
		log.Errorf("ChannelPoint(%v): unable to create channel sync "+
			"message: %v", c.cfg.chanState.FundingOutpoint, err)
	} else {
		closeSummary.LastChanSyncMsg = chanSync
	}

	// Create a summary of all the information needed to handle the
	// cooperative closure.
	closeInfo := &CooperativeCloseInfo{
		ChannelCloseSummary: closeSummary,
	}

	// With the event processed, we'll now notify all subscribers of the
	// event.
	c.Lock()
	for _, sub := range c.clientSubscriptions {
		select {
		case sub.CooperativeClosure <- closeInfo:
		case <-c.quit:
			c.Unlock()
			return fmt.Errorf("exiting")
		}
	}
	c.Unlock()

	return nil
}

// dispatchLocalForceClose processes a unilateral close by us being confirmed.
func (c *zkChainWatcher) dispatchLocalForceClose(
	commitSpend *chainntnfs.SpendDetail,
	localCommit channeldb.ChannelCommitment, commitSet CommitSet) error {

	log.Infof("Local unilateral close of ChannelPoint(%v) "+
		"detected", c.cfg.chanState.FundingOutpoint)

	forceClose, err := lnwallet.NewLocalForceCloseSummary(
		c.cfg.chanState, c.cfg.signer,
		commitSpend.SpendingTx, localCommit,
	)
	if err != nil {
		return err
	}

	// As we've detected that the channel has been closed, immediately
	// creating a close summary for future usage by related sub-systems.
	chanSnapshot := forceClose.ChanSnapshot
	closeSummary := &channeldb.ChannelCloseSummary{
		ChanPoint:               chanSnapshot.ChannelPoint,
		ChainHash:               chanSnapshot.ChainHash,
		ClosingTXID:             forceClose.CloseTx.TxHash(),
		RemotePub:               &chanSnapshot.RemoteIdentity,
		Capacity:                chanSnapshot.Capacity,
		CloseType:               channeldb.LocalForceClose,
		IsPending:               true,
		ShortChanID:             c.cfg.chanState.ShortChanID(),
		CloseHeight:             uint32(commitSpend.SpendingHeight),
		RemoteCurrentRevocation: c.cfg.chanState.RemoteCurrentRevocation,
		RemoteNextRevocation:    c.cfg.chanState.RemoteNextRevocation,
		LocalChanConfig:         c.cfg.chanState.LocalChanCfg,
	}

	// If our commitment output isn't dust or we have active HTLC's on the
	// commitment transaction, then we'll populate the balances on the
	// close channel summary.
	if forceClose.CommitResolution != nil {
		closeSummary.SettledBalance = chanSnapshot.LocalBalance.ToSatoshis()
		closeSummary.TimeLockedBalance = chanSnapshot.LocalBalance.ToSatoshis()
	}
	for _, htlc := range forceClose.HtlcResolutions.OutgoingHTLCs {
		htlcValue := btcutil.Amount(htlc.SweepSignDesc.Output.Value)
		closeSummary.TimeLockedBalance += htlcValue
	}

	// Attempt to add a channel sync message to the close summary.
	chanSync, err := c.cfg.chanState.ChanSyncMsg()
	if err != nil {
		log.Errorf("ChannelPoint(%v): unable to create channel sync "+
			"message: %v", c.cfg.chanState.FundingOutpoint, err)
	} else {
		closeSummary.LastChanSyncMsg = chanSync
	}

	// With the event processed, we'll now notify all subscribers of the
	// event.
	closeInfo := &LocalUnilateralCloseInfo{
		SpendDetail:            commitSpend,
		LocalForceCloseSummary: forceClose,
		ChannelCloseSummary:    closeSummary,
		CommitSet:              commitSet,
	}
	c.Lock()
	for _, sub := range c.clientSubscriptions {
		select {
		case sub.LocalUnilateralClosure <- closeInfo:
		case <-c.quit:
			c.Unlock()
			return fmt.Errorf("exiting")
		}
	}
	c.Unlock()

	return nil
}

// zkDispatchMerchClose processes a detected force close by the Merchant.
// It will return the escrowTxid, merchCloseTxid, and the merchClose pkScript.
func (c *zkChainWatcher) zkDispatchMerchClose(escrowTxid chainhash.Hash,
	merchCloseTxid chainhash.Hash, pkScript []byte, amount int64) error {

	c.Lock()
	for _, sub := range c.clientSubscriptions {
		select {
		case sub.ZkMerchClosure <- &ZkMerchCloseInfo{
			escrowTxid:     escrowTxid,
			merchCloseTxid: merchCloseTxid,
			pkScript:       pkScript,
			amount:         amount,
		}:
		case <-c.quit:
			c.Unlock()
			return fmt.Errorf("exiting")
		}
	}
	c.Unlock()

	return nil
}

// zkDispatchCustClose processes a detected force close by the Customer.
// It will return the escrowTxid, closeTxid, the custClose pkScript,
// the revLock, and custClosePk.
func (c *zkChainWatcher) zkDispatchCustClose(escrowTxid chainhash.Hash,
	closeTxid chainhash.Hash, pkScript []byte, revLock string,
	custClosePk string, amount int64) error {

	c.Lock()
	for _, sub := range c.clientSubscriptions {
		select {
		case sub.ZkCustClosure <- &ZkCustCloseInfo{
			escrowTxid:  escrowTxid,
			closeTxid:   closeTxid,
			pkScript:    pkScript,
			revLock:     revLock,
			custClosePk: custClosePk,
			amount:      amount,
		}:
		case <-c.quit:
			c.Unlock()
			return fmt.Errorf("exiting")
		}
	}
	c.Unlock()

	return nil
}

// zkDispatchBreach processes a detected force close by the customer
// or by the merchant.
// It will return the escrowTxid,closeTxid, the ClosePkScript,
// the revLock, and custClosePk.
func (c *zkChainWatcher) zkDispatchBreach(zkBreachInfo ZkBreachInfo) error {

	fmt.Printf("zkDispatchCustBreach, zkBreachInfo %#v:\n", zkBreachInfo)
	// Hand the retribution info over to the breach arbiter.
	if err := c.cfg.zkContractBreach(&zkBreachInfo); err != nil {
		log.Errorf("unable to hand breached contract off to "+
			"zkBreachArbiter: %v", err)
		return err
	}

	c.Lock()
	for _, sub := range c.clientSubscriptions {
		select {
		case sub.ZkContractBreach <- &zkBreachInfo:

		case <-c.quit:
			c.Unlock()
			return fmt.Errorf("exiting")
		}
	}
	c.Unlock()

	return nil
}

// // dispatchContractBreach processes a detected contract breached by the remote
// // party. This method is to be called once we detect that the remote party has
// // broadcast a prior revoked commitment state. This method well prepare all the
// // materials required to bring the cheater to justice, then notify all
// // registered subscribers of this event.
// func (c *zkChainWatcher) dispatchContractBreach(spendEvent *chainntnfs.SpendDetail,
// 	remoteCommit *channeldb.ChannelCommitment,
// 	broadcastStateNum uint64) error {

// 	log.Warnf("Remote peer has breached the channel contract for "+
// 		"ChannelPoint(%v). Revoked state #%v was broadcast!!!",
// 		c.cfg.chanState.FundingOutpoint, broadcastStateNum)

// 	if err := c.cfg.chanState.MarkBorked(); err != nil {
// 		return fmt.Errorf("unable to mark channel as borked: %v", err)
// 	}

// 	spendHeight := uint32(spendEvent.SpendingHeight)

// 	// Create a new reach retribution struct which contains all the data
// 	// needed to swiftly bring the cheating peer to justice.
// 	//
// 	// TODO(roasbeef): move to same package
// 	retribution, err := lnwallet.NewBreachRetribution(
// 		c.cfg.chanState, broadcastStateNum, spendHeight,
// 	)
// 	if err != nil {
// 		return fmt.Errorf("unable to create breach retribution: %v", err)
// 	}

// 	// Nil the curve before printing.
// 	if retribution.RemoteOutputSignDesc != nil &&
// 		retribution.RemoteOutputSignDesc.DoubleTweak != nil {
// 		retribution.RemoteOutputSignDesc.DoubleTweak.Curve = nil
// 	}
// 	if retribution.RemoteOutputSignDesc != nil &&
// 		retribution.RemoteOutputSignDesc.KeyDesc.PubKey != nil {
// 		retribution.RemoteOutputSignDesc.KeyDesc.PubKey.Curve = nil
// 	}
// 	if retribution.LocalOutputSignDesc != nil &&
// 		retribution.LocalOutputSignDesc.DoubleTweak != nil {
// 		retribution.LocalOutputSignDesc.DoubleTweak.Curve = nil
// 	}
// 	if retribution.LocalOutputSignDesc != nil &&
// 		retribution.LocalOutputSignDesc.KeyDesc.PubKey != nil {
// 		retribution.LocalOutputSignDesc.KeyDesc.PubKey.Curve = nil
// 	}

// 	log.Debugf("Punishment breach retribution created: %v",
// 		newLogClosure(func() string {
// 			retribution.KeyRing.CommitPoint.Curve = nil
// 			retribution.KeyRing.LocalHtlcKey = nil
// 			retribution.KeyRing.RemoteHtlcKey = nil
// 			retribution.KeyRing.ToLocalKey = nil
// 			retribution.KeyRing.ToRemoteKey = nil
// 			retribution.KeyRing.RevocationKey = nil
// 			return spew.Sdump(retribution)
// 		}))

// 	// Hand the retribution info over to the breach arbiter.
// 	if err := c.cfg.contractBreach(retribution); err != nil {
// 		log.Errorf("unable to hand breached contract off to "+
// 			"breachArbiter: %v", err)
// 		return err
// 	}

// 	// With the event processed, we'll now notify all subscribers of the
// 	// event.
// 	c.Lock()
// 	for _, sub := range c.clientSubscriptions {
// 		select {
// 		case sub.ContractBreach <- retribution:
// 		case <-c.quit:
// 			c.Unlock()
// 			return fmt.Errorf("quitting")
// 		}
// 	}
// 	c.Unlock()

// 	// At this point, we've successfully received an ack for the breach
// 	// close. We now construct and persist  the close summary, marking the
// 	// channel as pending force closed.
// 	//
// 	// TODO(roasbeef): instead mark we got all the monies?
// 	// TODO(halseth): move responsibility to breach arbiter?
// 	settledBalance := remoteCommit.LocalBalance.ToSatoshis()
// 	closeSummary := channeldb.ChannelCloseSummary{
// 		ChanPoint:               c.cfg.chanState.FundingOutpoint,
// 		ChainHash:               c.cfg.chanState.ChainHash,
// 		ClosingTXID:             *spendEvent.SpenderTxHash,
// 		CloseHeight:             spendHeight,
// 		RemotePub:               c.cfg.chanState.IdentityPub,
// 		Capacity:                c.cfg.chanState.Capacity,
// 		SettledBalance:          settledBalance,
// 		CloseType:               channeldb.BreachClose,
// 		IsPending:               true,
// 		ShortChanID:             c.cfg.chanState.ShortChanID(),
// 		RemoteCurrentRevocation: c.cfg.chanState.RemoteCurrentRevocation,
// 		RemoteNextRevocation:    c.cfg.chanState.RemoteNextRevocation,
// 		LocalChanConfig:         c.cfg.chanState.LocalChanCfg,
// 	}

// 	// Attempt to add a channel sync message to the close summary.
// 	chanSync, err := c.cfg.chanState.ChanSyncMsg()
// 	if err != nil {
// 		log.Errorf("ChannelPoint(%v): unable to create channel sync "+
// 			"message: %v", c.cfg.chanState.FundingOutpoint, err)
// 	} else {
// 		closeSummary.LastChanSyncMsg = chanSync
// 	}

// 	if err := c.cfg.chanState.CloseChannel(&closeSummary); err != nil {
// 		return err
// 	}

// 	log.Infof("Breached channel=%v marked pending-closed",
// 		c.cfg.chanState.FundingOutpoint)

// 	return nil
// }

// // waitForCommitmentPoint waits for the commitment point to be inserted into
// // the local database. We'll use this method in the DLP case, to wait for the
// // remote party to send us their point, as we can't proceed until we have that.
// func (c *zkChainWatcher) waitForCommitmentPoint() *btcec.PublicKey {
// 	// If we are lucky, the remote peer sent us the correct commitment
// 	// point during channel sync, such that we can sweep our funds. If we
// 	// cannot find the commit point, there's not much we can do other than
// 	// wait for us to retrieve it. We will attempt to retrieve it from the
// 	// peer each time we connect to it.
// 	//
// 	// TODO(halseth): actively initiate re-connection to the peer?
// 	backoff := minCommitPointPollTimeout
// 	for {
// 		commitPoint, err := c.cfg.chanState.DataLossCommitPoint()
// 		if err == nil {
// 			return commitPoint
// 		}

// 		log.Errorf("Unable to retrieve commitment point for "+
// 			"channel(%v) with lost state: %v. Retrying in %v.",
// 			c.cfg.chanState.FundingOutpoint, err, backoff)

// 		select {
// 		// Wait before retrying, with an exponential backoff.
// 		case <-time.After(backoff):
// 			backoff = 2 * backoff
// 			if backoff > maxCommitPointPollTimeout {
// 				backoff = maxCommitPointPollTimeout
// 			}

// 		case <-c.quit:
// 			return nil
// 		}
// 	}
// }
