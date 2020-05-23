package lnd

import (
	"bytes"
	"encoding/hex"
	"errors"
	"sync"

	"github.com/btcsuite/btcd/wire"

	"github.com/lightningnetwork/lnd/chainntnfs"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/contractcourt"
	"github.com/lightningnetwork/lnd/htlcswitch"
	"github.com/lightningnetwork/lnd/input"
	"github.com/lightningnetwork/lnd/libzkchannels"
	"github.com/lightningnetwork/lnd/lnwallet/chainfee"
	"github.com/lightningnetwork/lnd/zkchanneldb"
)

var (
	// zkretributionBucket stores zkretribution state on disk between detecting
	// a contract breach, broadcasting a zkjustice transaction that sweeps the
	// channel, and finally witnessing the zkjustice transaction confirm on
	// the blockchain. It is critical that such state is persisted on disk,
	// so that if our node restarts at any point during the zkretribution
	// procedure, we can recover and continue from the persisted state.
	zkretributionBucket = []byte("zkretribution")

	// zkjusticeTxnBucket holds the finalized zkjustice transactions for all
	// breached contracts. Entries are added to the zkjustice txn bucket just
	// before broadcasting the sweep txn.
	zkjusticeTxnBucket = []byte("zkjustice-txn")

	// errZkBrarShuttingDown is an error returned if the breacharbiter has
	// been signalled to exit.
	errZkBrarShuttingDown = errors.New("breacharbiter shutting down")
)

// ZkContractBreachEvent is an event the zkBreachArbiter will receive in case a
// contract breach is observed on-chain. It contains the necessary information
// to handle the breach, and a ProcessACK channel we will use to ACK the event
// when we have safely stored all the necessary information.
type ZkContractBreachEvent struct {
	// ChanPoint is the channel point of the breached channel.
	ChanPoint wire.OutPoint

	// ProcessACK is an error channel where a nil error should be sent
	// iff the breach zkretribution info is safely stored in the zkretribution
	// store. In case storing the information to the store fails, a non-nil
	// error should be sent.
	ProcessACK chan error

	// ZkBreachInfo is the information needed to act on this cust contract
	// breach.
	ZkBreachInfo contractcourt.ZkBreachInfo
}

// ZkBreachConfig bundles the required subsystems used by the zk breach arbiter. An
// instance of ZkBreachConfig is passed to newZkBreachArbiter during instantiation.
type ZkBreachConfig struct {
	// CloseLink allows the zk breach arbiter to shutdown any channel links for
	// which it detects a breach, ensuring now further activity will
	// continue across the link. The method accepts link's channel point and
	// a close type to be included in the channel close summary.
	CloseLink func(*wire.OutPoint, htlcswitch.ChannelCloseType)

	// DBPath provides path to zkChannelDb
	DBPath string

	// DB provides access to the user's channels, allowing the breach
	// arbiter to determine the current state of a user's channels, and how
	// it should respond to channel closure.
	DB *channeldb.DB

	// Estimator is used by the zk breach arbiter to determine an appropriate
	// fee level when generating, signing, and broadcasting sweep
	// transactions.
	Estimator chainfee.Estimator

	// GenSweepScript generates the receiving scripts for swept outputs.
	GenSweepScript func() ([]byte, error)

	// Notifier provides a publish/subscribe interface for event driven
	// notifications regarding the confirmation of txids.
	Notifier chainntnfs.ChainNotifier

	// PublishTransaction facilitates the process of broadcasting a
	// transaction to the network.
	PublishTransaction func(*wire.MsgTx, string) error

	// CustContractBreaches is a channel where the zkBreachArbiter will receive
	// notifications in the event of a contract breach being observed. A
	// ZkContractBreachEvent must be ACKed by the zkBreachArbiter, such that
	// the sending subsystem knows that the event is properly handed off.
	CustContractBreaches <-chan *ZkContractBreachEvent

	// Signer is used by the zk breach arbiter to generate sweep transactions,
	// which move coins from previously open channels back to the user's
	// wallet.
	Signer input.Signer

	// // Store is a persistent resource that maintains information regarding
	// // breached channels. This is used in conjunction with DB to recover
	// // from crashes, restarts, or other failures.
	// Store ZkRetributionStore
}

// zkBreachArbiter is a special subsystem which is responsible for watching and
// acting on the detection of any attempted uncooperative channel breaches by
// channel counterparties. This file essentially acts as deterrence code for
// those attempting to launch attacks against the daemon. In practice it's
// expected that the logic in this file never gets executed, but it is
// important to have it in place just in case we encounter cheating channel
// counterparties.
// TODO(roasbeef): closures in config for subsystem pointers to decouple?
type zkBreachArbiter struct {
	started sync.Once
	stopped sync.Once

	cfg *ZkBreachConfig

	quit chan struct{}
	wg   sync.WaitGroup
	sync.Mutex
}

// newZkBreachArbiter creates a new instance of a zkBreachArbiter initialized with
// its dependent objects.
func newZkBreachArbiter(cfg *ZkBreachConfig) *zkBreachArbiter {
	return &zkBreachArbiter{
		cfg:  cfg,
		quit: make(chan struct{}),
	}
}

// Start is an idempotent method that officially starts the zkBreachArbiter along
// with all other goroutines it needs to perform its functions.
func (b *zkBreachArbiter) Start() error {
	var err error
	b.started.Do(func() {
		err = b.start()
	})
	return err
}

func (b *zkBreachArbiter) start() error {
	zkbaLog.Tracef("Starting zk breach arbiter")

	// // Load all zkretributions currently persisted in the zkretribution store.
	// breachRetInfos := make(map[wire.OutPoint]zkretributionInfo)
	// if err := b.cfg.Store.ForAll(func(ret *zkretributionInfo) error {
	// 	breachRetInfos[ret.chanPoint] = *ret
	// 	return nil
	// }); err != nil {
	// 	return err
	// }

	// // Load all currently closed channels from disk, we will use the
	// // channels that have been marked fully closed to filter the zkretribution
	// // information loaded from disk. This is necessary in the event that the
	// // channel was marked fully closed, but was not removed from the
	// // zkretribution store.
	// closedChans, err := b.cfg.DB.FetchClosedChannels(false)
	// if err != nil {
	// 	zkbaLog.Errorf("Unable to fetch closing channels: %v", err)
	// 	return err
	// }

	// // Using the set of non-pending, closed channels, reconcile any
	// // discrepancies between the channeldb and the zkretribution store by
	// // removing any zkretribution information for which we have already
	// // finished our responsibilities. If the removal is successful, we also
	// // remove the entry from our in-memory map, to avoid any further action
	// // for this channel.
	// // TODO(halseth): no need continue on IsPending once closed channels
	// // actually means close transaction is confirmed.
	// for _, chanSummary := range closedChans {
	// 	if chanSummary.IsPending {
	// 		continue
	// 	}

	// 	chanPoint := &chanSummary.ChanPoint
	// 	if _, ok := breachRetInfos[*chanPoint]; ok {
	// 		if err := b.cfg.Store.Remove(chanPoint); err != nil {
	// 			zkbaLog.Errorf("Unable to remove closed "+
	// 				"chanid=%v from zk breach arbiter: %v",
	// 				chanPoint, err)
	// 			return err
	// 		}
	// 		delete(breachRetInfos, *chanPoint)
	// 	}
	// }

	// // Spawn the exactZkRetribution tasks to monitor and resolve any breaches
	// // that were loaded from the zkretribution store.
	// for chanPoint := range breachRetInfos {
	// 	retInfo := breachRetInfos[chanPoint]

	// 	// Register for a notification when the breach transaction is
	// 	// confirmed on chain.
	// 	breachTXID := retInfo.commitHash
	// 	breachScript := retInfo.zkBreachedOutputs[0].signDesc.Output.PkScript
	// 	confChan, err := b.cfg.Notifier.RegisterConfirmationsNtfn(
	// 		&breachTXID, breachScript, 1, retInfo.breachHeight,
	// 	)
	// 	if err != nil {
	// 		zkbaLog.Errorf("Unable to register for conf updates "+
	// 			"for txid: %v, err: %v", breachTXID, err)
	// 		return err
	// 	}

	// 	// Launch a new goroutine which to finalize the channel
	// 	// zkretribution after the breach transaction confirms.
	// 	b.wg.Add(1)
	// 	go b.exactZkRetribution(confChan, &retInfo)
	// }

	// Start watching the remaining active channels!
	b.wg.Add(1)
	go b.contractObserver()

	return nil
}

