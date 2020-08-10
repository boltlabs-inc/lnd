package zkchannels

import (
	"fmt"
	"io/ioutil"
	"log"
	"path"

	"github.com/lightningnetwork/lnd/libzkchannels"
	"github.com/lightningnetwork/lnd/lncfg"
	"github.com/lightningnetwork/lnd/zkchanneldb"
)

var (
	channelStateKey   = "channelStateKey"
	zkChannelsKey     = "zkChannelsKey"
	channelTokenKey   = "channelTokenKey"
	merchPkKey        = "merchPkKey"
	escrowTxidKey     = "escrowTxidKey"
	escrowPrevoutKey  = "escrowPrevoutKey"
	signedEscrowTxKey = "signedEscrowTxKey"
	txFeeInfoKey      = "txFeeInfoKey"
	custBalKey        = "custBalKey"
	merchBalKey       = "merchBalKey"
	feeCCKey          = "feeCCKey"
	feeMCKey          = "feeMCKey"
	minFeeKey         = "minFeeKey"
	maxFeeKey         = "maxFeeKey"
	pkScriptKey       = "pkScriptKey"
	totalReceivedKey  = "totalReceivedKey"
)

const (
	escrowTxid = "43e3075ffc665e085040cfbc0a0f63b4cb43dd95830f6d2c22a78112787c3dfe"
)

func SetupTempDBPaths() (string, string, error) {
	testDir, err := ioutil.TempDir("", "zkchanneldb")
	if err != nil {
		return "", "", err
	}
	custDBPath := path.Join(testDir, "zkcust.db")
	merchDBPath := path.Join(testDir, "zkmerch.db")

	return custDBPath, merchDBPath, nil
}

