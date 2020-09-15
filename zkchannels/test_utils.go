package zkchannels

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
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

func HardcodedTxs(txType string) (tx, txid string, err error) {
	m := make(map[string]string)
	m["escrowTx"] = "020000000001018d2744b606be69fd45d3661e1a61b81ec66d1417941ae33c4ac7079f2bd4aee80000000017160014d83e1345c76dc160630937746d2d2693562e9c58ffffffff02c8550f0000000000220020b718e637a405ba914aede6bcb3d14dec83deca9b0449a20c7163b94456eb01c34489e6050000000016001461492b43be394b9e6eeb077f17e73665bbfd455b024730440220425067bdd154997f29b2862f03177b2909c13e94a339a4da6a2dfb9b3b7a09d602201ed18258aa3e9edac30be79e78b4aeb707fc49710adbd0e6822ef9e4b5953ccd0121032581c94e62b16c1f5fc36c5ff6ddc5c3e7cc4e7e70e2ec3ab3f663cff9d9b7d000000000"
	m["escrowTxid"] = "19a3f487b02b90cb90fe148475e1cdb10946fe15738e351ba7fe311f8dc3d815"

	m["merchCloseTx"] = "0200000000010180ac1ade33d7bf47f27046b4e79ffe737f42cf79cce116a96f5b2612d3ed81740000000000ffffffff02b07c1e00000000002200204bbedbfa9d195e8fc160e6a237d0d702fdae3b5a2d9494beb048452c4647095ae80300000000000016001430d0e52d62063f511cf71bdd8ae633bd514503af04004830450221008bd7b4c25dcbaeb624776adc3e9851b5b5c5ee54d845ed6749c2e12d917e0a550220316529e58b8ac6a0eaf0fee04d3c4bd2d67316951cb8d29c4466c17ac3e4a94f01483045022100be920e94fd52426ff8e09e46132211ace0ad3115e97ff3f0f9209b225124c6db02206bc923bfa19bd9db9cc20269881969586f383b8689bb20728790b853109d111301475221038c2add1dc8cf2c57bac6e19d1f963e0c42103554e8b35e425bc2a78f4c22b273210217d55a1e3ecdd220fde4bddbbfd485a1596c0c5cb7ef11dbfcdb2dd9cf4b85af52ae00000000"
	m["merchCloseTxid"] = "7e4e51a76aaa9f23d6ada1522bfdb4ccefb5cd8875a4982bbb1bf02b6a99e203"

	m["revokedCloseEscrowTx"] = "0200000000010180ac1ade33d7bf47f27046b4e79ffe737f42cf79cce116a96f5b2612d3ed81740000000000ffffffff04703a0f0000000000220020b69ac2024f8d340320003dbe14fb02cf604680839eb0d6cac70dfce31aefa02f40420f000000000016001403d761347fe4398f4ab398266a830ead7c9c2f300000000000000000436a41e6b488e5fcf772078a79ecfc6a787a4aa13cab1782d317d09c84b013f5be48ff027160fb5e48252f02a00066dfa823d15844ad93e04f9c9b746e1f28ed4a1eaddbe803000000000000160014a496306b960746361e3528534d04b1ac4726655a04004730440220295696291e81970a3db52d124add3c45981fb47970e29e9bc3e2b8d8637af7c80220774a1c04a34b3a35cf05d86f1f0138aa2feda3882ac3ab23158310a3c9c7b4530147304402203f323da32dfd40e7b248236e2f86f264fc6ad3d298867049461066a48d6b5cf302206efae4371cfd6fa20e6ede9a8414d6184b129bf4d972520639c5b9863c4a53a901475221038c2add1dc8cf2c57bac6e19d1f963e0c42103554e8b35e425bc2a78f4c22b273210217d55a1e3ecdd220fde4bddbbfd485a1596c0c5cb7ef11dbfcdb2dd9cf4b85af52ae00000000"
	m["revokedCloseEscrowTxid"] = "13b04a68e5bc678d71f7533392d83ec7e6f1dd247e9ed68e4bfd0a28436250f3"

	m["latestCloseEscrowTx"] = "0200000000010115d8c38d1f31fea71b358e7315fe4609b1cde1758414fe90cb902bb087f4a3190000000000ffffffff040e3d0f0000000000220020a97c0ba2d9d85b434e59b67112f30b4f697b16ef9293359e93a1995d4c7f9ccb881300000000000016001403d761347fe4398f4ab398266a830ead7c9c2f300000000000000000436a41a79d68960d7fae77fd8dfee3a03dc7324b09229cd1074b9c50146fc5c87796b1027160fb5e48252f02a00066dfa823d15844ad93e04f9c9b746e1f28ed4a1eaddb4a01000000000000160014a496306b960746361e3528534d04b1ac4726655a0400483045022100ebcb62389d231c3483b9e4fd2ccf62865ff7533386cbfe2cc1eafe89441c850002201dc10e474c293d07b2bdd74825cdd30371dcc984b8f7a75d434a6d2115626361014730440220502e884192441357d4bcfceca004f9a4bd37d12324c27b9a9aea6b05b1ae044602204698b55f1901f6f413f3904958e8c51a3bf312aab1d216d53dc2ccfa4506c90501475221038c2add1dc8cf2c57bac6e19d1f963e0c42103554e8b35e425bc2a78f4c22b273210217d55a1e3ecdd220fde4bddbbfd485a1596c0c5cb7ef11dbfcdb2dd9cf4b85af52ae00000000"
	m["latestCloseEscrowTxid"] = "c3fc727faa847adf55a08ce1844dc801bca4442016b060781f2adbfa5cb8db6c"

	m["custCloseMerchTx"] = "0200000000010103e2996a2bf01bbb2b98a47588cdb5efccb4fd2b52a1add6239faa6aa7514e7e0000000000ffffffff04703a0f0000000000220020b69ac2024f8d340320003dbe14fb02cf604680839eb0d6cac70dfce31aefa02f703a0f000000000016001403d761347fe4398f4ab398266a830ead7c9c2f300000000000000000436a41e6b488e5fcf772078a79ecfc6a787a4aa13cab1782d317d09c84b013f5be48ff027160fb5e48252f02a00066dfa823d15844ad93e04f9c9b746e1f28ed4a1eaddbe803000000000000160014a496306b960746361e3528534d04b1ac4726655a05004830450221008fa4d30c116228934de9aa9636b0bf7331ecee4a785ae91417acba8df2400b5e02206e9fa60df1f4aa55a86970ad4cdc5b15b70fd98d4694b51af025b738465fed7e01483045022100d537d055947db9f5fc4971c17f65fca754c6caff89bb455c746f803adcb478ab0220451f6e92cea5ab72c672b79964000914aefd8c0c7ae6fd7a3d6e5cd08a8a02e601010172635221038c2add1dc8cf2c57bac6e19d1f963e0c42103554e8b35e425bc2a78f4c22b273210217d55a1e3ecdd220fde4bddbbfd485a1596c0c5cb7ef11dbfcdb2dd9cf4b85af52ae6702cf05b2752103780cd60a7ffeb777ec337e2c177e783625c4de907a4aee0f41269cc612fba457ac6800000000"
	m["custCloseMerchTxid"] = "2259fb14bf4a4a30ca0bbb98e9c1bfc9aac8d887c024f395abae9071cc77f141"

	m["merchClaimTx"] = "020000000001011168a2f0333adc67886e7dc9d83f1617346838988313f63749f7e76ac20c92600000000000cf050000011c841e000000000016001461492b43be394b9e6eeb077f17e73665bbfd455b0347304402204cd495f68921c1f171f07e735d693dcf5e390561561c02f5e1d0568fc457056e02202481cce621eda5737aeefb86b1a591578b7115f6673dff716df86c36787f38fc010072635221038c2add1dc8cf2c57bac6e19d1f963e0c42103554e8b35e425bc2a78f4c22b273210217d55a1e3ecdd220fde4bddbbfd485a1596c0c5cb7ef11dbfcdb2dd9cf4b85af52ae6702cf05b2752103780cd60a7ffeb777ec337e2c177e783625c4de907a4aee0f41269cc612fba457ac6800000000"
	m["merchClaimTxid"] = "dd1acbde4ab98446d08fda6aeabd9bd118710e11f3e128661cf26d2c2f7835d5"

	tx, ok := m[txType+"Tx"]
	if ok != true {
		err = fmt.Errorf("%v is not a transaction in HardcodedTxs", txType)
		return "", "", err
	}
	txid, ok = m[txType+"Txid"]
	if ok != true {
		err = fmt.Errorf("%v is not a transaction in HardcodedTxs", txType)
		return "", "", err
	}

	return tx, txid, err
}