// Stop is an idempotent method that signals the zkBreachArbiter to execute a
// graceful shutdown. This function will block until all goroutines spawned by
// the zkBreachArbiter have gracefully exited.
func (b *zkBreachArbiter) Stop() error {
	b.stopped.Do(func() {
		zkbaLog.Infof("zk Breach arbiter shutting down")

		close(b.quit)
		b.wg.Wait()
	})
	return nil
}

// // IsBreached queries the zk breach arbiter's zkretribution store to see if it is
// // aware of any channel breaches for a particular channel point.
// func (b *zkBreachArbiter) IsBreached(chanPoint *wire.OutPoint) (bool, error) {
// 	return b.cfg.Store.IsBreached(chanPoint)
// }

// contractObserver is the primary goroutine for the zkBreachArbiter. This
// goroutine is responsible for handling breach events coming from the
// contractcourt on the CustContractBreaches channel. If a channel breach is
// detected, then the contractObserver will execute the zkretribution logic
// required to sweep ALL outputs from a contested channel into the daemon's
// wallet.
//
// NOTE: This MUST be run as a goroutine.
func (b *zkBreachArbiter) contractObserver() {
	defer b.wg.Done()

	zkbaLog.Infof("Starting contract observer, watching for breaches.")

	for {
		select {
		case zkBreachEvent := <-b.cfg.CustContractBreaches:
			// We have been notified about a contract breach!
			// Handle the handoff, making sure we ACK the event
			// after we have safely added it to the zkretribution
			// store.
			b.wg.Add(1)
			go b.handleCustBreachHandoff(zkBreachEvent)

		case <-b.quit:
			return
		}
	}
}

// // zkConvertToSecondLevelRevoke takes a breached output, and a transaction that
// // spends it to the second level, and mutates the breach output into one that
// // is able to properly sweep that second level output. We'll use this function
// // when we go to sweep a breached commitment transaction, but the cheating
// // party has already attempted to take it to the second level
// func zkConvertToSecondLevelRevoke(bo *zkBreachedOutput, breachInfo *zkretributionInfo,
// 	spendDetails *chainntnfs.SpendDetail) {

// 	// In this case, we'll modify the witness type of this output to
// 	// actually prepare for a second level revoke.
// 	bo.witnessType = input.HtlcSecondLevelRevoke

// 	// We'll also redirect the outpoint to this second level output, so the
// 	// spending transaction updates it inputs accordingly.
// 	spendingTx := spendDetails.SpendingTx
// 	oldOp := bo.outpoint
// 	bo.outpoint = wire.OutPoint{
// 		Hash:  spendingTx.TxHash(),
// 		Index: 0,
// 	}

// 	// Next, we need to update the amount so we can do fee estimation
// 	// properly, and also so we can generate a valid signature as we need
// 	// to know the new input value (the second level transactions shaves
// 	// off some funds to fees).
// 	newAmt := spendingTx.TxOut[0].Value
// 	bo.amt = btcutil.Amount(newAmt)
// 	bo.signDesc.Output.Value = newAmt
// 	bo.signDesc.Output.PkScript = spendingTx.TxOut[0].PkScript

// 	// Finally, we'll need to adjust the witness program in the
// 	// SignDescriptor.
// 	bo.signDesc.WitnessScript = bo.secondLevelWitnessScript

// 	zkbaLog.Warnf("HTLC(%v) for ChannelPoint(%v) has been spent to the "+
// 		"second-level, adjusting -> %v", oldOp, breachInfo.chanPoint,
// 		bo.outpoint)
// }

// // waitForSpendEvent waits for any of the breached outputs to get spent, and
// // mutates the breachInfo to be able to sweep it. This method should be used
// // when we fail to publish the zkjustice tx because of a double spend, indicating
// // that the counter party has taken one of the breached outputs to the second
// // level. The spendNtfns map is a cache used to store registered spend
// // subscriptions, in case we must call this method multiple times.
// func (b *zkBreachArbiter) waitForSpendEvent(breachInfo *zkretributionInfo,
// 	spendNtfns map[wire.OutPoint]*chainntnfs.SpendEvent) error {

// 	inputs := breachInfo.zkBreachedOutputs

// 	// spend is used to wrap the index of the output that gets spent
// 	// together with the spend details.
// 	type spend struct {
// 		index  int
// 		detail *chainntnfs.SpendDetail
// 	}

// 	// We create a channel the first goroutine that gets a spend event can
// 	// signal. We make it buffered in case multiple spend events come in at
// 	// the same time.
// 	anySpend := make(chan struct{}, len(inputs))

// 	// The allSpends channel will be used to pass spend events from all the
// 	// goroutines that detects a spend before they are signalled to exit.
// 	allSpends := make(chan spend, len(inputs))

// 	// exit will be used to signal the goroutines that they can exit.
// 	exit := make(chan struct{})
// 	var wg sync.WaitGroup

// 	// We'll now launch a goroutine for each of the HTLC outputs, that will
// 	// signal the moment they detect a spend event.
// 	for i := range inputs {
// 		zkBreachedOutput := &inputs[i]

// 		zkbaLog.Infof("Checking spend from %v(%v) for ChannelPoint(%v)",
// 			zkBreachedOutput.witnessType, zkBreachedOutput.outpoint,
// 			breachInfo.chanPoint)

// 		// If we have already registered for a notification for this
// 		// output, we'll reuse it.
// 		spendNtfn, ok := spendNtfns[zkBreachedOutput.outpoint]
// 		if !ok {
// 			var err error
// 			spendNtfn, err = b.cfg.Notifier.RegisterSpendNtfn(
// 				&zkBreachedOutput.outpoint,
// 				zkBreachedOutput.signDesc.Output.PkScript,
// 				breachInfo.breachHeight,
// 			)
// 			if err != nil {
// 				zkbaLog.Errorf("Unable to check for spentness "+
// 					"of outpoint=%v: %v",
// 					zkBreachedOutput.outpoint, err)

// 				// Registration may have failed if we've been
// 				// instructed to shutdown. If so, return here
// 				// to avoid entering an infinite loop.
// 				select {
// 				case <-b.quit:
// 					return errZkBrarShuttingDown
// 				default:
// 					continue
// 				}
// 			}
// 			spendNtfns[zkBreachedOutput.outpoint] = spendNtfn
// 		}

// 		// Launch a goroutine waiting for a spend event.
// 		b.wg.Add(1)
// 		wg.Add(1)
// 		go func(index int, spendEv *chainntnfs.SpendEvent) {
// 			defer b.wg.Done()
// 			defer wg.Done()

