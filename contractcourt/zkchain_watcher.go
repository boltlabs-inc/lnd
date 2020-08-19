package contractcourt

import (
	"encoding/hex"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/lightningnetwork/lnd/chainntnfs"
	"github.com/lightningnetwork/lnd/libzkchannels"
	"github.com/lightningnetwork/lnd/lnwallet"
	"github.com/lightningnetwork/lnd/lnwallet/chainfee"
	"github.com/lightningnetwork/lnd/zkchanneldb"
	"github.com/lightningnetwork/lnd/zkchannels"
)

var (
	merchClaimTxKW = 0.520 // 520 weight units
	custClaimTxKW  = 0.518 // 518 weight units
)

// ZkMerchCloseInfo provides the information needed for the customer to retrieve
// the relevant CloseMerch transaction for that channel.
type ZkMerchCloseInfo struct {
	escrowTxid     chainhash.Hash
	merchCloseTxid chainhash.Hash
	pkScript       []byte
	amount         int64
}

// ZkMerchClaimInfo provides details about the merchClaim tx.
type ZkMerchClaimInfo struct {
	merchCloseTxid chainhash.Hash
	merchClaimTxid chainhash.Hash
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

	// ZkMerchClaim is a channel that will be sent upon in the
	// event that the Merchant's Claim transaction is confirmed.
	ZkMerchClaim chan *ZkMerchClaimInfo

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

	// WatchingMerchClose is true if ZkChainWatcher is watching a merchCloseTx
	// as opposed to an escrowTx
	WatchingMerchClose bool

	// CustChannelName is the name the customer gives to the channel.
	// For the merchant, this field will be left blank
	CustChannelName string

	// DBPath is the path to the customer or merchant's DB.
	DBPath string

	// Estimator is used by the zk breach arbiter to determine an appropriate
	// fee level when generating, signing, and broadcasting sweep
	// transactions.
	Estimator chainfee.Estimator

	// notifier is a reference to the channel notifier that we'll use to be
	// notified of output spends and when transactions are confirmed.
	Notifier chainntnfs.ChainNotifier

	// // signer is the main signer instances that will be responsible for
	// // signing any HTLC and commitment transaction generated by the state
	// // machine.
	// signer input.Signer

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

	// // isOurAddr is a function that returns true if the passed address is
	// // known to us.
	// isOurAddr func(btcutil.Address) bool

	// // extractStateNumHint extracts the encoded state hint using the passed
	// // obfuscater. This is used by the chain watcher to identify which
	// // state was broadcast and confirmed on-chain.
	// extractStateNumHint func(*wire.MsgTx, [lnwallet.StateHintSize]byte) uint64
}

// ZkFundingInfo contains funding outpoint, pkscript and, confirmation blockheight
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

	// // stateHintObfuscator is a 48-bit state hint that's used to obfuscate
	// // the current state number on the commitment transactions.
	// stateHintObfuscator [lnwallet.StateHintSize]byte

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

	log.Debug("Setting up new zkChainWatcher")
	return &zkChainWatcher{
		cfg:                 cfg,
		quit:                make(chan struct{}),
		clientSubscriptions: make(map[uint64]*ZkChainEventSubscription),
	}, nil
}

// Start starts all goroutines that the zkChainWatcher needs to perform its
// duties.
func (c *zkChainWatcher) Start() error {
	log.Debug("Starting new zkChainWatcher")

	// // zkch TODO: What does this do? It is blocking
	// if !atomic.CompareAndSwapInt32(&c.started, 0, 1) {
	// 	return nil
	// }

	log.Debugf("watching for escrow: %#v\n", c.cfg.ZkFundingInfo.FundingOut.Hash.String())

	spendNtfn, err := c.cfg.Notifier.RegisterSpendNtfn(
		&c.cfg.ZkFundingInfo.FundingOut,
		c.cfg.ZkFundingInfo.PkScript,
		c.cfg.ZkFundingInfo.BroadcastHeight,
	)
	if err != nil {
		log.Debug("zkchainwatcher err:", err)
		return err
	}
	log.Debug("spend notifier finished")

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
		ZkMerchClaim:            make(chan *ZkMerchClaimInfo, 1),
		ZkCustClosure:           make(chan *ZkCustCloseInfo, 1),
		ZkContractBreach:        make(chan *ZkBreachInfo, 1),
		LocalUnilateralClosure:  make(chan *LocalUnilateralCloseInfo, 1),
		CooperativeClosure:      make(chan *CooperativeCloseInfo, 1),
		ContractBreach:          make(chan *lnwallet.BreachRetribution, 1),
		Cancel: func() {
			c.Lock()
			delete(c.clientSubscriptions, clientID)
			c.Unlock()
		},
	}

	c.Lock()
	c.clientSubscriptions[clientID] = sub
	c.Unlock()

	return sub
}