func InitMerchConstants() (txFeeInfo libzkchannels.TransactionFeeInfo, skM, payoutSkM, childSkM, disputeSkM string) {
	txFeeInfo = libzkchannels.TransactionFeeInfo{
		BalMinCust:  lncfg.BalMinCust,
		BalMinMerch: lncfg.BalMinMerch,
		ValCpFp:     lncfg.ValCpfp,
		FeeCC:       1000,
		FeeMC:       1000,
		MinFee:      lncfg.MinFee,
		MaxFee:      lncfg.MaxFee,
	}
	skM = "e6e0c5310bb03809e1b2a1595a349f002125fa557d481e51f401ddaf3287e6ae"
	payoutSkM = "5611111111111111111111111111111100000000000000000000000000000000"
	childSkM = "5811111111111111111111111111111100000000000000000000000000000000"
	disputeSkM = "5711111111111111111111111111111100000000000000000000000000000000"
	return txFeeInfo, skM, payoutSkM, childSkM, disputeSkM
}

func InitCustConstants() (txFeeInfo libzkchannels.TransactionFeeInfo, custBal int64, merchBal int64, skC string, payoutSk string) {
	txFeeInfo = libzkchannels.TransactionFeeInfo{
		BalMinCust:  lncfg.BalMinCust,
		BalMinMerch: lncfg.BalMinMerch,
		ValCpFp:     lncfg.ValCpfp,
		FeeCC:       1000,
		FeeMC:       1000,
		MinFee:      lncfg.MinFee,
		MaxFee:      lncfg.MaxFee,
	}
	custBal = int64(1000000)
	merchBal = int64(5000)
	skC = "1a1971e1379beec67178509e25b6772c66cb67bb04d70df2b4bcdb8c08a01827"
	payoutSk = "4157697b6428532758a9d0f9a73ce58befe3fd665797427d1c5bb3d33f6a132e"
	return txFeeInfo, custBal, merchBal, skC, payoutSk
}