// 			select {
// 			// The output has been taken to the second level!
// 			case sp, ok := <-spendEv.Spend:
// 				if !ok {
// 					return
// 				}

// 				zkbaLog.Infof("Detected spend on %s(%v) by "+
// 					"txid(%v) for ChannelPoint(%v)",
// 					inputs[index].witnessType,
// 					inputs[index].outpoint,
// 					sp.SpenderTxHash,
// 					breachInfo.chanPoint)

// 				// First we send the spend event on the
// 				// allSpends channel, such that it can be
// 				// handled after all go routines have exited.
// 				allSpends <- spend{index, sp}

// 				// Finally we'll signal the anySpend channel
// 				// that a spend was detected, such that the
// 				// other goroutines can be shut down.
// 				anySpend <- struct{}{}
// 			case <-exit:
// 				return
// 			case <-b.quit:
// 				return
// 			}
// 		}(i, spendNtfn)
// 	}

// 	// We'll wait for any of the outputs to be spent, or that we are
// 	// signalled to exit.
// 	select {
// 	// A goroutine have signalled that a spend occurred.
// 	case <-anySpend:
// 		// Signal for the remaining goroutines to exit.
// 		close(exit)
// 		wg.Wait()

// 		// At this point all goroutines that can send on the allSpends
// 		// channel have exited. We can therefore safely close the
// 		// channel before ranging over its content.
// 		close(allSpends)

// 		doneOutputs := make(map[int]struct{})
// 		for s := range allSpends {
// 			zkBreachedOutput := &inputs[s.index]
// 			delete(spendNtfns, zkBreachedOutput.outpoint)

// 			switch zkBreachedOutput.witnessType {
// 			case input.HtlcAcceptedRevoke:
// 				fallthrough
// 			case input.HtlcOfferedRevoke:
// 				zkbaLog.Infof("Spend on second-level"+
// 					"%s(%v) for ChannelPoint(%v) "+
// 					"transitions to second-level output",
// 					zkBreachedOutput.witnessType,
// 					zkBreachedOutput.outpoint,
// 					breachInfo.chanPoint)

// 				// In this case we'll morph our initial revoke
// 				// spend to instead point to the second level
// 				// output, and update the sign descriptor in the
// 				// process.
// 				zkConvertToSecondLevelRevoke(
// 					zkBreachedOutput, breachInfo, s.detail,
// 				)

// 				continue
// 			}

// 			zkbaLog.Infof("Spend on %s(%v) for ChannelPoint(%v) "+
// 				"transitions output to terminal state, "+
// 				"removing input from zkjustice transaction",
// 				zkBreachedOutput.witnessType,
// 				zkBreachedOutput.outpoint, breachInfo.chanPoint)

// 			doneOutputs[s.index] = struct{}{}
// 		}

// 		// Filter the inputs for which we can no longer proceed.
// 		var nextIndex int
// 		for i := range inputs {
// 			if _, ok := doneOutputs[i]; ok {
// 				continue
// 			}

// 			inputs[nextIndex] = inputs[i]
// 			nextIndex++
// 		}

// 		// Update our remaining set of outputs before continuing with
// 		// another attempt at publication.
// 		breachInfo.zkBreachedOutputs = inputs[:nextIndex]

// 	case <-b.quit:
// 		return errZkBrarShuttingDown
// 	}

// 	return nil
// }

// exactZkDispute is a goroutine which is executed once a contract breach has
// been detected by a breachObserver. This function creates and broadcasts
// the disputeTx, which the merchant uses to punish a customer broadcasting
// a revoked custCloseTx
//
// NOTE: This MUST be run as a goroutine.
func (b *zkBreachArbiter) exactZkDispute(confChan *chainntnfs.ConfirmationEvent,
	breachInfo *contractcourt.ZkBreachInfo) {

	defer b.wg.Done()

	// TODO(roasbeef): state needs to be checkpointed here
	var breachConfHeight uint32
	select {
	case breachConf, ok := <-confChan.Confirmed:
		// If the second value is !ok, then the channel has been closed
		// signifying a daemon shutdown, so we exit.

		if !ok {
			return
		}

		breachConfHeight = breachConf.BlockHeight

		// Otherwise, if this is a real confirmation notification, then
		// we fall through to complete our duty.
	case <-b.quit:
		return
	}

	zkbaLog.Debugf("Breach transaction %v has been confirmed, sweeping "+
		"revoked funds", breachInfo.CloseTxid)

	breachTxid := breachInfo.CloseTxid.String()
	index := uint32(0)
	// TODO ZKLND-33: Generate outputPk from merchant's wallet
	outputPk := "03df51984d6b8b8b1cc693e239491f77a36c9e9dfe4a486e9972a18e03610a0d22"
	revLock := breachInfo.RevLock
	revSecret := breachInfo.RevSecret
	custClosePk := breachInfo.CustClosePk
	amount := breachInfo.Amount

	// open the zkchanneldb to load merchState and channelState
	zkMerchDB, err := zkchanneldb.SetupDB(b.cfg.DBPath)
	if err != nil {
		zkchLog.Error(err)
		return
	}

	merchState, err := zkchanneldb.GetMerchState(zkMerchDB)
	if err != nil {
		zkchLog.Error(err)
		return
	}

	var channelState libzkchannels.ChannelState
	err = zkchanneldb.GetMerchField(zkMerchDB, "channelStateKey", &channelState)
	if err != nil {
		zkchLog.Error(err)
		return
	}

	err = zkMerchDB.Close()
	if err != nil {
		zkchLog.Error(err)
	}
	zkbaLog.Debugf("merchState %#v:", merchState)

	toSelfDelay, err := libzkchannels.GetSelfDelayBE(channelState)
	if err != nil {
		zkchLog.Error(err)
		return
	}

	// with all the info needed, create and sign the Dispute/Justice Tx.
	finalTxStr, err := libzkchannels.MerchantSignDisputeTx(breachTxid, index, amount, toSelfDelay, outputPk,
		revLock, revSecret, custClosePk, merchState)
	if err != nil {
		zkchLog.Error(err)
		return
	}

	// Dummy finalTxStr for zkBreachArbiter Test
	// finalTxStr := "0200000000010120827352e391fe0c6cbaee2d07679ec67e30119448e4183b7818ff29171f489a0000000000ffffffff0106270000000000001600141d2cc47e2a0d77927a333a2165fe2d343b79eefc04483045022100e95687eb9aec340a662d57e29e80efe23bd013ce4b9a2e0383cd3c5f3370a526022063b260ddc1b9a251ca5f6be6fc9d6a30b29402d7e66959985487b7b61530ad3001200d4a4cf5b18a70f5e9f6a677924d0b2450f3d0561402a42338fd08da905112a101017063a8203ae763fc25bc086f73836f21aa05d387cd3adac5a93d5345e782128133cb03e4882102f9e7281167132a13b2326cea57d789b9fd2ea4440c9e9b78e6402e722a1e5c526702cf05b27521027160fb5e48252f02a00066dfa823d15844ad93e04f9c9b746e1f28ed4a1eaddb68ac00000000"

	zkchLog.Warnf("finalTxStr: %#v\n", finalTxStr)

	// Broadcast dispute Tx on chain
	finalTxBytes, err := hex.DecodeString(finalTxStr)
	if err != nil {
		zkchLog.Error(err)
		return
	}

	var finalTx wire.MsgTx
	err = finalTx.Deserialize(bytes.NewReader(finalTxBytes))
	if err != nil {
		zkchLog.Error(err)
		return
	}

	zkchLog.Debugf("Broadcasting dispute Tx: %#v\n", finalTx)

	err = b.cfg.PublishTransaction(&finalTx, "")
	if err != nil {
		zkchLog.Error(err)
		return
	}

	// As a conclusionary step, we register for a notification to be
	// dispatched once the zkjustice tx is confirmed. After confirmation we
	// notify the caller that initiated the zkretribution workflow that the
	// deed has been done.
	zkbaLog.Debugf("finalTx.TxHash() %#v:", finalTx.TxHash())
	zkbaLog.Debugf("finalTx.TxOut[0].PkScript %#v:", finalTx.TxOut[0].PkScript)

	zkjusticeTXID := finalTx.TxHash()
	zkjusticeScript := finalTx.TxOut[0].PkScript
	confChan, err = b.cfg.Notifier.RegisterConfirmationsNtfn(
		&zkjusticeTXID, zkjusticeScript, 1, breachConfHeight,
	)

	if err != nil {
		zkbaLog.Errorf("Unable to register for conf for txid(%v): %v",
			zkjusticeTXID, err)
		return
	}

	select {
	case _, ok := <-confChan.Confirmed:
		if !ok {
			return
		}

		zkbaLog.Infof("ZkJustice for ChannelPoint(%v) has "+
			"been served, %v revoked funds "+
			"have been claimed", breachInfo.EscrowTxid,
			breachInfo.Amount)

		return
	case <-b.quit:
		return
	}
}