func SetupLibzkChannels(zkChannelName string, custDBPath string, merchDBPath string) {
	// Load MerchState and ChannelState
	zkMerchDB, err := zkchanneldb.SetupMerchDB(merchDBPath)
	if err != nil {
		log.Fatalf("%v", err)
	}

	merchState, err := zkchanneldb.GetMerchState(zkMerchDB)
	if err != nil {
		log.Fatalf("%v", err)
	}

	var channelState libzkchannels.ChannelState
	err = zkchanneldb.GetMerchField(zkMerchDB, channelStateKey, &channelState)
	if err != nil {
		log.Fatalf("%v", err)
	}

	err = zkMerchDB.Close()
	if err != nil {
		log.Fatalf("%v", err)
	}

	txFeeInfo := libzkchannels.TransactionFeeInfo{
		BalMinCust:  lncfg.BalMinCust,
		BalMinMerch: lncfg.BalMinMerch,
		ValCpFp:     lncfg.ValCpfp,
		FeeCC:       1000,
		FeeMC:       1000,
		MinFee:      lncfg.MinFee,
		MaxFee:      lncfg.MaxFee,
	}
	feeCC := txFeeInfo.FeeCC
	feeMC := txFeeInfo.FeeMC

	custBal := int64(1000000)
	merchBal := int64(5000)

	merchPKM := fmt.Sprintf("%v", *merchState.PkM)

	skC := "1a1971e1379beec67178509e25b6772c66cb67bb04d70df2b4bcdb8c08a01827"
	payoutSk := "4157697b6428532758a9d0f9a73ce58befe3fd665797427d1c5bb3d33f6a132e"

	channelToken, custState, err := libzkchannels.InitCustomer(merchPKM, custBal, merchBal, txFeeInfo, "cust")
	if err != nil {
		log.Fatalf("%v", err)
	}

	channelToken, custState, err = libzkchannels.LoadCustomerWallet(custState, channelToken, skC, payoutSk)
	if err != nil {
		log.Fatalf("%v", err)
	}

	// details about input utxo
	inputSats := int64(100000000)
	cust_utxo_txid := "e8aed42b9f07c74a3ce31a9417146dc61eb8611a1e66d345fd69be06b644278d"
	cust_utxo_index := uint32(0)
	custInputSk := "5511111111111111111111111111111100000000000000000000000000000000"

	custSk := fmt.Sprintf("%v", custState.SkC)
	custPk := fmt.Sprintf("%v", custState.PkC)
	// merchSk := fmt.Sprintf("%v", *merchState.SkM)
	merchPk := fmt.Sprintf("%v", *merchState.PkM)
	fmt.Println("merchPk", merchPk)
	// changeSk := "4157697b6428532758a9d0f9a73ce58befe3fd665797427d1c5bb3d33f6a132e"
	changePk := "037bed6ab680a171ef2ab564af25eff15c0659313df0bbfb96414da7c7d1e65882"

	merchClosePk := fmt.Sprintf("%v", *merchState.PayoutPk)
	toSelfDelay, err := libzkchannels.GetSelfDelayBE(channelState)
	fmt.Println("toSelfDelay", toSelfDelay)
	if err != nil {
		log.Fatalf("%v", err)
	}

	outputSats := custBal + merchBal
	// escrowTxid_BE, escrowTxid_LE, escrowPrevout, err := FormEscrowTx(cust_utxo_txid, 0, custSk, inputSats, outputSats, custPk, merchPk, changePk, false)
	txFee := int64(500)
	signedEscrowTx, escrowTxid_BE, escrowTxid_LE, escrowPrevout, err := libzkchannels.SignEscrowTx(cust_utxo_txid, cust_utxo_index, custInputSk, inputSats, outputSats, custPk, merchPk, changePk, false, txFee)
	if err != nil {
		log.Fatalf("%v", err)
	}
	_ = signedEscrowTx

	merchTxPreimage, err := libzkchannels.FormMerchCloseTx(escrowTxid_LE, custPk, merchPk, merchClosePk, custBal, merchBal, feeMC, txFeeInfo.ValCpFp, toSelfDelay)
	if err != nil {
		log.Fatalf("%v", err)
	}

	custSig, err := libzkchannels.CustomerSignMerchCloseTx(custSk, merchTxPreimage)
	if err != nil {
		log.Fatalf("%v", err)
	}

	isOk, merchTxid_BE, merchTxid_LE, merchPrevout, merchState, err := libzkchannels.MerchantVerifyMerchCloseTx(escrowTxid_LE, custPk, custBal, merchBal, feeMC, txFeeInfo.ValCpFp, toSelfDelay, custSig, merchState)
	if err != nil {
		log.Fatalf("%v", err)
	}
	_ = merchTxid_LE

	txInfo := libzkchannels.FundingTxInfo{
		EscrowTxId:    escrowTxid_BE, // big-endian
		EscrowPrevout: escrowPrevout, // big-endian
		MerchTxId:     merchTxid_BE,  // big-endian
		MerchPrevout:  merchPrevout,  // big-endian
		InitCustBal:   custBal,
		InitMerchBal:  merchBal,
	}

	custClosePk := custState.PayoutPk
	escrowSig, merchSig, err := libzkchannels.MerchantSignInitCustCloseTx(txInfo, custState.RevLock, custState.PkC, custClosePk, toSelfDelay, merchState, feeCC, feeMC, txFeeInfo.ValCpFp)
	if err != nil {
		log.Fatalf("%v", err)
	}

	isOk, channelToken, custState, err = libzkchannels.CustomerVerifyInitCustCloseTx(txInfo, txFeeInfo, channelState, channelToken, escrowSig, merchSig, custState)
	if err != nil {
		log.Fatalf("%v", err)
	}

	initCustState, initHash, err := libzkchannels.CustomerGetInitialState(custState)
	if err != nil {
		log.Fatalf("%v", err)
	}

	isOk, merchState, err = libzkchannels.MerchantValidateInitialState(channelToken, initCustState, initHash, merchState)
	if !isOk {
		fmt.Println("error: ", err)
	}

	CloseEscrowTx, CloseEscrowTxId_LE, custState, err := libzkchannels.ForceCustomerCloseTx(channelState, channelToken, true, custState)
	if err != nil {
		log.Fatalf("%v", err)
	}
	_ = CloseEscrowTx
	_ = CloseEscrowTxId_LE

	CloseMerchTx, CloseMerchTxId_LE, custState, err := libzkchannels.ForceCustomerCloseTx(channelState, channelToken, false, custState)
	if err != nil {
		log.Fatalf("%v", err)
	}
	_ = CloseMerchTx
	_ = CloseMerchTxId_LE
	// End of libzkchannels_test.go

	// Save variables needed to create cust close in zkcust.db
	zkCustDB, err := zkchanneldb.CreateZkChannelBucket(zkChannelName, custDBPath)
	if err != nil {
		log.Fatalf("%v", err)
	}

	err = zkchanneldb.AddCustState(zkCustDB, zkChannelName, custState)
	if err != nil {
		log.Fatalf("%v", err)
	}

	err = zkchanneldb.AddField(zkCustDB, zkChannelName, channelState, channelStateKey)
	if err != nil {
		log.Fatalf("%v", err)
	}

	err = zkchanneldb.AddField(zkCustDB, zkChannelName, channelToken, channelTokenKey)
	if err != nil {
		log.Fatalf("%v", err)
	}

	err = zkchanneldb.AddField(zkCustDB, zkChannelName, escrowTxid, escrowTxidKey)
	if err != nil {
		log.Fatalf("%v", err)
	}

	err = zkCustDB.Close()
	if err != nil {
		log.Fatal(err)
	}

	// Save variables needed to create merch close in zkmerch.db
	zkMerchDB, err = zkchanneldb.OpenMerchBucket(merchDBPath)
	if err != nil {
		log.Fatalf("%v", err)
	}

	err = zkchanneldb.AddMerchState(zkMerchDB, merchState)
	if err != nil {
		log.Fatalf("%v", err)
	}

	// ZKLND-11 Merchant support for multiple channels
	// cannot use this method for storing escrowTxid as it will get
	// overwritten by new channels
	err = zkchanneldb.AddMerchField(zkMerchDB, escrowTxid_LE, escrowTxidKey)
	if err != nil {
		log.Fatalf("%v", err)
	}

	err = zkchanneldb.AddMerchField(zkMerchDB, channelState, channelStateKey)
	if err != nil {
		log.Fatalf("%v", err)
	}

	err = zkchanneldb.AddMerchField(zkMerchDB, channelToken, channelTokenKey)
	if err != nil {
		log.Fatalf("%v", err)
	}

	err = zkMerchDB.Close()
	if err != nil {
		log.Fatal(err)
	}

}