func InitFundingUTXO() (inputSats int64, cust_utxo_txid string, cust_utxo_index uint32, custInputSk string, changeSk string, changePk string, escrowFee int64) {
	inputSats = int64(100000000)
	cust_utxo_txid = "e8aed42b9f07c74a3ce31a9417146dc61eb8611a1e66d345fd69be06b644278d"
	cust_utxo_index = uint32(0)
	custInputSk = "5511111111111111111111111111111100000000000000000000000000000000"
	changeSk = "4157697b6428532758a9d0f9a73ce58befe3fd665797427d1c5bb3d33f6a132e"
	changePk = "037bed6ab680a171ef2ab564af25eff15c0659313df0bbfb96414da7c7d1e65882"
	escrowFee = int64(500)
	return inputSats, cust_utxo_txid, cust_utxo_index, custInputSk, changeSk, changePk, escrowFee
}

func InitBreachConstants() (closePkScript, revLock, revSecret, custClosePk string, amount int64) {
	closePkScript = "002072e2c63c7d43aa00de70c445e915b4d9157e270129a4852e0c89c44f644a9757"
	revLock = "802143564b3fc76045db83e6919a68675dc02203ea79721507fb0998d2a259fd"
	revSecret = "92bbe0429d2b7acef6ef5ea1e6afe8b3188d4f31c4b42f635a404365f81726f2"
	custClosePk = "027160fb5e48252f02a00066dfa823d15844ad93e04f9c9b746e1f28ed4a1eaddb"
	amount = 997990
	return closePkScript, revLock, revSecret, custClosePk, amount
}