// exactZkCloseMerch is a goroutine which is executed once a contract breach has
// been detected by a breachObserver. This function creates and broadcasts
// the latest custClose, which the customer uses to close a channel in response
// to the merchant broadcasting merchClose.
//
// NOTE: This MUST be run as a goroutine.
func (b *zkBreachArbiter) exactZkCloseMerch(confChan *chainntnfs.ConfirmationEvent,
	breachInfo *contractcourt.ZkBreachInfo) {

	defer b.wg.Done()

	// TODO(roasbeef): state needs to be checkpointed here
	var breachConfHeight uint32
	select {
	case breachConf, ok := <-confChan.Confirmed:
		// If the second value is !ok, then the channel has been closed
		// signifying a daemon shutdown, so we exit.

		if !ok {
			return
		}

		breachConfHeight = breachConf.BlockHeight

		// Otherwise, if this is a real confirmation notification, then
		// we fall through to complete our duty.
	case <-b.quit:
		return
	}

	zkbaLog.Debugf("Merch Close transaction %v has been confirmed. ", breachInfo.CloseTxid)

	closeFromEscrow := false
	closeMerchTx, _, err := GetSignedCustCloseTxs(breachInfo.CustChannelName, closeFromEscrow)

	zkchLog.Warnf("closeMerchTx: %#v\n", closeMerchTx)

	// Broadcast dispute Tx on chain
	finalTxBytes, err := hex.DecodeString(closeMerchTx)
	if err != nil {
		zkchLog.Error(err)
	}

	var finalTx wire.MsgTx
	err = finalTx.Deserialize(bytes.NewReader(finalTxBytes))
	if err != nil {
		zkchLog.Error(err)
	}

	zkchLog.Debugf("Broadcasting closeMerchTx: %#v\n", finalTx)

	err = b.cfg.PublishTransaction(&finalTx, "")
	if err != nil {
		zkchLog.Error(err)
	}

	// As a conclusionary step, we register for a notification to be
	// dispatched once the zkjustice tx is confirmed. After confirmation we
	// notify the caller that initiated the zkretribution workflow that the
	// deed has been done.
	zkbaLog.Debugf("finalTx.TxHash() %#v:", finalTx.TxHash())
	zkbaLog.Debugf("finalTx.TxOut[0].PkScript %#v:", finalTx.TxOut[0].PkScript)

	merchCloseTxid := finalTx.TxHash()
	merchCloseScript := finalTx.TxOut[0].PkScript
	confChan, err = b.cfg.Notifier.RegisterConfirmationsNtfn(
		&merchCloseTxid, merchCloseScript, 1, breachConfHeight,
	)

	if err != nil {
		zkbaLog.Errorf("Unable to register for conf for txid(%v): %v",
			merchCloseTxid, err)
		return
	}

	select {
	case _, ok := <-confChan.Confirmed:
		if !ok {
			return
		}

		zkbaLog.Infof("ZkJustice for ChannelPoint(%v) has "+
			"been served, %v revoked funds "+
			"have been claimed", breachInfo.EscrowTxid,
			breachInfo.Amount)

		return
	case <-b.quit:
		return
	}
}

// // cleanupBreach marks the given channel point as fully resolved and removes the
// // zkretribution for that the channel from the zkretribution store.
// func (b *zkBreachArbiter) cleanupBreach(chanPoint *wire.OutPoint) error {
// 	// With the channel closed, mark it in the database as such.
// 	err := b.cfg.DB.MarkChanFullyClosed(chanPoint)
// 	if err != nil {
// 		return fmt.Errorf("unable to mark chan as closed: %v", err)
// 	}

// 	// ZkJustice has been carried out; we can safely delete the zkretribution
// 	// info from the database.
// 	err = b.cfg.Store.Remove(chanPoint)
// 	if err != nil {
// 		return fmt.Errorf("unable to remove zkretribution from db: %v",
// 			err)
// 	}

// 	return nil
// }