// zkCloseObserver is a dedicated goroutine that will watch for any closes of the
// channel that it's watching on chain. In the event of an on-chain event, the
// close observer will assembled the proper materials required to claim the
// funds of the channel on-chain (if required), then dispatch these as
// notifications to all subscribers.
func (c *zkChainWatcher) zkCloseObserver(spendNtfn *chainntnfs.SpendEvent) {
	defer c.wg.Done()

	log.Debug("zkCloseObserver is running")
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

		if c.cfg.WatchingMerchClose {
			log.Debug("Spend from merchClose detected")
		} else {
			log.Debug("Spend from escrow detected")
		}

		inputTxid := commitSpend.SpentOutPoint.Hash
		commitTxBroadcast := commitSpend.SpendingTx
		closeTxid := *commitSpend.SpenderTxHash
		spendHeight := commitSpend.SpendingHeight
		numOutputs := len(commitTxBroadcast.TxOut)
		amount := commitTxBroadcast.TxOut[0].Value
		sequence := commitSpend.SpendingTx.TxIn[0].Sequence

		// Identify what kind of tx spent from the escrowTx/merchCloseTx
		isMerchCloseTx := numOutputs == 2 // merchCloseTx has two outputs.
		isCustCloseTx := numOutputs == 4  // custCloseTx has four outputs.

		isMerchClaimTx := sequence != uint32(4294967295) && numOutputs == 1 // merchClaim can only be spent after waiting for the relative timelock

		switch {

		case isMerchClaimTx:

			log.Debug("Merch-claim-tx detected")

			// commitTxBroadcast.TxOut will always have at least one
			// output since it was confirmed on chain.
			closePkScript := commitTxBroadcast.TxOut[0].PkScript

			err := c.zkDispatchMerchClaim(inputTxid, closeTxid, closePkScript, amount)
			if err != nil {
				log.Errorf("unable to handle remote "+
					"close for channel=%v",
					inputTxid, err)
			}

		case isMerchCloseTx && isMerch:
			log.Debug("Merch-close-tx detected")

			// commitTxBroadcast.TxOut will always have at least one
			// output since it was confirmed on chain.
			closePkScript := commitTxBroadcast.TxOut[0].PkScript

			// escrow script is the 4th item in the witness field.
			// Check that there are 4 items in the witness field
			if len(commitTxBroadcast.TxIn[0].Witness) < 4 {
				log.Error("Merch-close Witness field has less than 4 items")
				return
			}
			escrowScript := commitTxBroadcast.TxIn[0].Witness[3]

			// custPubkey starts on the 37th byte, and finishes on the
			// 69th byte of the escrowScript
			if len(escrowScript) != 71 {
				log.Errorf("escrowScript in merch-close is not 71 bytes as expected."+
					"escrowScript: %x", escrowScript)
				return
			}
			custPubkey := hex.EncodeToString(escrowScript[36:69])
			log.Debug("custPubkey read from merch-close-tx: ", custPubkey)

			// save custClose details so that it can be claimed manually with cli command
			err := c.storeMerchClaimTx(inputTxid.String(), closeTxid.String(), custPubkey,
				amount, spendHeight)
			if err != nil {
				log.Errorf("Unable to store MerchClaimTx for channel %v ",
					inputTxid, err)
			}

			err = c.zkDispatchMerchClose(inputTxid, closeTxid, closePkScript, amount)
			if err != nil {
				log.Errorf("unable to handle remote "+
					"close for channel=%v",
					inputTxid, err)
			}

		case isMerchCloseTx && !isMerch:
			log.Debug("Merch-close-tx detected")

			// Mark the channel as pending close to prevent further payments on it
			err := zkchannels.UpdateCustChannelState(c.cfg.DBPath, c.cfg.CustChannelName, "PendingClose")
			if err != nil {
				log.Error(err)
			}
			pkScript := commitTxBroadcast.TxOut[0].PkScript

			zkBreachInfo := ZkBreachInfo{
				IsMerchClose:    true,
				EscrowTxid:      inputTxid,
				CloseTxid:       closeTxid,
				ClosePkScript:   pkScript,
				CustChannelName: c.cfg.CustChannelName,
				// RevLock:       revLock,
				// RevSecret:     revSecret,
				// CustClosePk:   custClosePk,
				Amount: amount,
			}

			err = c.zkDispatchBreach(zkBreachInfo)

			if err != nil {
				log.Errorf("unable to handle remote "+
					"close for channel=%v",
					inputTxid, err)
			}

		// custCloseTx has 4 outputs.
		case isCustCloseTx:

			ClosePkScript := commitTxBroadcast.TxOut[0].PkScript

			opreturnScript := commitTxBroadcast.TxOut[2].PkScript
			revLockBytes := opreturnScript[2:34]
			revLock := hex.EncodeToString(revLockBytes)

			custClosePkBytes := opreturnScript[34:67]
			custClosePk := hex.EncodeToString(custClosePkBytes)

			// If the user is a customer and they broadcasted custClose,
			// there is nothing more to do.
			if !isMerch {

				log.Debug("storeCustClaimTx")
				// save custClose details so that it can be claimed manually with cli command
				err := c.storeCustClaimTx(inputTxid.String(), closeTxid.String(), revLock, custClosePk, amount, spendHeight)
				if err != nil {
					log.Errorf("Unable to store CustClaimTx for channel %v ",
						inputTxid, err)
				}

				log.Debug("zkDispatchCustClose")
				err = c.zkDispatchCustClose(inputTxid, closeTxid, ClosePkScript, revLock, custClosePk, amount)

				if err != nil {
					log.Errorf("unable to handle remote "+
						"custClose for channel=%v",
						inputTxid, err)
				}
			}
			// if the closeTx has more than 2 outputs, it is a custCloseTx. Now we
			// check to see if custCloseTx corresponds to a revoked state by
			// checking if we have seen the revocation lock before.
			if isMerch {

				// Mark the channel as pending close to prevent further payments on it
				err := zkchannels.UpdateMerchChannelState(c.cfg.DBPath, inputTxid.String(), "PendingClose")
				if err != nil {
					log.Error(err)
				}
				// open the zkchanneldb to load merchState and channelState
				zkMerchDB, err := zkchanneldb.OpenMerchBucket("zkmerch.db")
				if err != nil {
					log.Error(err)
					return
				}

				merchState, err := zkchanneldb.GetMerchState(zkMerchDB)
				if err != nil {
					log.Error(err)
					return
				}
				zkMerchDB.Close()

				isOldRevLock, revSecret, err := libzkchannels.MerchantCheckRevLock(revLock, merchState)

				// The Revocation Lock in the custCloseTx corresponds to an old
				// state. We must send the relevant transaction information
				// to the breachArbiter to broadcast the dispute/justice tx.
				if isOldRevLock {
					log.Debug("Revoked Cust-close-tx detected")

					zkBreachInfo := ZkBreachInfo{
						IsMerchClose:  false,
						EscrowTxid:    inputTxid,
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
							inputTxid, err)
					}
					// The Revocation Lock has not been seen before, therefore it
					// corresponds to the customer's latest state. We should
					// store the Revocation Lock to prevent a customer making
					// a payment with it.
				} else {
					log.Debug("Latest Cust-close-tx detected")

					err := c.zkDispatchCustClose(inputTxid, closeTxid, ClosePkScript, revLock, custClosePk, amount)

					if err != nil {
						log.Errorf("unable to handle remote "+
							"custClose for channel=%v",
							inputTxid, err)
					}
				}
				if err != nil {
					log.Errorf("unable to handle remote "+
						"close for channel=%v",
						inputTxid, err)
				}

			}

		default:
			log.Errorf("unable to determine the type of close tx "+
				"spending from txid=%v",
				inputTxid)

		}

		// Now that a spend has been detected, we've done our job, so
		// we'll exit immediately.
		return

	// The zkChainWatcher has been signalled to exit, so we'll do so now.
	case <-c.quit:
		return
	}
}