func SetupTempDBPaths() (string, string, error) {
	testDir, err := ioutil.TempDir("", "zkchanneldb")
	if err != nil {
		return "", "", err
	}
	custDBPath := path.Join(testDir, "zkcust.db")
	merchDBPath := path.Join(testDir, "zkmerch.db")

	return custDBPath, merchDBPath, nil
}

func SetupTestZkDBs() (custDBpath string, merchDBpath string, err error) {

	custDBPath, merchDBPath, err := SetupTempDBPaths()
	if err != nil {
		return "", "", err
	}

	dbUrl := "redis://127.0.0.1/"
	selfDelay := int16(1487) // used to be 1487
	txFeeInfo, skM, payoutSkM, childSkM, disputeSkM := InitMerchConstants()

	channelState, err := libzkchannels.ChannelSetup("channel", selfDelay, txFeeInfo.BalMinCust, txFeeInfo.BalMinMerch, txFeeInfo.ValCpFp, false)
	if err != nil {
		return "", "", err
	}

	// Init Merchant DB
	channelState, merchState, err := libzkchannels.InitMerchant(dbUrl, channelState, "merch")
	if err != nil {
		return "", "", err
	}

	channelState, merchState, err = libzkchannels.LoadMerchantWallet(merchState, channelState, skM, payoutSkM, childSkM, disputeSkM)

	// zkDB add merchState & channelState
	zkMerchDB, err := zkchanneldb.SetupMerchDB(merchDBPath)
	if err != nil {
		return "", "", err
	}

	// save merchStateBytes in zkMerchDB
	err = zkchanneldb.AddMerchState(zkMerchDB, merchState)
	if err != nil {
		return "", "", err
	}

	// save channelStateBytes in zkMerchDB
	err = zkchanneldb.AddMerchField(zkMerchDB, channelState, channelStateKey)
	if err != nil {
		return "", "", err
	}

	zkchannels := make(map[string]libzkchannels.ChannelToken)

	err = zkchanneldb.AddMerchField(zkMerchDB, zkchannels, zkChannelsKey)
	if err != nil {
		return "", "", err
	}

	err = zkMerchDB.Close()
	if err != nil {
		return "", "", err
	}

	// Init Cust DB
	err = zkchanneldb.InitDB(custDBPath)
	if err != nil {
		return "", "", err
	}

	return custDBPath, merchDBPath, nil
}