// handleCustBreachHandoff handles a new breach event, by writing it to disk, then
// notifies the zkBreachArbiter contract observer goroutine that a channel's
// contract has been breached by the prior counterparty. Once notified the
// zkBreachArbiter will attempt to sweep ALL funds within the channel using the
// information provided within the BreachRetribution generated due to the
// breach of channel contract. The funds will be swept only after the breaching
// transaction receives a necessary number of confirmations.
//
// NOTE: This MUST be run as a goroutine.
func (b *zkBreachArbiter) handleCustBreachHandoff(zkBreachEvent *ZkContractBreachEvent) {
	defer b.wg.Done()

	chanPoint := zkBreachEvent.ChanPoint
	zkbaLog.Debugf("Handling breach handoff for ChannelPoint(%v)",
		chanPoint)

	// // A read from this channel indicates that a channel breach has been
	// // detected! So we notify the main coordination goroutine with the
	// // information needed to bring the counterparty to zkjustice.
	// breachInfo := zkBreachEvent.ZkBreachInfo

	zkbaLog.Warnf("REVOKED STATE FOR ChannelPoint(%v) "+
		"broadcast, REMOTE PEER IS DOING SOMETHING "+
		"SKETCHY!!!",
		chanPoint)

	// // Immediately notify the HTLC switch that this link has been
	// // breached in order to ensure any incoming or outgoing
	// // multi-hop HTLCs aren't sent over this link, nor any other
	// // links associated with this peer.
	// b.cfg.CloseLink(&chanPoint, htlcswitch.CloseBreach)

	// // TODO(roasbeef): need to handle case of remote broadcast
	// // mid-local initiated state-transition, possible
	// // false-positive?

	// Acquire the mutex to ensure consistency between the call to
	// IsBreached and Add below.
	b.Lock()

	// // We first check if this breach info is already added to the
	// // zkretribution store.
	// breached, err := b.cfg.Store.IsBreached(&chanPoint)
	// if err != nil {
	// 	b.Unlock()
	// 	zkbaLog.Errorf("Unable to check breach info in DB: %v", err)

	// 	select {
	// 	case zkBreachEvent.ProcessACK <- err:
	// 	case <-b.quit:
	// 	}
	// 	return
	// }

	// // If this channel is already marked as breached in the zkretribution
	// // store, we already have handled the handoff for this breach. In this
	// // case we can safely ACK the handoff, and return.
	// if breached {
	// 	b.Unlock()

	// 	select {
	// 	case zkBreachEvent.ProcessACK <- nil:
	// 	case <-b.quit:
	// 	}
	// 	return
	// }

	// // Using the breach information provided by the wallet and the
	// // channel snapshot, construct the zkretribution information that
	// // will be persisted to disk.
	// retInfo := newZkRetributionInfo(&chanPoint, breachInfo)

	// // Persist the pending zkretribution state to disk.
	// err = b.cfg.Store.Add(retInfo)
	// b.Unlock()
	// if err != nil {
	// 	zkbaLog.Errorf("Unable to persist zkretribution "+
	// 		"info to db: %v", err)
	// }

	// err := fmt.Errorf("mock error from retribution store")
	var err error

	// Now that the breach has been persisted, try to send an
	// acknowledgment back to the close observer with the error. If
	// the ack is successful, the close observer will mark the
	// channel as pending-closed in the channeldb.
	select {
	case zkBreachEvent.ProcessACK <- err:
		// Bail if we failed to persist zkretribution info.
		if err != nil {
			return
		}

	case <-b.quit:
		return
	}

	// Now that a new channel contract has been added to the zkretribution
	// store, we first register for a notification to be dispatched once
	// the breach transaction (the revoked commitment transaction) has been
	// confirmed in the chain to ensure we're not dealing with a moving
	// target.

	breachTXID := zkBreachEvent.ZkBreachInfo.CloseTxid
	breachScript := zkBreachEvent.ZkBreachInfo.ClosePkScript
	breachHeight := uint32(300) // TODO: Put in actual breachHeight

	cfChan, err := b.cfg.Notifier.RegisterConfirmationsNtfn(
		&breachTXID, breachScript, 1, breachHeight,
	)
	if err != nil {
		zkbaLog.Errorf("Unable to register for conf updates for "+
			"txid: %v, err: %v", breachTXID, err)
		return
	}

	zkbaLog.Warnf("A channel has been breached with txid: %v. Waiting "+
		"for confirmation, then zkjustice will be served!", breachTXID)

	// exactZkRetribution will broadcast either the latest custClose,
	// or the
	b.wg.Add(1)
	if zkBreachEvent.ZkBreachInfo.IsMerchClose {
		go b.exactZkCloseMerch(cfChan, &zkBreachEvent.ZkBreachInfo)
	} else {
		go b.exactZkDispute(cfChan, &zkBreachEvent.ZkBreachInfo)
	}
}

// // zkBreachedOutput contains all the information needed to sweep a breached
// // output. A breached output is an output that we are now entitled to due to a
// // revoked commitment transaction being broadcast.
// type zkBreachedOutput struct {
// 	amt         btcutil.Amount
// 	outpoint    wire.OutPoint
// 	witnessType input.StandardWitnessType
// 	signDesc    input.SignDescriptor
// 	confHeight  uint32

// 	secondLevelWitnessScript []byte

// 	witnessFunc input.WitnessGenerator
// }

// // makeZkBreachedOutput assembles a new zkBreachedOutput that can be used by the
// // zk breach arbiter to construct a zkjustice or sweep transaction.
// func makeZkBreachedOutput(outpoint *wire.OutPoint,
// 	witnessType input.StandardWitnessType,
// 	secondLevelScript []byte,
// 	signDescriptor *input.SignDescriptor,
// 	confHeight uint32) zkBreachedOutput {

// 	amount := signDescriptor.Output.Value

// 	return zkBreachedOutput{
// 		amt:                      btcutil.Amount(amount),
// 		outpoint:                 *outpoint,
// 		secondLevelWitnessScript: secondLevelScript,
// 		witnessType:              witnessType,
// 		signDesc:                 *signDescriptor,
// 		confHeight:               confHeight,
// 	}
// }

// // Amount returns the number of satoshis contained in the breached output.
// func (bo *zkBreachedOutput) Amount() btcutil.Amount {
// 	return bo.amt
// }

// // OutPoint returns the breached output's identifier that is to be included as a
// // transaction input.
// func (bo *zkBreachedOutput) OutPoint() *wire.OutPoint {
// 	return &bo.outpoint
// }

// // WitnessType returns the type of witness that must be generated to spend the
// // breached output.
// func (bo *zkBreachedOutput) WitnessType() input.WitnessType {
// 	return bo.witnessType
// }

// // SignDesc returns the breached output's SignDescriptor, which is used during
// // signing to compute the witness.
// func (bo *zkBreachedOutput) SignDesc() *input.SignDescriptor {
// 	return &bo.signDesc
// }

// // CraftInputScript computes a valid witness that allows us to spend from the
// // breached output. It does so by first generating and memoizing the witness
// // generation function, which parameterized primarily by the witness type and
// // sign descriptor. The method then returns the witness computed by invoking
// // this function on the first and subsequent calls.
// func (bo *zkBreachedOutput) CraftInputScript(signer input.Signer, txn *wire.MsgTx,
// 	hashCache *txscript.TxSigHashes, txinIdx int) (*input.Script, error) {

// 	// First, we ensure that the witness generation function has been
// 	// initialized for this breached output.
// 	bo.witnessFunc = bo.witnessType.WitnessGenerator(signer, bo.SignDesc())

// 	// Now that we have ensured that the witness generation function has
// 	// been initialized, we can proceed to execute it and generate the
// 	// witness for this particular breached output.
// 	return bo.witnessFunc(txn, hashCache, txinIdx)
// }

// // BlocksToMaturity returns the relative timelock, as a number of blocks, that
// // must be built on top of the confirmation height before the output can be
// // spent.
// func (bo *zkBreachedOutput) BlocksToMaturity() uint32 {
// 	return 0
// }

// // HeightHint returns the minimum height at which a confirmed spending tx can
// // occur.
// func (bo *zkBreachedOutput) HeightHint() uint32 {
// 	return bo.confHeight
// }

// // Add compile-time constraint ensuring zkBreachedOutput implements the Input
// // interface.
// var _ input.Input = (*zkBreachedOutput)(nil)

// // zkretributionInfo encapsulates all the data needed to sweep all the contested
// // funds within a channel whose contract has been breached by the prior
// // counterparty. This struct is used to create the zkjustice transaction which
// // spends all outputs of the commitment transaction into an output controlled
// // by the wallet.
// type zkretributionInfo struct {
// 	commitHash   chainhash.Hash
// 	chanPoint    wire.OutPoint
// 	chainHash    chainhash.Hash
// 	breachHeight uint32

// 	zkBreachedOutputs []zkBreachedOutput
// }

// // newZkRetributionInfo constructs a zkretributionInfo containing all the
// // information required by the zk breach arbiter to recover funds from breached
// // channels.  The information is primarily populated using the BreachRetribution
// // delivered by the wallet when it detects a channel breach.
// func newZkRetributionInfo(chanPoint *wire.OutPoint,
// 	breachInfo *lnwallet.BreachRetribution) *zkretributionInfo {

// 	// Determine the number of second layer HTLCs we will attempt to sweep.
// 	nHtlcs := len(breachInfo.HtlcRetributions)

// 	// Initialize a slice to hold the outputs we will attempt to sweep. The
// 	// maximum capacity of the slice is set to 2+nHtlcs to handle the case
// 	// where the local, remote, and all HTLCs are not dust outputs.  All
// 	// HTLC outputs provided by the wallet are guaranteed to be non-dust,
// 	// though the commitment outputs are conditionally added depending on
// 	// the nil-ness of their sign descriptors.
// 	zkBreachedOutputs := make([]zkBreachedOutput, 0, nHtlcs+2)