// storeMerchClaimTx creates a signed merchClaimTx for the given closeTx
func (c *zkChainWatcher) storeMerchClaimTx(escrowTxidLittleEn string, closeTxidLittleEn string,
	custClosePk string, amount int64, spendHeight int32) error {

	log.Debugf("storeMerchClaimTx inputs: ", escrowTxidLittleEn, closeTxidLittleEn,
		custClosePk, amount, spendHeight)

	// Load the current merchState and channelState so that it can be retrieved
	// later when it is needed to sign the claim tx.
	zkMerchDB, err := zkchanneldb.OpenMerchBucket("zkmerch.db")
	if err != nil {
		log.Error(err)
		return err
	}

	merchState, err := zkchanneldb.GetMerchState(zkMerchDB)
	if err != nil {
		log.Error(err)
		return err
	}

	var channelState libzkchannels.ChannelState
	err = zkchanneldb.GetMerchField(zkMerchDB, "channelStateKey", &channelState)
	if err != nil {
		log.Error(err)
		return err
	}

	err = zkMerchDB.Close()
	if err != nil {
		log.Error(err)
		return err
	}

	toSelfDelay, err := libzkchannels.GetSelfDelayBE(channelState)
	if err != nil {
		log.Error(err)
		return err
	}
	log.Debugf("channelState: %#v", channelState)
	log.Debugf("toSelfDelay: %#v", toSelfDelay)

	// TODO: ZKLND-33 Generate a fresh outputPk for the claimed outputs. For now this is just
	// reusing the merchant's public key
	outputPk := *merchState.PkM
	index := uint32(0)

	log.Debugf("closeTxidLittleEn: %#v", closeTxidLittleEn)
	log.Debugf("index: %#v", index)
	log.Debugf("amount: %#v", amount)
	log.Debugf("toSelfDelay: %#v", toSelfDelay)
	log.Debugf("custClosePk: %#v", custClosePk)
	log.Debugf("outputPk: %#v", outputPk)
	log.Debugf("merchState: %#v", merchState)

	feePerKw, err := c.cfg.Estimator.EstimateFeePerKW(1)
	if err != nil {
		return err
	} // 520 weight units
	txFee := int64(merchClaimTxKW*float64(feePerKw) + 1) // round down to int64\

	inAmt := amount
	outAmt := int64(inAmt - txFee)
	signedMerchClaimTx, err := libzkchannels.MerchantSignMerchClaimTx(closeTxidLittleEn, index, inAmt, outAmt, toSelfDelay, custClosePk, outputPk, 0, 0, merchState)
	if err != nil {
		log.Errorf("libzkchannels.MerchantSignMerchClaimTx: ", err)
		return err
	}

	log.Debugf("signedMerchClaimTx: %#v", signedMerchClaimTx)

	// use escrowTxid as the bucket name
	bucketEscrowTxid := escrowTxidLittleEn

	zkMerchClaimDB, err := zkchanneldb.OpenZkClaimBucket(bucketEscrowTxid, "zkclaim.db")
	if err != nil {
		log.Error(err)
		return err
	}
	err = zkchanneldb.AddStringField(zkMerchClaimDB, bucketEscrowTxid, signedMerchClaimTx, "signedMerchClaimTxKey")
	if err != nil {
		log.Error(err)
		return err
	}
	err = zkchanneldb.AddField(zkMerchClaimDB, bucketEscrowTxid, spendHeight, "spendHeightKey")
	if err != nil {
		log.Error(err)
		return err
	}

	zkMerchClaimDB.Close()

	return nil
}