func TearDownZkDBs(custDBPath, merchDBPath string) error {
	if custDBPath != "" {
		err := os.RemoveAll(custDBPath)
		if err != nil {
			return err
		}
	}
	if merchDBPath != "" {
		err := os.RemoveAll(merchDBPath)
		if err != nil {
			return err
		}
	}
	return nil
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

	txFeeInfo, custBal, merchBal, skC, payoutSk := InitCustConstants()
	feeCC := txFeeInfo.FeeCC
	feeMC := txFeeInfo.FeeMC

	merchPKM := fmt.Sprintf("%v", *merchState.PkM)

	channelToken, custState, err := libzkchannels.InitCustomer(merchPKM, custBal, merchBal, txFeeInfo, "cust")
	if err != nil {
		log.Fatalf("%v", err)
	}

	channelToken, custState, err = libzkchannels.LoadCustomerWallet(custState, channelToken, skC, payoutSk)
	if err != nil {
		log.Fatalf("%v", err)
	}

	custSk := fmt.Sprintf("%v", custState.SkC)
	custPk := fmt.Sprintf("%v", custState.PkC)
	// merchSk := fmt.Sprintf("%v", *merchState.SkM)
	merchPk := fmt.Sprintf("%v", *merchState.PkM)

	inputSats, cust_utxo_txid, cust_utxo_index, custInputSk, _, changePk, txFee := InitFundingUTXO()

	merchClosePk := fmt.Sprintf("%v", *merchState.PayoutPk)
	merchChildPk := fmt.Sprintf("%v", *merchState.ChildPk)
	toSelfDelay, err := libzkchannels.GetSelfDelayBE(channelState)
	if err != nil {
		log.Fatalf("%v", err)
	}

	outputSats := custBal + merchBal
	signedEscrowTx, escrowTxid_BE, escrowTxid_LE, escrowPrevout, err := libzkchannels.SignEscrowTx(cust_utxo_txid, cust_utxo_index, custInputSk, inputSats, outputSats, custPk, merchPk, changePk, false, txFee)
	if err != nil {
		log.Fatalf("%v", err)
	}
	_ = signedEscrowTx

	merchTxPreimage, err := libzkchannels.FormMerchCloseTx(escrowTxid_LE, custPk, merchPk, merchClosePk, merchChildPk, custBal, merchBal, feeMC, txFeeInfo.ValCpFp, toSelfDelay)
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
		log.Fatalf("%v", err)
	}

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

	err = zkchanneldb.AddField(zkCustDB, zkChannelName, escrowTxid_LE, escrowTxidKey)
	if err != nil {
		log.Fatalf("%v", err)
	}

	err = zkCustDB.Close()
	if err != nil {
		log.Fatal(err)
	}

	// merchant's channelState must be in "Open" to run merchClose
	err = UpdateCustChannelState(custDBPath, zkChannelName, "Open")
	if err != nil {
		log.Fatalf("%v", err)
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

	// merchant's channelState must be in "Open" to run merchClose
	err = UpdateMerchChannelState(merchDBPath, escrowTxid_LE, "Open")
	if err != nil {
		log.Fatalf("%v", err)
	}

	// ////////// To print out example txs and txids //////////

	// fmt.Println("escrowTx", signedEscrowTx)
	// fmt.Println("escrowTxid", escrowTxid_LE)
	// {
	// 	signedMerchCloseTx, _, merchTxid2_LE, _, _ := libzkchannels.ForceMerchantCloseTx(escrowTxid_LE, merchState, txFeeInfo.ValCpFp)
	// 	// log.Fatal(err)
	// 	fmt.Println("merchCloseTx", signedMerchCloseTx)
	// 	fmt.Println("merchCloseTxid", merchTxid2_LE)
	// }

	// CloseEscrowTx, CloseEscrowTxId_LE, custState, err := libzkchannels.ForceCustomerCloseTx(channelState, channelToken, true, custState)
	// if err != nil {
	// 	log.Fatalf("%v", err)
	// }
	// fmt.Println("oldCloseEscrowTx", CloseEscrowTx)
	// fmt.Println("oldCloseEscrowTxId", CloseEscrowTxId_LE)

	// CloseMerchTx, CloseMerchTxId_LE, custState, err := libzkchannels.ForceCustomerCloseTx(channelState, channelToken, false, custState)
	// if err != nil {
	// 	log.Fatalf("%v", err)
	// }
	// fmt.Println("oldCloseMerchTx", CloseMerchTx)
	// fmt.Println("oldCloseMerchTxid", CloseMerchTxId_LE)

	// {
	// 	outputPk := "0376dbe15da5257bfc94c37a8af793e022f01a6d981263a73defe292a564c691d2"
	// 	claimAmount := custBal + merchBal - feeCC - feeMC
	// 	claimOutAmount := claimAmount - txFee
	// 	SignedMerchClaimTx, err := libzkchannels.MerchantSignMerchClaimTx(merchTxid_LE, uint32(0), claimAmount, claimOutAmount, toSelfDelay, custPk, outputPk, uint32(0), int64(0), merchState)
	// 	fmt.Println("merchClaimTx", SignedMerchClaimTx)

	// 	// calculate txid for merchClaimTx
	// 	serializedTx, err := hex.DecodeString(SignedMerchClaimTx)
	// 	if err != nil {
	// 		log.Fatal(err)
	// 	}
	// 	var msgTx wire.MsgTx
	// 	err = msgTx.Deserialize(bytes.NewReader(serializedTx))
	// 	if err != nil {
	// 		log.Fatal(err)
	// 	}
	// 	txid := msgTx.TxHash().String()
	// 	fmt.Println("merchClaimTxid", txid)
	// }
}