// 	// First, record the breach information for the local channel point if
// 	// it is not considered dust, which is signaled by a non-nil sign
// 	// descriptor. Here we use CommitmentNoDelay (or
// 	// CommitmentNoDelayTweakless for newer commitments) since this output
// 	// belongs to us and has no time-based constraints on spending.
// 	if breachInfo.LocalOutputSignDesc != nil {
// 		witnessType := input.CommitmentNoDelay
// 		if breachInfo.LocalOutputSignDesc.SingleTweak == nil {
// 			witnessType = input.CommitSpendNoDelayTweakless
// 		}

// 		localOutput := makeZkBreachedOutput(
// 			&breachInfo.LocalOutpoint,
// 			witnessType,
// 			// No second level script as this is a commitment
// 			// output.
// 			nil,
// 			breachInfo.LocalOutputSignDesc,
// 			breachInfo.BreachHeight,
// 		)

// 		zkBreachedOutputs = append(zkBreachedOutputs, localOutput)
// 	}

// 	// Second, record the same information regarding the remote outpoint,
// 	// again if it is not dust, which belongs to the party who tried to
// 	// steal our money! Here we set witnessType of the zkBreachedOutput to
// 	// CommitmentRevoke, since we will be using a revoke key, withdrawing
// 	// the funds from the commitment transaction immediately.
// 	if breachInfo.RemoteOutputSignDesc != nil {
// 		remoteOutput := makeZkBreachedOutput(
// 			&breachInfo.RemoteOutpoint,
// 			input.CommitmentRevoke,
// 			// No second level script as this is a commitment
// 			// output.
// 			nil,
// 			breachInfo.RemoteOutputSignDesc,
// 			breachInfo.BreachHeight,
// 		)

// 		zkBreachedOutputs = append(zkBreachedOutputs, remoteOutput)
// 	}

// 	// Lastly, for each of the breached HTLC outputs, record each as a
// 	// breached output with the appropriate witness type based on its
// 	// directionality. All HTLC outputs provided by the wallet are assumed
// 	// to be non-dust.
// 	for i, breachedHtlc := range breachInfo.HtlcRetributions {
// 		// Using the breachedHtlc's incoming flag, determine the
// 		// appropriate witness type that needs to be generated in order
// 		// to sweep the HTLC output.
// 		var htlcWitnessType input.StandardWitnessType
// 		if breachedHtlc.IsIncoming {
// 			htlcWitnessType = input.HtlcAcceptedRevoke
// 		} else {
// 			htlcWitnessType = input.HtlcOfferedRevoke
// 		}

// 		htlcOutput := makeZkBreachedOutput(
// 			&breachInfo.HtlcRetributions[i].OutPoint,
// 			htlcWitnessType,
// 			breachInfo.HtlcRetributions[i].SecondLevelWitnessScript,
// 			&breachInfo.HtlcRetributions[i].SignDesc,
// 			breachInfo.BreachHeight)

// 		zkBreachedOutputs = append(zkBreachedOutputs, htlcOutput)
// 	}

// 	return &zkretributionInfo{
// 		commitHash:        breachInfo.BreachTransaction.TxHash(),
// 		chainHash:         breachInfo.ChainHash,
// 		chanPoint:         *chanPoint,
// 		zkBreachedOutputs: zkBreachedOutputs,
// 		breachHeight:      breachInfo.BreachHeight,
// 	}
// }

// // createZkJusticeTx creates a transaction which exacts "zkjustice" by sweeping ALL
// // the funds within the channel which we are now entitled to due to a breach of
// // the channel's contract by the counterparty. This function returns a *fully*
// // signed transaction with the witness for each input fully in place.
// func (b *zkBreachArbiter) createZkJusticeTx(
// 	r *zkretributionInfo) (*wire.MsgTx, error) {

// 	// We will assemble the breached outputs into a slice of spendable
// 	// outputs, while simultaneously computing the estimated weight of the
// 	// transaction.
// 	var (
// 		spendableOutputs []input.Input
// 		weightEstimate   input.TxWeightEstimator
// 	)

// 	// Allocate enough space to potentially hold each of the breached
// 	// outputs in the zkretribution info.
// 	spendableOutputs = make([]input.Input, 0, len(r.zkBreachedOutputs))

// 	// The zkjustice transaction we construct will be a segwit transaction
// 	// that pays to a p2wkh output. Components such as the version,
// 	// nLockTime, and output are already included in the TxWeightEstimator.
// 	weightEstimate.AddP2WKHOutput()

// 	// Next, we iterate over the breached outputs contained in the
// 	// zkretribution info.  For each, we switch over the witness type such
// 	// that we contribute the appropriate weight for each input and witness,
// 	// finally adding to our list of spendable outputs.
// 	for i := range r.zkBreachedOutputs {
// 		// Grab locally scoped reference to breached output.
// 		inp := &r.zkBreachedOutputs[i]

// 		// First, determine the appropriate estimated witness weight for
// 		// the give witness type of this breached output. If the witness
// 		// weight cannot be estimated, we will omit it from the
// 		// transaction.
// 		witnessWeight, _, err := inp.WitnessType().SizeUpperBound()
// 		if err != nil {
// 			zkbaLog.Warnf("could not determine witness weight "+
// 				"for breached output in zkretribution info: %v",
// 				err)
// 			continue
// 		}
// 		weightEstimate.AddWitnessInput(witnessWeight)

// 		// Finally, append this input to our list of spendable outputs.
// 		spendableOutputs = append(spendableOutputs, inp)
// 	}

// 	txWeight := int64(weightEstimate.Weight())
// 	return b.sweepSpendableOutputsTxn(txWeight, spendableOutputs...)
// }

// // sweepSpendableOutputsTxn creates a signed transaction from a sequence of
// // spendable outputs by sweeping the funds into a single p2wkh output.
// func (b *zkBreachArbiter) sweepSpendableOutputsTxn(txWeight int64,
// 	inputs ...input.Input) (*wire.MsgTx, error) {

// 	// First, we obtain a new public key script from the wallet which we'll
// 	// sweep the funds to.
// 	// TODO(roasbeef): possibly create many outputs to minimize change in
// 	// the future?
// 	pkScript, err := b.cfg.GenSweepScript()
// 	if err != nil {
// 		return nil, err
// 	}

// 	// Compute the total amount contained in the inputs.
// 	var totalAmt btcutil.Amount
// 	for _, input := range inputs {
// 		totalAmt += btcutil.Amount(input.SignDesc().Output.Value)
// 	}

// 	// We'll actually attempt to target inclusion within the next two
// 	// blocks as we'd like to sweep these funds back into our wallet ASAP.
// 	feePerKw, err := b.cfg.Estimator.EstimateFeePerKW(2)
// 	if err != nil {
// 		return nil, err
// 	}
// 	txFee := feePerKw.FeeForWeight(txWeight)

// 	// TODO(roasbeef): already start to siphon their funds into fees
// 	sweepAmt := int64(totalAmt - txFee)

// 	// With the fee calculated, we can now create the transaction using the
// 	// information gathered above and the provided zkretribution information.
// 	txn := wire.NewMsgTx(2)

// 	// We begin by adding the output to which our funds will be deposited.
// 	txn.AddTxOut(&wire.TxOut{
// 		PkScript: pkScript,
// 		Value:    sweepAmt,
// 	})