// storeCustClaimTx creates a signed custClaimTx for the given closeTx
func (c *zkChainWatcher) storeCustClaimTx(escrowTxidLittleEn string, closeTxid string,
	revLock string, custClosePk string, inputAmount int64, spendHeight int32) error {

	log.Debugf("storeCustClaimTx inputs: ", escrowTxidLittleEn, closeTxid,
		revLock, custClosePk, inputAmount, spendHeight)

	channelName := c.cfg.CustChannelName

	// Load the current custState and channelState so that it can be retrieved
	// later when it is needed to sign the claim tx.
	zkCustDB, err := zkchanneldb.OpenZkChannelBucket(channelName, "zkcust.db")
	if err != nil {
		log.Error(err)
		return err
	}

	custState, err := zkchanneldb.GetCustState(zkCustDB, channelName)
	if err != nil {
		log.Error(err)
		return err
	}

	var channelState libzkchannels.ChannelState
	err = zkchanneldb.GetField(zkCustDB, channelName, "channelStateKey", &channelState)
	if err != nil {
		log.Error(err)
		return err
	}

	err = zkCustDB.Close()
	if err != nil {
		log.Error(err)
		return err
	}

	toSelfDelay, err := libzkchannels.GetSelfDelayBE(channelState)

	feePerKw, err := c.cfg.Estimator.EstimateFeePerKW(1)
	if err != nil {
		return err
	}
	txFee := int64(custClaimTxKW*float64(feePerKw) + 1) // round down to int64\

	// TODO: ZKLND-33 Generate a fresh outputPk for the claimed outputs. For now this is just
	// reusing the custClosePk
	outputPk := custClosePk
	index := uint32(0)
	// note that "inputAmount" is taken from the UTXO observed on chain, so
	// feeCC/feeMC have already been factored in
	cpfpIndex := uint32(3)
	cpfpAmount := channelState.ValCpfp
	outAmt := inputAmount + cpfpAmount - txFee
	signedCustClaimWithChild, err := libzkchannels.CustomerSignClaimTx(channelState, closeTxid, index, inputAmount, outAmt, toSelfDelay, outputPk, custState.RevLock, custClosePk, cpfpIndex, cpfpAmount, custState)
	if err != nil {
		log.Error(err)
		return err
	}
	log.Debugf("signedCustClaimWithChild: %#v", signedCustClaimWithChild)

	signedCustClaimWithoutChild, err := libzkchannels.CustomerSignClaimTx(channelState, closeTxid, index, inputAmount, outAmt, toSelfDelay, outputPk, custState.RevLock, custClosePk, uint32(0), int64(0), custState)
	if err != nil {
		log.Error(err)
		return err
	}
	log.Debugf("signedCustClaimWithoutChild: %#v", signedCustClaimWithoutChild)

	// use escrowTxid as the bucket name
	bucketEscrowTxid := escrowTxidLittleEn

	zkCustClaimDB, err := zkchanneldb.OpenZkClaimBucket(bucketEscrowTxid, "zkclaim.db")
	if err != nil {
		log.Error(err)
		return err
	}
	err = zkchanneldb.AddField(zkCustClaimDB, bucketEscrowTxid, signedCustClaimWithChild, "signedCustClaimWithChildKey")
	if err != nil {
		log.Error(err)
		return err
	}
	err = zkchanneldb.AddField(zkCustClaimDB, bucketEscrowTxid, signedCustClaimWithoutChild, "signedCustClaimWithoutChildKey")
	if err != nil {
		log.Error(err)
		return err
	}
	err = zkchanneldb.AddField(zkCustClaimDB, bucketEscrowTxid, spendHeight, "spendHeightKey")
	if err != nil {
		log.Error(err)
		return err
	}

	zkCustClaimDB.Close()

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

// zkDispatchMerchClaim processes a detected claimTx by the Merchant.
// It will return the merchCloseTxid, merchClaimTxid, and the merchClaim pkScript.
func (c *zkChainWatcher) zkDispatchMerchClaim(merchCloseTxid chainhash.Hash,
	merchClaimTxid chainhash.Hash, pkScript []byte, amount int64) error {

	c.Lock()
	for _, sub := range c.clientSubscriptions {
		select {
		case sub.ZkMerchClaim <- &ZkMerchClaimInfo{
			merchCloseTxid: merchCloseTxid,
			merchClaimTxid: merchClaimTxid,
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
	log.Debug("zkDispatchCustBreach, zkBreachInfo %#v:", zkBreachInfo)
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