// 	// Next, we add all of the spendable outputs as inputs to the
// 	// transaction.
// 	for _, input := range inputs {
// 		txn.AddTxIn(&wire.TxIn{
// 			PreviousOutPoint: *input.OutPoint(),
// 		})
// 	}

// 	// Before signing the transaction, check to ensure that it meets some
// 	// basic validity requirements.
// 	btx := btcutil.NewTx(txn)
// 	if err := blockchain.CheckTransactionSanity(btx); err != nil {
// 		return nil, err
// 	}

// 	// Create a sighash cache to improve the performance of hashing and
// 	// signing SigHashAll inputs.
// 	hashCache := txscript.NewTxSigHashes(txn)

// 	// Create a closure that encapsulates the process of initializing a
// 	// particular output's witness generation function, computing the
// 	// witness, and attaching it to the transaction. This function accepts
// 	// an integer index representing the intended txin index, and the
// 	// breached output from which it will spend.
// 	addWitness := func(idx int, so input.Input) error {
// 		// First, we construct a valid witness for this outpoint and
// 		// transaction using the SpendableOutput's witness generation
// 		// function.
// 		inputScript, err := so.CraftInputScript(
// 			b.cfg.Signer, txn, hashCache, idx,
// 		)
// 		if err != nil {
// 			return err
// 		}

// 		// Then, we add the witness to the transaction at the
// 		// appropriate txin index.
// 		txn.TxIn[idx].Witness = inputScript.Witness

// 		return nil
// 	}

// 	// Finally, generate a witness for each output and attach it to the
// 	// transaction.
// 	for i, input := range inputs {
// 		if err := addWitness(i, input); err != nil {
// 			return nil, err
// 		}
// 	}

// 	return txn, nil
// }

// // ZkRetributionStore provides an interface for managing a persistent map from
// // wire.OutPoint -> zkretributionInfo. Upon learning of a breach, a ZkBreachArbiter
// // should record the zkretributionInfo for the breached channel, which serves a
// // checkpoint in the event that zkretribution needs to be resumed after failure.
// // A ZkRetributionStore provides an interface for managing the persisted set, as
// // well as mapping user defined functions over the entire on-disk contents.
// //
// // Calls to ZkRetributionStore may occur concurrently. A concrete instance of
// // ZkRetributionStore should use appropriate synchronization primitives, or
// // be otherwise safe for concurrent access.
// type ZkRetributionStore interface {
// 	// Add persists the zkretributionInfo to disk, using the information's
// 	// chanPoint as the key. This method should overwrite any existing
// 	// entries found under the same key, and an error should be raised if
// 	// the addition fails.
// 	Add(retInfo *zkretributionInfo) error

// 	// IsBreached queries the zkretribution store to see if the zk breach arbiter
// 	// is aware of any breaches for the provided channel point.
// 	IsBreached(chanPoint *wire.OutPoint) (bool, error)

// 	// Finalize persists the finalized zkjustice transaction for a particular
// 	// channel.
// 	Finalize(chanPoint *wire.OutPoint, finalTx *wire.MsgTx) error

// 	// GetFinalizedTxn loads the finalized zkjustice transaction, if any, from
// 	// the zkretribution store. The finalized transaction will be nil if
// 	// Finalize has not yet been called for this channel point.
// 	GetFinalizedTxn(chanPoint *wire.OutPoint) (*wire.MsgTx, error)

// 	// Remove deletes the zkretributionInfo from disk, if any exists, under
// 	// the given key. An error should be re raised if the removal fails.
// 	Remove(key *wire.OutPoint) error

// 	// ForAll iterates over the existing on-disk contents and applies a
// 	// chosen, read-only callback to each. This method should ensure that it
// 	// immediately propagate any errors generated by the callback.
// 	ForAll(cb func(*zkretributionInfo) error) error
// }

// // zkretributionStore handles persistence of zkretribution states to disk and is
// // backed by a boltdb bucket. The primary responsibility of the zkretribution
// // store is to ensure that we can recover from a restart in the middle of a
// // breached contract zkretribution.
// type zkretributionStore struct {
// 	db *channeldb.DB
// }

// // newZkRetributionStore creates a new instance of a zkretributionStore.
// func newZkRetributionStore(db *channeldb.DB) *zkretributionStore {
// 	return &zkretributionStore{
// 		db: db,
// 	}
// }

// // Add adds a zkretribution state to the zkretributionStore, which is then persisted
// // to disk.
// func (rs *zkretributionStore) Add(ret *zkretributionInfo) error {
// 	return rs.db.Update(func(tx *bbolt.Tx) error {
// 		// If this is our first contract breach, the zkretributionBucket
// 		// won't exist, in which case, we just create a new bucket.
// 		retBucket, err := tx.CreateBucketIfNotExists(zkretributionBucket)
// 		if err != nil {
// 			return err
// 		}

// 		var outBuf bytes.Buffer
// 		if err := writeOutpoint(&outBuf, &ret.chanPoint); err != nil {
// 			return err
// 		}

// 		var retBuf bytes.Buffer
// 		if err := ret.Encode(&retBuf); err != nil {
// 			return err
// 		}

// 		return retBucket.Put(outBuf.Bytes(), retBuf.Bytes())
// 	})
// }

// // Finalize writes a signed zkjustice transaction to the zkretribution store. This
// // is done before publishing the transaction, so that we can recover the txid on
// // startup and re-register for confirmation notifications.
// func (rs *zkretributionStore) Finalize(chanPoint *wire.OutPoint,
// 	finalTx *wire.MsgTx) error {
// 	return rs.db.Update(func(tx *bbolt.Tx) error {
// 		zkjusticeBkt, err := tx.CreateBucketIfNotExists(zkjusticeTxnBucket)
// 		if err != nil {
// 			return err
// 		}

// 		var chanBuf bytes.Buffer
// 		if err := writeOutpoint(&chanBuf, chanPoint); err != nil {
// 			return err
// 		}

// 		var txBuf bytes.Buffer
// 		if err := finalTx.Serialize(&txBuf); err != nil {
// 			return err
// 		}

// 		return zkjusticeBkt.Put(chanBuf.Bytes(), txBuf.Bytes())
// 	})
// }

// // GetFinalizedTxn loads the finalized zkjustice transaction for the provided
// // channel point. The finalized transaction will be nil if Finalize has yet to
// // be called for this channel point.
// func (rs *zkretributionStore) GetFinalizedTxn(
// 	chanPoint *wire.OutPoint) (*wire.MsgTx, error) {

// 	var finalTxBytes []byte
// 	if err := rs.db.View(func(tx *bbolt.Tx) error {
// 		zkjusticeBkt := tx.Bucket(zkjusticeTxnBucket)
// 		if zkjusticeBkt == nil {
// 			return nil
// 		}

// 		var chanBuf bytes.Buffer
// 		if err := writeOutpoint(&chanBuf, chanPoint); err != nil {
// 			return err
// 		}

// 		finalTxBytes = zkjusticeBkt.Get(chanBuf.Bytes())

// 		return nil
// 	}); err != nil {
// 		return nil, err
// 	}

// 	if finalTxBytes == nil {
// 		return nil, nil
// 	}

// 	finalTx := &wire.MsgTx{}
// 	err := finalTx.Deserialize(bytes.NewReader(finalTxBytes))

// 	return finalTx, err
// }

// // IsBreached queries the zkretribution store to discern if this channel was
// // previously breached. This is used when connecting to a peer to determine if
// // it is safe to add a link to the htlcswitch, as we should never add a channel
// // that has already been breached.
// func (rs *zkretributionStore) IsBreached(chanPoint *wire.OutPoint) (bool, error) {
// 	var found bool
// 	err := rs.db.View(func(tx *bbolt.Tx) error {
// 		retBucket := tx.Bucket(zkretributionBucket)
// 		if retBucket == nil {
// 			return nil
// 		}

// 		var chanBuf bytes.Buffer
// 		if err := writeOutpoint(&chanBuf, chanPoint); err != nil {
// 			return err
// 		}

// 		retInfo := retBucket.Get(chanBuf.Bytes())
// 		if retInfo != nil {
// 			found = true
// 		}

// 		return nil
// 	})

// 	return found, err
// }

// // Remove removes a zkretribution state and finalized zkjustice transaction by
// // channel point  from the zkretribution store.
// func (rs *zkretributionStore) Remove(chanPoint *wire.OutPoint) error {
// 	return rs.db.Update(func(tx *bbolt.Tx) error {
// 		retBucket := tx.Bucket(zkretributionBucket)

// 		// We return an error if the bucket is not already created,
// 		// since normal operation of the zk breach arbiter should never try
// 		// to remove a finalized zkretribution state that is not already
// 		// stored in the db.
// 		if retBucket == nil {
// 			return errors.New("Unable to remove zkretribution " +
// 				"because the zkretribution bucket doesn't exist.")
// 		}

// 		// Serialize the channel point we are intending to remove.
// 		var chanBuf bytes.Buffer
// 		if err := writeOutpoint(&chanBuf, chanPoint); err != nil {
// 			return err
// 		}
// 		chanBytes := chanBuf.Bytes()

// 		// Remove the persisted zkretribution info and finalized zkjustice
// 		// transaction.
// 		if err := retBucket.Delete(chanBytes); err != nil {
// 			return err
// 		}

// 		// If we have not finalized this channel breach, we can exit
// 		// early.
// 		zkjusticeBkt := tx.Bucket(zkjusticeTxnBucket)
// 		if zkjusticeBkt == nil {
// 			return nil
// 		}

// 		return zkjusticeBkt.Delete(chanBytes)
// 	})
// }

// // ForAll iterates through all stored zkretributions and executes the passed
// // callback function on each zkretribution.
// func (rs *zkretributionStore) ForAll(cb func(*zkretributionInfo) error) error {
// 	return rs.db.View(func(tx *bbolt.Tx) error {
// 		// If the bucket does not exist, then there are no pending
// 		// zkretributions.
// 		retBucket := tx.Bucket(zkretributionBucket)
// 		if retBucket == nil {
// 			return nil
// 		}

// 		// Otherwise, we fetch each serialized zkretribution info,
// 		// deserialize it, and execute the passed in callback function
// 		// on it.
// 		return retBucket.ForEach(func(_, retBytes []byte) error {
// 			ret := &zkretributionInfo{}
// 			err := ret.Decode(bytes.NewBuffer(retBytes))
// 			if err != nil {
// 				return err
// 			}

// 			return cb(ret)
// 		})
// 	})
// }

// // Encode serializes the zkretribution into the passed byte stream.
// func (ret *zkretributionInfo) Encode(w io.Writer) error {
// 	var scratch [4]byte

// 	if _, err := w.Write(ret.commitHash[:]); err != nil {
// 		return err
// 	}

// 	if err := writeOutpoint(w, &ret.chanPoint); err != nil {
// 		return err
// 	}

// 	if _, err := w.Write(ret.chainHash[:]); err != nil {
// 		return err
// 	}

// 	binary.BigEndian.PutUint32(scratch[:], ret.breachHeight)
// 	if _, err := w.Write(scratch[:]); err != nil {
// 		return err
// 	}

// 	nOutputs := len(ret.zkBreachedOutputs)
// 	if err := wire.WriteVarInt(w, 0, uint64(nOutputs)); err != nil {
// 		return err
// 	}

// 	for _, output := range ret.zkBreachedOutputs {
// 		if err := output.Encode(w); err != nil {
// 			return err
// 		}
// 	}

// 	return nil
// }

// // Dencode deserializes a zkretribution from the passed byte stream.
// func (ret *zkretributionInfo) Decode(r io.Reader) error {
// 	var scratch [32]byte

// 	if _, err := io.ReadFull(r, scratch[:]); err != nil {
// 		return err
// 	}
// 	hash, err := chainhash.NewHash(scratch[:])
// 	if err != nil {
// 		return err
// 	}
// 	ret.commitHash = *hash

// 	if err := readOutpoint(r, &ret.chanPoint); err != nil {
// 		return err
// 	}

// 	if _, err := io.ReadFull(r, scratch[:]); err != nil {
// 		return err
// 	}
// 	chainHash, err := chainhash.NewHash(scratch[:])
// 	if err != nil {
// 		return err
// 	}
// 	ret.chainHash = *chainHash

// 	if _, err := io.ReadFull(r, scratch[:4]); err != nil {
// 		return err
// 	}
// 	ret.breachHeight = binary.BigEndian.Uint32(scratch[:4])

// 	nOutputsU64, err := wire.ReadVarInt(r, 0)
// 	if err != nil {
// 		return err
// 	}
// 	nOutputs := int(nOutputsU64)

// 	ret.zkBreachedOutputs = make([]zkBreachedOutput, nOutputs)
// 	for i := range ret.zkBreachedOutputs {
// 		if err := ret.zkBreachedOutputs[i].Decode(r); err != nil {
// 			return err
// 		}
// 	}

// 	return nil
// }

// // Encode serializes a zkBreachedOutput into the passed byte stream.
// func (bo *zkBreachedOutput) Encode(w io.Writer) error {
// 	var scratch [8]byte

// 	binary.BigEndian.PutUint64(scratch[:8], uint64(bo.amt))
// 	if _, err := w.Write(scratch[:8]); err != nil {
// 		return err
// 	}

// 	if err := writeOutpoint(w, &bo.outpoint); err != nil {
// 		return err
// 	}

// 	err := input.WriteSignDescriptor(w, &bo.signDesc)
// 	if err != nil {
// 		return err
// 	}

// 	err = wire.WriteVarBytes(w, 0, bo.secondLevelWitnessScript)
// 	if err != nil {
// 		return err
// 	}

// 	binary.BigEndian.PutUint16(scratch[:2], uint16(bo.witnessType))
// 	if _, err := w.Write(scratch[:2]); err != nil {
// 		return err
// 	}

// 	return nil
// }

// // Decode deserializes a zkBreachedOutput from the passed byte stream.
// func (bo *zkBreachedOutput) Decode(r io.Reader) error {
// 	var scratch [8]byte

// 	if _, err := io.ReadFull(r, scratch[:8]); err != nil {
// 		return err
// 	}
// 	bo.amt = btcutil.Amount(binary.BigEndian.Uint64(scratch[:8]))

// 	if err := readOutpoint(r, &bo.outpoint); err != nil {
// 		return err
// 	}

// 	if err := input.ReadSignDescriptor(r, &bo.signDesc); err != nil {
// 		return err
// 	}

// 	wScript, err := wire.ReadVarBytes(r, 0, 1000, "witness script")
// 	if err != nil {
// 		return err
// 	}
// 	bo.secondLevelWitnessScript = wScript

// 	if _, err := io.ReadFull(r, scratch[:2]); err != nil {
// 		return err
// 	}
// 	bo.witnessType = input.StandardWitnessType(
// 		binary.BigEndian.Uint16(scratch[:2]),
// 	)

// 	return nil
// }
