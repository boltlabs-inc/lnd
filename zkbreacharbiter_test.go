// +build !rpctest

package lnd

import (
	"encoding/hex"
	"io/ioutil"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/lightningnetwork/lnd/chainntnfs"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/contractcourt"
	"github.com/lightningnetwork/lnd/htlcswitch"
	"github.com/lightningnetwork/lnd/libzkchannels"
	"github.com/lightningnetwork/lnd/lnwallet"
	"github.com/lightningnetwork/lnd/lnwallet/chainfee"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/zkchanneldb"
)

// var (
// 	breachOutPoints = []wire.OutPoint{
// 		{
// 			Hash: [chainhash.HashSize]byte{
// 				0x51, 0xb6, 0x37, 0xd8, 0xfc, 0xd2, 0xc6, 0xda,
// 				0x48, 0x59, 0xe6, 0x96, 0x31, 0x13, 0xa1, 0x17,
// 				0x2d, 0xe7, 0x93, 0xe4, 0xb7, 0x25, 0xb8, 0x4d,
// 				0x1f, 0xb, 0x4c, 0xf9, 0x9e, 0xc5, 0x8c, 0xe9,
// 			},
// 			Index: 9,
// 		},
// 		{
// 			Hash: [chainhash.HashSize]byte{
// 				0xb7, 0x94, 0x38, 0x5f, 0x2d, 0x1e, 0xf7, 0xab,
// 				0x4d, 0x92, 0x73, 0xd1, 0x90, 0x63, 0x81, 0xb4,
// 				0x4f, 0x2f, 0x6f, 0x25, 0x88, 0xa3, 0xef, 0xb9,
// 				0x6a, 0x49, 0x18, 0x83, 0x31, 0x98, 0x47, 0x53,
// 			},
// 			Index: 49,
// 		},
// 		{
// 			Hash: [chainhash.HashSize]byte{
// 				0x81, 0xb6, 0x37, 0xd8, 0xfc, 0xd2, 0xc6, 0xda,
// 				0x63, 0x59, 0xe6, 0x96, 0x31, 0x13, 0xa1, 0x17,
// 				0xd, 0xe7, 0x95, 0xe4, 0xb7, 0x25, 0xb8, 0x4d,
// 				0x1e, 0xb, 0x4c, 0xfd, 0x9e, 0xc5, 0x8c, 0xe9,
// 			},
// 			Index: 23,
// 		},
// 	}

// 	breachKeys = [][]byte{
// 		{0x04, 0x11, 0xdb, 0x93, 0xe1, 0xdc, 0xdb, 0x8a,
// 			0x01, 0x6b, 0x49, 0x84, 0x0f, 0x8c, 0x53, 0xbc, 0x1e,
// 			0xb6, 0x8a, 0x38, 0x2e, 0x97, 0xb1, 0x48, 0x2e, 0xca,
// 			0xd7, 0xb1, 0x48, 0xa6, 0x90, 0x9a, 0x5c, 0xb2, 0xe0,
// 			0xea, 0xdd, 0xfb, 0x84, 0xcc, 0xf9, 0x74, 0x44, 0x64,
// 			0xf8, 0x2e, 0x16, 0x0b, 0xfa, 0x9b, 0x8b, 0x64, 0xf9,
// 			0xd4, 0xc0, 0x3f, 0x99, 0x9b, 0x86, 0x43, 0xf6, 0x56,
// 			0xb4, 0x12, 0xa3,
// 		},
// 		{0x07, 0x11, 0xdb, 0x93, 0xe1, 0xdc, 0xdb, 0x8a,
// 			0x01, 0x6b, 0x49, 0x84, 0x0f, 0x8c, 0x53, 0xbc, 0x1e,
// 			0xb6, 0x8a, 0x38, 0x2e, 0x97, 0xb1, 0x48, 0x2e, 0xca,
// 			0xd7, 0xb1, 0x48, 0xa6, 0x90, 0x9a, 0x5c, 0xb2, 0xe0,
// 			0xea, 0xdd, 0xfb, 0x84, 0xcc, 0xf9, 0x74, 0x44, 0x64,
// 			0xf8, 0x2e, 0x16, 0x0b, 0xfa, 0x9b, 0x8b, 0x64, 0xf9,
// 			0xd4, 0xc0, 0x3f, 0x99, 0x9b, 0x86, 0x43, 0xf6, 0x56,
// 			0xb4, 0x12, 0xa3,
// 		},
// 		{0x02, 0xce, 0x0b, 0x14, 0xfb, 0x84, 0x2b, 0x1b,
// 			0xa5, 0x49, 0xfd, 0xd6, 0x75, 0xc9, 0x80, 0x75, 0xf1,
// 			0x2e, 0x9c, 0x51, 0x0f, 0x8e, 0xf5, 0x2b, 0xd0, 0x21,
// 			0xa9, 0xa1, 0xf4, 0x80, 0x9d, 0x3b, 0x4d,
// 		},
// 		{0x02, 0xce, 0x0b, 0x14, 0xfb, 0x84, 0x2b, 0x1b,
// 			0x2e, 0x9c, 0x51, 0x0f, 0x8e, 0xf5, 0x2b, 0xd0, 0x21,
// 			0xa5, 0x49, 0xfd, 0xd6, 0x75, 0xc9, 0x80, 0x75, 0xf1,
// 			0xa3, 0xa1, 0xf4, 0x80, 0x9d, 0x3b, 0x4d,
// 		},
// 	}

// 	breachedOutputs = []breachedOutput{
// 		{
// 			amt:         btcutil.Amount(1e7),
// 			outpoint:    breachOutPoints[0],
// 			witnessType: input.CommitmentNoDelay,
// 			signDesc: input.SignDescriptor{
// 				SingleTweak: []byte{
// 					0x02, 0x02, 0x02, 0x02, 0x02, 0x02,
// 					0x02, 0x02, 0x02, 0x02, 0x02, 0x02,
// 					0x02, 0x02, 0x02, 0x02, 0x02, 0x02,
// 					0x02, 0x02, 0x02, 0x02, 0x02, 0x02,
// 					0x02, 0x02, 0x02, 0x02, 0x02, 0x02,
// 					0x02, 0x02,
// 				},
// 				WitnessScript: []byte{
// 					0x00, 0x14, 0xee, 0x91, 0x41, 0x7e,
// 					0x85, 0x6c, 0xde, 0x10, 0xa2, 0x91,
// 					0x1e, 0xdc, 0xbd, 0xbd, 0x69, 0xe2,
// 					0xef, 0xb5, 0x71, 0x48,
// 				},
// 				Output: &wire.TxOut{
// 					Value: 5000000000,
// 					PkScript: []byte{
// 						0x41, // OP_DATA_65
// 						0x04, 0xd6, 0x4b, 0xdf, 0xd0,
// 						0x9e, 0xb1, 0xc5, 0xfe, 0x29,
// 						0x5a, 0xbd, 0xeb, 0x1d, 0xca,
// 						0x42, 0x81, 0xbe, 0x98, 0x8e,
// 						0x2d, 0xa0, 0xb6, 0xc1, 0xc6,
// 						0xa5, 0x9d, 0xc2, 0x26, 0xc2,
// 						0x86, 0x24, 0xe1, 0x81, 0x75,
// 						0xe8, 0x51, 0xc9, 0x6b, 0x97,
// 						0x3d, 0x81, 0xb0, 0x1c, 0xc3,
// 						0x1f, 0x04, 0x78, 0x34, 0xbc,
// 						0x06, 0xd6, 0xd6, 0xed, 0xf6,
// 						0x20, 0xd1, 0x84, 0x24, 0x1a,
// 						0x6a, 0xed, 0x8b, 0x63,
// 						0xa6, // 65-byte signature
// 						0xac, // OP_CHECKSIG
// 					},
// 				},
// 				HashType: txscript.SigHashAll,
// 			},
// 			secondLevelWitnessScript: breachKeys[0],
// 		},
// 		{
// 			amt:         btcutil.Amount(1e7),
// 			outpoint:    breachOutPoints[0],
// 			witnessType: input.CommitSpendNoDelayTweakless,
// 			signDesc: input.SignDescriptor{
// 				WitnessScript: []byte{
// 					0x00, 0x14, 0xee, 0x91, 0x41, 0x7e,
// 					0x85, 0x6c, 0xde, 0x10, 0xa2, 0x91,
// 					0x1e, 0xdc, 0xbd, 0xbd, 0x69, 0xe2,
// 					0xef, 0xb5, 0x71, 0x48,
// 				},
// 				Output: &wire.TxOut{
// 					Value: 5000000000,
// 					PkScript: []byte{
// 						0x41, // OP_DATA_65
// 						0x04, 0xd6, 0x4b, 0xdf, 0xd0,
// 						0x9e, 0xb1, 0xc5, 0xfe, 0x29,
// 						0x5a, 0xbd, 0xeb, 0x1d, 0xca,
// 						0x42, 0x81, 0xbe, 0x98, 0x8e,
// 						0x2d, 0xa0, 0xb6, 0xc1, 0xc6,
// 						0xa5, 0x9d, 0xc2, 0x26, 0xc2,
// 						0x86, 0x24, 0xe1, 0x81, 0x75,
// 						0xe8, 0x51, 0xc9, 0x6b, 0x97,
// 						0x3d, 0x81, 0xb0, 0x1c, 0xc3,
// 						0x1f, 0x04, 0x78, 0x34, 0xbc,
// 						0x06, 0xd6, 0xd6, 0xed, 0xf6,
// 						0x20, 0xd1, 0x84, 0x24, 0x1a,
// 						0x6a, 0xed, 0x8b, 0x63,
// 						0xa6, // 65-byte signature
// 						0xac, // OP_CHECKSIG
// 					},
// 				},
// 				HashType: txscript.SigHashAll,
// 			},
// 			secondLevelWitnessScript: breachKeys[0],
// 		},
// 		{
// 			amt:         btcutil.Amount(2e9),
// 			outpoint:    breachOutPoints[1],
// 			witnessType: input.CommitmentRevoke,
// 			signDesc: input.SignDescriptor{
// 				SingleTweak: []byte{
// 					0x02, 0x02, 0x02, 0x02, 0x02, 0x02,
// 					0x02, 0x02, 0x02, 0x02, 0x02, 0x02,
// 					0x02, 0x02, 0x02, 0x02, 0x02, 0x02,
// 					0x02, 0x02, 0x02, 0x02, 0x02, 0x02,
// 					0x02, 0x02, 0x02, 0x02, 0x02, 0x02,
// 					0x02, 0x02,
// 				},
// 				WitnessScript: []byte{
// 					0x00, 0x14, 0xee, 0x91, 0x41, 0x7e,
// 					0x85, 0x6c, 0xde, 0x10, 0xa2, 0x91,
// 					0x1e, 0xdc, 0xbd, 0xbd, 0x69, 0xe2,
// 					0xef, 0xb5, 0x71, 0x48,
// 				},
// 				Output: &wire.TxOut{
// 					Value: 5000000000,
// 					PkScript: []byte{
// 						0x41, // OP_DATA_65
// 						0x04, 0xd6, 0x4b, 0xdf, 0xd0,
// 						0x9e, 0xb1, 0xc5, 0xfe, 0x29,
// 						0x5a, 0xbd, 0xeb, 0x1d, 0xca,
// 						0x42, 0x81, 0xbe, 0x98, 0x8e,
// 						0x2d, 0xa0, 0xb6, 0xc1, 0xc6,
// 						0xa5, 0x9d, 0xc2, 0x26, 0xc2,
// 						0x86, 0x24, 0xe1, 0x81, 0x75,
// 						0xe8, 0x51, 0xc9, 0x6b, 0x97,
// 						0x3d, 0x81, 0xb0, 0x1c, 0xc3,
// 						0x1f, 0x04, 0x78, 0x34, 0xbc,
// 						0x06, 0xd6, 0xd6, 0xed, 0xf6,
// 						0x20, 0xd1, 0x84, 0x24, 0x1a,
// 						0x6a, 0xed, 0x8b, 0x63,
// 						0xa6, // 65-byte signature
// 						0xac, // OP_CHECKSIG
// 					},
// 				},
// 				HashType: txscript.SigHashAll,
// 			},
// 			secondLevelWitnessScript: breachKeys[0],
// 		},
// 		{
// 			amt:         btcutil.Amount(3e4),
// 			outpoint:    breachOutPoints[2],
// 			witnessType: input.CommitmentDelayOutput,
// 			signDesc: input.SignDescriptor{
// 				SingleTweak: []byte{
// 					0x02, 0x02, 0x02, 0x02, 0x02, 0x02,
// 					0x02, 0x02, 0x02, 0x02, 0x02, 0x02,
// 					0x02, 0x02, 0x02, 0x02, 0x02, 0x02,
// 					0x02, 0x02, 0x02, 0x02, 0x02, 0x02,
// 					0x02, 0x02, 0x02, 0x02, 0x02, 0x02,
// 					0x02, 0x02,
// 				},
// 				WitnessScript: []byte{
// 					0x00, 0x14, 0xee, 0x91, 0x41, 0x7e,
// 					0x85, 0x6c, 0xde, 0x10, 0xa2, 0x91,
// 					0x1e, 0xdc, 0xbd, 0xbd, 0x69, 0xe2,
// 					0xef, 0xb5, 0x71, 0x48,
// 				},
// 				Output: &wire.TxOut{
// 					Value: 5000000000,
// 					PkScript: []byte{
// 						0x41, // OP_DATA_65
// 						0x04, 0xd6, 0x4b, 0xdf, 0xd0,
// 						0x9e, 0xb1, 0xc5, 0xfe, 0x29,
// 						0x5a, 0xbd, 0xeb, 0x1d, 0xca,
// 						0x42, 0x81, 0xbe, 0x98, 0x8e,
// 						0x2d, 0xa0, 0xb6, 0xc1, 0xc6,
// 						0xa5, 0x9d, 0xc2, 0x26, 0xc2,
// 						0x86, 0x24, 0xe1, 0x81, 0x75,
// 						0xe8, 0x51, 0xc9, 0x6b, 0x97,
// 						0x3d, 0x81, 0xb0, 0x1c, 0xc3,
// 						0x1f, 0x04, 0x78, 0x34, 0xbc,
// 						0x06, 0xd6, 0xd6, 0xed, 0xf6,
// 						0x20, 0xd1, 0x84, 0x24, 0x1a,
// 						0x6a, 0xed, 0x8b, 0x63,
// 						0xa6, // 65-byte signature
// 						0xac, // OP_CHECKSIG
// 					},
// 				},
// 				HashType: txscript.SigHashAll,
// 			},
// 			secondLevelWitnessScript: breachKeys[0],
// 		},
// 	}

// 	retributionMap = make(map[wire.OutPoint]retributionInfo)
// 	retributions   = []retributionInfo{
// 		{
// 			commitHash: [chainhash.HashSize]byte{
// 				0xb7, 0x94, 0x38, 0x5f, 0x2d, 0x1e, 0xf7, 0xab,
// 				0x4d, 0x92, 0x73, 0xd1, 0x90, 0x63, 0x81, 0xb4,
// 				0x4f, 0x2f, 0x6f, 0x25, 0x88, 0xa3, 0xef, 0xb9,
// 				0x6a, 0x49, 0x18, 0x83, 0x31, 0x98, 0x47, 0x53,
// 			},
// 			chainHash: [chainhash.HashSize]byte{
// 				0x4d, 0x92, 0x73, 0xd1, 0x90, 0x63, 0x81, 0xb4,
// 				0x4f, 0x2f, 0x6f, 0x25, 0x88, 0xa3, 0xef, 0xb9,
// 				0xb7, 0x94, 0x38, 0x5f, 0x2d, 0x1e, 0xf7, 0xab,
// 				0x6b, 0x49, 0x18, 0x83, 0x31, 0x98, 0x47, 0x53,
// 			},
// 			chanPoint:    breachOutPoints[0],
// 			breachHeight: 337,
// 			// Set to breachedOutputs 0 and 1 in init()
// 			breachedOutputs: []breachedOutput{{}, {}},
// 		},
// 		{
// 			commitHash: [chainhash.HashSize]byte{
// 				0x51, 0xb6, 0x37, 0xd8, 0xfc, 0xd2, 0xc6, 0xda,
// 				0x48, 0x59, 0xe6, 0x96, 0x31, 0x13, 0xa1, 0x17,
// 				0x2d, 0xe7, 0x93, 0xe4, 0xb7, 0x25, 0xb8, 0x4d,
// 				0x1f, 0xb, 0x4c, 0xf9, 0x9e, 0xc5, 0x8c, 0xe9,
// 			},
// 			chainHash: [chainhash.HashSize]byte{
// 				0x4f, 0x2f, 0x6f, 0x25, 0x88, 0xa3, 0xef, 0xb9,
// 				0xb7, 0x94, 0x39, 0x5f, 0x2d, 0x1e, 0xf7, 0xab,
// 				0x6b, 0x49, 0x18, 0x83, 0x31, 0x98, 0x47, 0x53,
// 				0x4d, 0x92, 0x73, 0xd1, 0x90, 0x63, 0x81, 0xb4,
// 			},
// 			chanPoint:    breachOutPoints[1],
// 			breachHeight: 420420,
// 			// Set to breachedOutputs 1 and 2 in init()
// 			breachedOutputs: []breachedOutput{{}, {}},
// 		},
// 	}
// )

// zkchannel test txs
const (
	escrowTx   = "0200000000010132eb1608d217f821e3bfa16c4ab70dc3e56f1d65a7d84dd4a2aba9b244e50d520000000000ffffffff030627000000000000220020f9f4144b30c03dc6f56e29553755e6d1554c32bdad05f8f043283226e07200b40a00000000000000160014d05ac4b544f30ed4648d25eff1700f8e5e0a03b80000000000000000436a414d8e9a9f606a8210478c1fe86a79a98e43484c825df7b18dc351ada564bebd25027160fb5e48252f02a00066dfa823d15844ad93e04f9c9b746e1f28ed4a1eaddb0400483045022100e76b48106224b2c896e3095794dded09a38160b46cd86439fe83f399cc2ba9c1022062e4cb1e650c79e4ca78ac347cf08a3ea6377f4d1566169f960ec268a6c0099501473044022005bfbf4706cc3af47366b9a19200adbb6e58c8266a72fd3cd19b4a4e8eb4f9a20220254d2627588a0a66cd69267f01e54d14b941895f649b8d5dbf52273aa686545b0147522103742aaaffc1dd1e8016355012dbcbc7145291412078e209c3e8bf0fe323ccbf29210217d55a1e3ecdd220fde4bddbbfd485a1596c0c5cb7ef11dbfcdb2dd9cf4b85af52ae00000000"
	escrowTxid = "9a481f1729ff18783b18e4489411307ec69e67072deeba6c0cfe91e352738220"

	merchCloseTxid = "f7aaea919dfd714948253c26b5f0ce45bbc16cca6d01c916696ac97566ced04f"

	custCloseTxid = "9a481f1729ff18783b18e4489411307ec69e67072deeba6c0cfe91e352738220"
	// custCloseTx   = "0200000000010132eb1608d217f821e3bfa16c4ab70dc3e56f1d65a7d84dd4a2aba9b244e50d520000000000ffffffff030627000000000000220020f9f4144b30c03dc6f56e29553755e6d1554c32bdad05f8f043283226e07200b40a00000000000000160014d05ac4b544f30ed4648d25eff1700f8e5e0a03b80000000000000000436a414d8e9a9f606a8210478c1fe86a79a98e43484c825df7b18dc351ada564bebd25027160fb5e48252f02a00066dfa823d15844ad93e04f9c9b746e1f28ed4a1eaddb0400483045022100e76b48106224b2c896e3095794dded09a38160b46cd86439fe83f399cc2ba9c1022062e4cb1e650c79e4ca78ac347cf08a3ea6377f4d1566169f960ec268a6c0099501473044022005bfbf4706cc3af47366b9a19200adbb6e58c8266a72fd3cd19b4a4e8eb4f9a20220254d2627588a0a66cd69267f01e54d14b941895f649b8d5dbf52273aa686545b0147522103742aaaffc1dd1e8016355012dbcbc7145291412078e209c3e8bf0fe323ccbf29210217d55a1e3ecdd220fde4bddbbfd485a1596c0c5cb7ef11dbfcdb2dd9cf4b85af52ae00000000"

// custCloseMerchTxid = "cc7e73d1f20d69b9bfa858a79a7cfc03529123a12fba57700fad923d4ebe341f"
// custCloseMerchTx   = "020000000001014360aa5440c33ba6ddf6cc1799a49fc46b99b07130544b3134f8cf8a6169fb6b0000000000ffffffff030627000000000000220020eab1b21b1227f90a54c80bd0ac992ff57daca6b89510c68281be97a995bd2ac70a00000000000000160014234f7938f5c82ddbcd99c810a885eb336ce097670000000000000000436a416dad4b8de4ea7765e89a69def72048c6bd1b9f2fb143aec83d0b924c42adc074027160fb5e48252f02a00066dfa823d15844ad93e04f9c9b746e1f28ed4a1eaddb050047304402203a440bde54817e96a8611fb69571304e1503a31da20da8bee2033ba06bb5f4cf02200a2fdba6d89004cbb78f995d1a84251df90fbaf36f0022d08df9828a3cf46c72014730440220137c9d3f83f7e8aeb4235e098f446be72aebb06c6aaf92248c2f33e82f38a7760220020517fb88acfbb5d97131f653d4abfe284866d4f526573181cbd3315264831901010172635221025f6ed070ebe3a585e70f77058e3c4ba0c373c3bedebda19a077a860955b6026b210217d55a1e3ecdd220fde4bddbbfd485a1596c0c5cb7ef11dbfcdb2dd9cf4b85af52ae6702cf05b2752103efac29be9b04fc77d27298e36e3c0eded7ab1a98cbe1aee629827df37ef9b6cfac6800000000"
)

// func init() {
// 	// Ensure that breached outputs are initialized before starting tests.
// 	if err := initBreachedOutputs(); err != nil {
// 		panic(err)
// 	}

// 	// Populate a retribution map to for convenience, to allow lookups by
// 	// channel point.
// 	for i := range retributions {
// 		retInfo := &retributions[i]
// 		retInfo.breachedOutputs[0] = breachedOutputs[i]
// 		retInfo.breachedOutputs[1] = breachedOutputs[i+1]

// 		retributionMap[retInfo.chanPoint] = *retInfo

// 	}
// }

// // FailingRetributionStore wraps a RetributionStore and supports controlled
// // restarts of the persistent instance. This allows us to test (1) that no
// // modifications to the entries are made between calls or through side effects,
// // and (2) that the database is actually being persisted between actions.
// type FailingRetributionStore interface {
// 	RetributionStore

// 	Restart()
// }

// // failingRetributionStore is a concrete implementation of a
// // FailingRetributionStore. It wraps an underlying RetributionStore and is
// // parameterized entirely by a restart function, which is intended to simulate a
// // full stop/start of the store.
// type failingRetributionStore struct {
// 	mu sync.Mutex

// 	rs RetributionStore

// 	nextAddErr error

// 	restart func() RetributionStore
// }

// // newFailingRetributionStore creates a new failing retribution store. The given
// // restart closure should ensure that it is reloading its contents from the
// // persistent source.
// func newFailingRetributionStore(
// 	restart func() RetributionStore) *failingRetributionStore {

// 	return &failingRetributionStore{
// 		mu:      sync.Mutex{},
// 		rs:      restart(),
// 		restart: restart,
// 	}
// }

// // FailNextAdd instructs the retribution store to return the provided error. If
// // the error is nil, a generic default will be used.
// func (frs *failingRetributionStore) FailNextAdd(err error) {
// 	if err == nil {
// 		err = errors.New("retribution store failed")
// 	}

// 	frs.mu.Lock()
// 	frs.nextAddErr = err
// 	frs.mu.Unlock()
// }

// func (frs *failingRetributionStore) Restart() {
// 	frs.mu.Lock()
// 	frs.rs = frs.restart()
// 	frs.mu.Unlock()
// }

// // Add forwards the call to the underlying retribution store, unless this Add
// // has been previously instructed to fail.
// func (frs *failingRetributionStore) Add(retInfo *retributionInfo) error {
// 	frs.mu.Lock()
// 	defer frs.mu.Unlock()

// 	if frs.nextAddErr != nil {
// 		err := frs.nextAddErr
// 		frs.nextAddErr = nil
// 		return err
// 	}

// 	return frs.rs.Add(retInfo)
// }

// func (frs *failingRetributionStore) IsBreached(chanPoint *wire.OutPoint) (bool, error) {
// 	frs.mu.Lock()
// 	defer frs.mu.Unlock()

// 	return frs.rs.IsBreached(chanPoint)
// }

// func (frs *failingRetributionStore) Finalize(chanPoint *wire.OutPoint,
// 	finalTx *wire.MsgTx) error {

// 	frs.mu.Lock()
// 	defer frs.mu.Unlock()

// 	return frs.rs.Finalize(chanPoint, finalTx)
// }

// func (frs *failingRetributionStore) GetFinalizedTxn(
// 	chanPoint *wire.OutPoint) (*wire.MsgTx, error) {

// 	frs.mu.Lock()
// 	defer frs.mu.Unlock()

// 	return frs.rs.GetFinalizedTxn(chanPoint)
// }

// func (frs *failingRetributionStore) Remove(key *wire.OutPoint) error {
// 	frs.mu.Lock()
// 	defer frs.mu.Unlock()

// 	return frs.rs.Remove(key)
// }

// func (frs *failingRetributionStore) ForAll(cb func(*retributionInfo) error) error {
// 	frs.mu.Lock()
// 	defer frs.mu.Unlock()

// 	return frs.rs.ForAll(cb)
// }

// // Parse the pubkeys in the breached outputs.
// func initBreachedOutputs() error {
// 	for i := range breachedOutputs {
// 		bo := &breachedOutputs[i]

// 		// Parse the sign descriptor's pubkey.
// 		pubkey, err := btcec.ParsePubKey(breachKeys[i], btcec.S256())
// 		if err != nil {
// 			return fmt.Errorf("unable to parse pubkey: %v",
// 				breachKeys[i])
// 		}
// 		bo.signDesc.KeyDesc.PubKey = pubkey
// 	}

// 	return nil
// }

// // Test that breachedOutput Encode/Decode works.
// func TestBreachedOutputSerialization(t *testing.T) {
// 	for i := range breachedOutputs {
// 		bo := &breachedOutputs[i]

// 		var buf bytes.Buffer

// 		if err := bo.Encode(&buf); err != nil {
// 			t.Fatalf("unable to serialize breached output [%v]: %v",
// 				i, err)
// 		}

// 		desBo := &breachedOutput{}
// 		if err := desBo.Decode(&buf); err != nil {
// 			t.Fatalf("unable to deserialize "+
// 				"breached output [%v]: %v", i, err)
// 		}

// 		if !reflect.DeepEqual(bo, desBo) {
// 			t.Fatalf("original and deserialized "+
// 				"breached outputs not equal:\n"+
// 				"original     : %+v\n"+
// 				"deserialized : %+v\n",
// 				bo, desBo)
// 		}
// 	}
// }

// // Test that retribution Encode/Decode works.
// func TestRetributionSerialization(t *testing.T) {
// 	for i := range retributions {
// 		ret := &retributions[i]

// 		var buf bytes.Buffer

// 		if err := ret.Encode(&buf); err != nil {
// 			t.Fatalf("unable to serialize retribution [%v]: %v",
// 				i, err)
// 		}

// 		desRet := &retributionInfo{}
// 		if err := desRet.Decode(&buf); err != nil {
// 			t.Fatalf("unable to deserialize retribution [%v]: %v",
// 				i, err)
// 		}

// 		if !reflect.DeepEqual(ret, desRet) {
// 			t.Fatalf("original and deserialized "+
// 				"retribution infos not equal:\n"+
// 				"original     : %+v\n"+
// 				"deserialized : %+v\n",
// 				ret, desRet)
// 		}
// 	}
// }

// // copyRetInfo creates a complete copy of the given retributionInfo.
// func copyRetInfo(retInfo *retributionInfo) *retributionInfo {
// 	nOutputs := len(retInfo.breachedOutputs)

// 	ret := &retributionInfo{
// 		commitHash:      retInfo.commitHash,
// 		chainHash:       retInfo.chainHash,
// 		chanPoint:       retInfo.chanPoint,
// 		breachHeight:    retInfo.breachHeight,
// 		breachedOutputs: make([]breachedOutput, nOutputs),
// 	}

// 	for i := range retInfo.breachedOutputs {
// 		ret.breachedOutputs[i] = retInfo.breachedOutputs[i]
// 	}

// 	return ret
// }

// // mockRetributionStore implements the RetributionStore interface and is backed
// // by an in-memory map. Access to the internal state is provided by a mutex.
// // TODO(cfromknecht) extend to support and test controlled failures.
// type mockRetributionStore struct {
// 	mu       sync.Mutex
// 	state    map[wire.OutPoint]*retributionInfo
// 	finalTxs map[wire.OutPoint]*wire.MsgTx
// }

// func newMockRetributionStore() *mockRetributionStore {
// 	return &mockRetributionStore{
// 		mu:       sync.Mutex{},
// 		state:    make(map[wire.OutPoint]*retributionInfo),
// 		finalTxs: make(map[wire.OutPoint]*wire.MsgTx),
// 	}
// }

// func (rs *mockRetributionStore) Add(retInfo *retributionInfo) error {
// 	rs.mu.Lock()
// 	rs.state[retInfo.chanPoint] = copyRetInfo(retInfo)
// 	rs.mu.Unlock()

// 	return nil
// }

// func (rs *mockRetributionStore) IsBreached(chanPoint *wire.OutPoint) (bool, error) {
// 	rs.mu.Lock()
// 	_, ok := rs.state[*chanPoint]
// 	rs.mu.Unlock()

// 	return ok, nil
// }

// func (rs *mockRetributionStore) Finalize(chanPoint *wire.OutPoint,
// 	finalTx *wire.MsgTx) error {

// 	rs.mu.Lock()
// 	rs.finalTxs[*chanPoint] = finalTx
// 	rs.mu.Unlock()

// 	return nil
// }

// func (rs *mockRetributionStore) GetFinalizedTxn(
// 	chanPoint *wire.OutPoint) (*wire.MsgTx, error) {

// 	rs.mu.Lock()
// 	finalTx := rs.finalTxs[*chanPoint]
// 	rs.mu.Unlock()

// 	return finalTx, nil
// }

// func (rs *mockRetributionStore) Remove(key *wire.OutPoint) error {
// 	rs.mu.Lock()
// 	delete(rs.state, *key)
// 	delete(rs.finalTxs, *key)
// 	rs.mu.Unlock()

// 	return nil
// }

// func (rs *mockRetributionStore) ForAll(cb func(*retributionInfo) error) error {
// 	rs.mu.Lock()
// 	defer rs.mu.Unlock()

// 	for _, retInfo := range rs.state {
// 		if err := cb(copyRetInfo(retInfo)); err != nil {
// 			return err
// 		}
// 	}

// 	return nil
// }

// var retributionStoreTestSuite = []struct {
// 	name string
// 	test func(FailingRetributionStore, *testing.T)
// }{
// 	{
// 		"Initialization",
// 		testRetributionStoreInit,
// 	},
// 	{
// 		"Add/Remove",
// 		testRetributionStoreAddRemove,
// 	},
// 	{
// 		"Persistence",
// 		testRetributionStorePersistence,
// 	},
// 	{
// 		"Overwrite",
// 		testRetributionStoreOverwrite,
// 	},
// 	{
// 		"RemoveEmpty",
// 		testRetributionStoreRemoveEmpty,
// 	},
// }

// // TestMockRetributionStore instantiates a mockRetributionStore and tests its
// // behavior using the general RetributionStore test suite.
// func TestMockRetributionStore(t *testing.T) {
// 	for _, test := range retributionStoreTestSuite {
// 		t.Run(
// 			"mockRetributionStore."+test.name,
// 			func(tt *testing.T) {
// 				mrs := newMockRetributionStore()
// 				frs := newFailingRetributionStore(
// 					func() RetributionStore { return mrs },
// 				)
// 				test.test(frs, tt)
// 			},
// 		)
// 	}
// }

// func makeTestChannelDB() (*channeldb.DB, func(), error) {
// 	// First, create a temporary directory to be used for the duration of
// 	// this test.
// 	tempDirName, err := ioutil.TempDir("", "channeldb")
// 	if err != nil {
// 		return nil, nil, err
// 	}

// 	cleanUp := func() {
// 		os.RemoveAll(tempDirName)
// 	}

// 	db, err := channeldb.Open(tempDirName)
// 	if err != nil {
// 		cleanUp()
// 		return nil, nil, err
// 	}

// 	return db, cleanUp, nil
// }

// // TestChannelDBRetributionStore instantiates a retributionStore backed by a
// // channeldb.DB, and tests its behavior using the general RetributionStore test
// // suite.
// func TestChannelDBRetributionStore(t *testing.T) {
// 	// Finally, instantiate retribution store and execute RetributionStore
// 	// test suite.
// 	for _, test := range retributionStoreTestSuite {
// 		t.Run(
// 			"channeldbDBRetributionStore."+test.name,
// 			func(tt *testing.T) {
// 				db, cleanUp, err := makeTestChannelDB()
// 				if err != nil {
// 					t.Fatalf("unable to open channeldb: %v", err)
// 				}
// 				defer db.Close()
// 				defer cleanUp()

// 				restartDb := func() RetributionStore {
// 					// Close and reopen channeldb
// 					if err = db.Close(); err != nil {
// 						t.Fatalf("unable to close "+
// 							"channeldb during "+
// 							"restart: %v",
// 							err)
// 					}
// 					db, err = channeldb.Open(db.Path())
// 					if err != nil {
// 						t.Fatalf("unable to open "+
// 							"channeldb: %v", err)
// 					}

// 					return newRetributionStore(db)
// 				}

// 				frs := newFailingRetributionStore(restartDb)
// 				test.test(frs, tt)
// 			},
// 		)
// 	}
// }

// // countRetributions uses a retribution store's ForAll to count the number of
// // elements emitted from the store.
// func countRetributions(t *testing.T, rs RetributionStore) int {
// 	count := 0
// 	err := rs.ForAll(func(_ *retributionInfo) error {
// 		count++
// 		return nil
// 	})
// 	if err != nil {
// 		t.Fatalf("unable to list retributions in db: %v", err)
// 	}
// 	return count
// }

// // testRetributionStoreAddRemove executes a generic test suite for any concrete
// // implementation of the RetributionStore interface. This test adds all
// // retributions to the store, confirms that they are all present, and then
// // removes each one individually.  Between each addition or removal, the number
// // of elements in the store is checked to ensure that it only changes by one.
// func testRetributionStoreAddRemove(frs FailingRetributionStore, t *testing.T) {
// 	// Make sure that a new retribution store is actually empty.
// 	if count := countRetributions(t, frs); count != 0 {
// 		t.Fatalf("expected 0 retributions, found %v", count)
// 	}

// 	// Add all retributions, check that ForAll returns the correct
// 	// information, and then remove all retributions.
// 	testRetributionStoreAdds(frs, t, false)
// 	testRetributionStoreForAll(frs, t, false)
// 	testRetributionStoreRemoves(frs, t, false)
// }

// // testRetributionStorePersistence executes the same general test as
// // testRetributionStoreAddRemove, except that it also restarts the store between
// // each operation to ensure that the results are properly persisted.
// func testRetributionStorePersistence(frs FailingRetributionStore, t *testing.T) {
// 	// Make sure that a new retribution store is still empty after failing
// 	// right off the bat.
// 	frs.Restart()
// 	if count := countRetributions(t, frs); count != 0 {
// 		t.Fatalf("expected 1 retributions, found %v", count)
// 	}

// 	// Insert all retributions into the database, restarting and checking
// 	// between subsequent calls to test that each intermediate additions are
// 	// persisted.
// 	testRetributionStoreAdds(frs, t, true)

// 	// After all retributions have been inserted, verify that the store
// 	// emits a distinct set of retributions that are equivalent to the test
// 	// vector.
// 	testRetributionStoreForAll(frs, t, true)

// 	// Remove all retributions from the database, restarting and checking
// 	// between subsequent calls to test that each intermediate removals are
// 	// persisted.
// 	testRetributionStoreRemoves(frs, t, true)
// }

// // testRetributionStoreInit ensures that a retribution store is always
// // initialized with no retributions.
// func testRetributionStoreInit(frs FailingRetributionStore, t *testing.T) {
// 	// Make sure that a new retribution store starts empty.
// 	if count := countRetributions(t, frs); count != 0 {
// 		t.Fatalf("expected 0 retributions, found %v", count)
// 	}
// }

// // testRetributionStoreRemoveEmpty ensures that a retribution store will not
// // fail or panic if it is instructed to remove an entry while empty.
// func testRetributionStoreRemoveEmpty(frs FailingRetributionStore, t *testing.T) {
// 	testRetributionStoreRemoves(frs, t, false)
// }

// // testRetributionStoreOverwrite ensures that attempts to write retribution
// // information regarding a channel point that already exists does not change the
// // total number of entries held by the retribution store.
// func testRetributionStoreOverwrite(frs FailingRetributionStore, t *testing.T) {
// 	// Initially, add all retributions to store.
// 	testRetributionStoreAdds(frs, t, false)

// 	// Overwrite the initial entries again.
// 	for i, retInfo := range retributions {
// 		if err := frs.Add(&retInfo); err != nil {
// 			t.Fatalf("unable to add to retribution %v to store: %v",
// 				i, err)
// 		}
// 	}

// 	// Check that retribution store still has 2 entries.
// 	if count := countRetributions(t, frs); count != 2 {
// 		t.Fatalf("expected 2 retributions, found %v", count)
// 	}
// }

// // testRetributionStoreAdds adds all of the test retributions to the database,
// // ensuring that the total number of elements increases by exactly 1 after each
// // operation.  If the `failing` flag is provide, the test will restart the
// // database and confirm that the delta is still 1.
// func testRetributionStoreAdds(
// 	frs FailingRetributionStore,
// 	t *testing.T,
// 	failing bool) {

// 	// Iterate over retributions, adding each from the store. If we are
// 	// testing the store under failures, we restart the store and verify
// 	// that the contents are the same.
// 	for i, retInfo := range retributions {
// 		// Snapshot number of entries before and after the addition.
// 		nbefore := countRetributions(t, frs)
// 		if err := frs.Add(&retInfo); err != nil {
// 			t.Fatalf("unable to add to retribution %v to store: %v",
// 				i, err)
// 		}
// 		nafter := countRetributions(t, frs)

// 		// Check that only one retribution was added.
// 		if nafter-nbefore != 1 {
// 			t.Fatalf("expected %v retributions, found %v",
// 				nbefore+1, nafter)
// 		}

// 		if failing {
// 			frs.Restart()

// 			// Check that retribution store has persisted addition
// 			// after restarting.
// 			nrestart := countRetributions(t, frs)
// 			if nrestart-nbefore != 1 {
// 				t.Fatalf("expected %v retributions, found %v",
// 					nbefore+1, nrestart)
// 			}
// 		}
// 	}
// }

// // testRetributionStoreRemoves removes all of the test retributions to the
// // database, ensuring that the total number of elements decreases by exactly 1
// // after each operation.  If the `failing` flag is provide, the test will
// // restart the database and confirm that the delta is the same.
// func testRetributionStoreRemoves(
// 	frs FailingRetributionStore,
// 	t *testing.T,
// 	failing bool) {

// 	// Iterate over retributions, removing each from the store. If we are
// 	// testing the store under failures, we restart the store and verify
// 	// that the contents are the same.
// 	for i, retInfo := range retributions {
// 		// Snapshot number of entries before and after the removal.
// 		nbefore := countRetributions(t, frs)
// 		err := frs.Remove(&retInfo.chanPoint)
// 		switch {
// 		case nbefore == 0 && err == nil:

// 		case nbefore > 0 && err != nil:
// 			t.Fatalf("unable to remove to retribution %v "+
// 				"from store: %v", i, err)
// 		}
// 		nafter := countRetributions(t, frs)

// 		// If the store is empty, increment nbefore to simulate the
// 		// removal of one element.
// 		if nbefore == 0 {
// 			nbefore++
// 		}

// 		// Check that only one retribution was removed.
// 		if nbefore-nafter != 1 {
// 			t.Fatalf("expected %v retributions, found %v",
// 				nbefore-1, nafter)
// 		}

// 		if failing {
// 			frs.Restart()

// 			// Check that retribution store has persisted removal
// 			// after restarting.
// 			nrestart := countRetributions(t, frs)
// 			if nbefore-nrestart != 1 {
// 				t.Fatalf("expected %v retributions, found %v",
// 					nbefore-1, nrestart)
// 			}
// 		}
// 	}
// }

// // testRetributionStoreForAll iterates over the current entries in the
// // retribution store, ensuring that each entry in the database is unique, and
// // corresponds to exactly one of the entries in the test vector. If the
// // `failing` flag is provide, the test will restart the database and confirm
// // that the entries again validate against the test vectors.
// func testRetributionStoreForAll(
// 	frs FailingRetributionStore,
// 	t *testing.T,
// 	failing bool) {

// 	// nrets is the number of retributions in the test vector
// 	nrets := len(retributions)

// 	// isRestart indicates whether or not the database has been restarted.
// 	// When testing for failures, this allows the test case to make a second
// 	// attempt without causing a subsequent restart on the second pass.
// 	var isRestart bool

// restartCheck:
// 	// Construct a set of all channel points presented by the store. Entries
// 	// are only be added to the set if their corresponding retribution
// 	// information matches the test vector.
// 	var foundSet = make(map[wire.OutPoint]struct{})

// 	// Iterate through the stored retributions, checking to see if we have
// 	// an equivalent retribution in the test vector. This will return an
// 	// error unless all persisted retributions exist in the test vector.
// 	if err := frs.ForAll(func(ret *retributionInfo) error {
// 		// Fetch the retribution information from the test vector. If
// 		// the entry does not exist, the test returns an error.
// 		if exRetInfo, ok := retributionMap[ret.chanPoint]; ok {
// 			// Compare the presented retribution information with
// 			// the expected value, fail if they are inconsistent.
// 			if !reflect.DeepEqual(ret, &exRetInfo) {
// 				return fmt.Errorf("unexpected retribution "+
// 					"retrieved from db --\n"+
// 					"want: %#v\ngot: %#v", exRetInfo, ret,
// 				)
// 			}

// 			// Retribution information from database matches the
// 			// test vector, record the channel point in the found
// 			// map.
// 			foundSet[ret.chanPoint] = struct{}{}

// 		} else {
// 			return fmt.Errorf("unknown retribution retrieved "+
// 				"from db: %v", ret)
// 		}

// 		return nil
// 	}); err != nil {
// 		t.Fatalf("failed to iterate over persistent retributions: %v",
// 			err)
// 	}

// 	// Check that retribution store emits nrets entries
// 	if count := countRetributions(t, frs); count != nrets {
// 		t.Fatalf("expected %v retributions, found %v", nrets, count)
// 	}

// 	// Confirm that all of the retributions emitted from the iteration
// 	// correspond to unique channel points.
// 	nunique := len(foundSet)
// 	if nunique != nrets {
// 		t.Fatalf("expected %v unique retributions, only found %v",
// 			nrets, nunique)
// 	}

// 	// If in failure mode on only on first pass, restart the database and
// 	// rexecute the test.
// 	if failing && !isRestart {
// 		frs.Restart()
// 		isRestart = true

// 		goto restartCheck
// 	}
// }

func initZkBreachedState(t *testing.T) (*zkBreachArbiter,
	*lnwallet.LightningChannel, *lnwallet.LightningChannel,
	*lnwallet.LocalForceCloseSummary, chan *ZkContractBreachEvent,
	func(), func()) {
	// Create a pair of channels using a notifier that allows us to signal
	// a spend of the funding transaction. Alice's channel will be the on
	// observing a breach.
	alice, bob, cleanUpChans, err := createInitChannels(1)
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

	// Send one HTLC to Bob and perform a state transition to lock it in.
	htlcAmount := lnwire.NewMSatFromSatoshis(20000)
	htlc, _ := createHTLC(0, htlcAmount)
	if _, err := alice.AddHTLC(htlc, nil); err != nil {
		t.Fatalf("alice unable to add htlc: %v", err)
	}
	if _, err := bob.ReceiveHTLC(htlc); err != nil {
		t.Fatalf("bob unable to recv add htlc: %v", err)
	}
	if err := forceStateTransition(alice, bob); err != nil {
		t.Fatalf("Can't update the channel state: %v", err)
	}

	// Generate the force close summary at this point in time, this will
	// serve as the old state bob will broadcast.
	bobClose, err := bob.ForceClose()
	if err != nil {
		t.Fatalf("unable to force close bob's channel: %v", err)
	}

	// Now send another HTLC and perform a state transition, this ensures
	// Alice is ahead of the state Bob will broadcast.
	htlc2, _ := createHTLC(1, htlcAmount)
	if _, err := alice.AddHTLC(htlc2, nil); err != nil {
		t.Fatalf("alice unable to add htlc: %v", err)
	}
	if _, err := bob.ReceiveHTLC(htlc2); err != nil {
		t.Fatalf("bob unable to recv add htlc: %v", err)
	}
	if err := forceStateTransition(alice, bob); err != nil {
		t.Fatalf("Can't update the channel state: %v", err)
	}

	return brar, alice, bob, bobClose, custContractBreaches, cleanUpChans,
		cleanUpArb
}

// // TestBreachHandoffSuccess tests that a channel's close observer properly
// // delivers retribution information to the breach arbiter in response to a
// // breach close. This test verifies correctness in the event that the handoff
// // experiences no interruptions.
// func TestZkBreachHandoffSuccess(t *testing.T) {
// 	brar, alice, _, bobClose, contractBreaches,
// 		cleanUpChans, cleanUpArb := initBreachedState(t)
// 	defer cleanUpChans()
// 	defer cleanUpArb()

// 	chanPoint := alice.ChanPoint

// 	// Signal a spend of the funding transaction and wait for the close
// 	// observer to exit.
// 	breach := &ContractBreachEvent{
// 		ChanPoint:  *chanPoint,
// 		ProcessACK: make(chan error, 1),
// 		BreachRetribution: &lnwallet.BreachRetribution{
// 			BreachTransaction: bobClose.CloseTx,
// 			LocalOutputSignDesc: &input.SignDescriptor{
// 				Output: &wire.TxOut{
// 					PkScript: breachKeys[0],
// 				},
// 			},
// 		},
// 	}
// 	contractBreaches <- breach

// 	// We'll also wait to consume the ACK back from the breach arbiter.
// 	select {
// 	case err := <-breach.ProcessACK:
// 		if err != nil {
// 			t.Fatalf("handoff failed: %v", err)
// 		}
// 	case <-time.After(time.Second * 15):
// 		t.Fatalf("breach arbiter didn't send ack back")
// 	}

// 	// After exiting, the breach arbiter should have persisted the
// 	// retribution information and the channel should be shown as pending
// 	// force closed.
// 	assertArbiterBreach(t, brar, chanPoint)

// 	// Send another breach event. Since the handoff for this channel was
// 	// already ACKed, the breach arbiter should immediately ACK and ignore
// 	// this event.
// 	breach = &ContractBreachEvent{
// 		ChanPoint:  *chanPoint,
// 		ProcessACK: make(chan error, 1),
// 		BreachRetribution: &lnwallet.BreachRetribution{
// 			BreachTransaction: bobClose.CloseTx,
// 			LocalOutputSignDesc: &input.SignDescriptor{
// 				Output: &wire.TxOut{
// 					PkScript: breachKeys[0],
// 				},
// 			},
// 		},
// 	}

// 	contractBreaches <- breach

// 	// We'll also wait to consume the ACK back from the breach arbiter.
// 	select {
// 	case err := <-breach.ProcessACK:
// 		if err != nil {
// 			t.Fatalf("handoff failed: %v", err)
// 		}
// 	case <-time.After(time.Second * 15):
// 		t.Fatalf("breach arbiter didn't send ack back")
// 	}

// 	// State should not have changed.
// 	assertArbiterBreach(t, brar, chanPoint)
// }

// // TestBreachHandoffFail tests that a channel's close observer properly
// // delivers retribution information to the breach arbiter in response to a
// // breach close. This test verifies correctness in the event that the breach
// // arbiter fails to write the information to disk, and that a subsequent attempt
// // at the handoff succeeds.
// func TestBreachHandoffFail(t *testing.T) {
// 	brar, alice, _, bobClose, contractBreaches,
// 		cleanUpChans, cleanUpArb := initBreachedState(t)
// 	defer cleanUpChans()
// 	defer cleanUpArb()

// 	// Before alerting Alice of the breach, instruct our failing retribution
// 	// store to fail the next database operation, which we expect to write
// 	// the information handed off by the channel's close observer.
// 	fstore := brar.cfg.Store.(*failingRetributionStore)
// 	fstore.FailNextAdd(nil)

// 	// Signal the notifier to dispatch spend notifications of the funding
// 	// transaction using the transaction from bob's closing summary.
// 	chanPoint := alice.ChanPoint
// 	breach := &ContractBreachEvent{
// 		ChanPoint:  *chanPoint,
// 		ProcessACK: make(chan error, 1),
// 		BreachRetribution: &lnwallet.BreachRetribution{
// 			BreachTransaction: bobClose.CloseTx,
// 			LocalOutputSignDesc: &input.SignDescriptor{
// 				Output: &wire.TxOut{
// 					PkScript: breachKeys[0],
// 				},
// 			},
// 		},
// 	}
// 	contractBreaches <- breach

// 	// We'll also wait to consume the ACK back from the breach arbiter.
// 	select {
// 	case err := <-breach.ProcessACK:
// 		if err == nil {
// 			t.Fatalf("breach write should have failed")
// 		}
// 	case <-time.After(time.Second * 15):
// 		t.Fatalf("breach arbiter didn't send ack back")
// 	}

// 	// Since the handoff failed, the breach arbiter should not show the
// 	// channel as breached, and the channel should also not have been marked
// 	// pending closed.
// 	assertNoArbiterBreach(t, brar, chanPoint)
// 	assertNotPendingClosed(t, alice)

// 	brar, cleanUpArb, err := createTestZkArbiter(
// 		t, contractBreaches, alice.State().Db,
// 	)
// 	if err != nil {
// 		t.Fatalf("unable to initialize test breach arbiter: %v", err)
// 	}
// 	defer cleanUpArb()

// 	// Signal a spend of the funding transaction and wait for the close
// 	// observer to exit. This time we are allowing the handoff to succeed.
// 	breach = &ContractBreachEvent{
// 		ChanPoint:  *chanPoint,
// 		ProcessACK: make(chan error, 1),
// 		BreachRetribution: &lnwallet.BreachRetribution{
// 			BreachTransaction: bobClose.CloseTx,
// 			LocalOutputSignDesc: &input.SignDescriptor{
// 				Output: &wire.TxOut{
// 					PkScript: breachKeys[0],
// 				},
// 			},
// 		},
// 	}

// 	contractBreaches <- breach

// 	select {
// 	case err := <-breach.ProcessACK:
// 		if err != nil {
// 			t.Fatalf("handoff failed: %v", err)
// 		}
// 	case <-time.After(time.Second * 15):
// 		t.Fatalf("breach arbiter didn't send ack back")
// 	}

// 	// Check that the breach was properly recorded in the breach arbiter,
// 	// and that the close observer marked the channel as pending closed
// 	// before exiting.
// 	assertArbiterBreach(t, brar, chanPoint)
// }

// type publAssertion func(*testing.T, map[wire.OutPoint]*wire.MsgTx,
// 	chan *wire.MsgTx)

// type breachTest struct {
// 	name string

// 	// spend2ndLevel requests that second level htlcs be spent *again*, as
// 	// if by a remote party or watchtower. The outpoint of the second level
// 	// htlc is in effect "readded" to the set of inputs.
// 	spend2ndLevel bool

// 	// sendFinalConf informs the test to send a confirmation for the justice
// 	// transaction before asserting the arbiter is cleaned up.
// 	sendFinalConf bool

// 	// whenNonZeroInputs is called after spending an input but there are
// 	// further inputs to spend in the test.
// 	whenNonZeroInputs publAssertion

// 	// whenZeroInputs is called after spending an input but there are no
// 	// further inputs to spend in the test.
// 	whenZeroInputs publAssertion
// }

// var (
// 	// commitSpendTx is used to spend commitment outputs.
// 	commitSpendTx = &wire.MsgTx{
// 		TxOut: []*wire.TxOut{
// 			{Value: 500000000},
// 		},
// 	}
// 	// htlc2ndLevlTx is used to transition an htlc output on the commitment
// 	// transaction to a second level htlc.
// 	htlc2ndLevlTx = &wire.MsgTx{
// 		TxOut: []*wire.TxOut{
// 			{Value: 20000},
// 		},
// 	}
// 	// htlcSpendTx is used to spend from a second level htlc.
// 	htlcSpendTx = &wire.MsgTx{
// 		TxOut: []*wire.TxOut{
// 			{Value: 10000},
// 		},
// 	}
// )

var zkBreachTests = []breachTest{
	{
		name:          "all spends",
		spend2ndLevel: true,
		whenNonZeroInputs: func(t *testing.T,
			inputs map[wire.OutPoint]*wire.MsgTx,
			publTx chan *wire.MsgTx) {

			var tx *wire.MsgTx
			select {
			case tx = <-publTx:
			case <-time.After(5 * time.Second):
				t.Fatalf("tx was not published")
			}

			// The justice transaction should have thee same number
			// of inputs as we are tracking in the test.
			if len(tx.TxIn) != len(inputs) {
				t.Fatalf("expected justice txn to have %d "+
					"inputs, found %d", len(inputs),
					len(tx.TxIn))
			}

			// Ensure that each input exists on the justice
			// transaction.
			for in := range inputs {
				findInputIndex(t, in, tx)
			}

		},
		whenZeroInputs: func(t *testing.T,
			inputs map[wire.OutPoint]*wire.MsgTx,
			publTx chan *wire.MsgTx) {

			// Sanity check to ensure the brar doesn't try to
			// broadcast another sweep, since all outputs have been
			// spent externally.
			select {
			case <-publTx:
				t.Fatalf("tx published unexpectedly")
			case <-time.After(50 * time.Millisecond):
			}
		},
	},
}

// TestBreachSpends checks the behavior of the breach arbiter in response to
// spend events on a channels outputs by asserting that it properly removes or
// modifies the inputs from the justice txn.
func TestZkBreachSpends(t *testing.T) {
	for _, test := range zkBreachTests {
		tc := test
		t.Run(tc.name, func(t *testing.T) {
			testZkBreachSpends(t, tc)
		})
	}
}

func testZkBreachSpends(t *testing.T, test breachTest) {
	brar, alice, _, _, custContractBreaches,
		cleanUpChans, cleanUpArb := initZkBreachedState(t)
	defer cleanUpChans()
	defer cleanUpArb()

	// var (
	// 	height       = bobClose.ChanSnapshot.CommitHeight
	// 	forceCloseTx = bobClose.CloseTx
	// 	chanPoint    = alice.ChanPoint
	// 	publTx       = make(chan *wire.MsgTx)
	// 	publErr      error
	// 	publMtx      sync.Mutex
	// )

	// Use hardcoded zkchannel fundingOutpoint
	var escrowTxidHash chainhash.Hash
	err := chainhash.Decode(&escrowTxidHash, escrowTxid)
	if err != nil {
		t.Fatal(err)
	}

	fundingOut := &wire.OutPoint{
		Hash:  escrowTxidHash,
		Index: uint32(0),
	}

	var (
		// height = uint64(0)
		// forceCloseTx   = custCloseTx
		forceCloseTxid = custCloseTxid
		chanPoint      = fundingOut
		publTx         = make(chan *wire.MsgTx)
		publErr        error
		publMtx        sync.Mutex
	)

	// Make PublishTransaction always return ErrDoubleSpend to begin with.
	publErr = lnwallet.ErrDoubleSpend
	brar.cfg.PublishTransaction = func(tx *wire.MsgTx) error {
		publTx <- tx

		publMtx.Lock()
		defer publMtx.Unlock()
		return publErr
	}

	// // Notify the breach arbiter about the breach.
	// retribution, err := lnwallet.NewBreachRetribution(
	// 	alice.State(), height, 1,
	// )
	// if err != nil {
	// 	t.Fatalf("unable to create breach retribution: %v", err)
	// }

	// Hard coded example
	// TODO: Move to test list
	ZkCustBreachInfo := contractcourt.ZkBreachInfo{
		EscrowTxid:    chainhash.Hash{0xb5, 0x7f, 0x93, 0x7a, 0x5e, 0xb2, 0xd3, 0x3b, 0x27, 0x17, 0xc, 0x0, 0x9d, 0xc2, 0xbe, 0x3d, 0x4d, 0x89, 0xe3, 0x7f, 0xdf, 0x47, 0xa7, 0xd, 0x75, 0x32, 0x1d, 0xde, 0xf5, 0xe5, 0xe, 0x57},
		CloseTxid:     chainhash.Hash{0x13, 0x4b, 0x35, 0x6, 0x1c, 0x7e, 0xfa, 0xf0, 0x3d, 0x85, 0xe2, 0xed, 0x69, 0xe7, 0xed, 0xd7, 0xb6, 0x10, 0xa, 0x18, 0xd1, 0xff, 0x46, 0xfd, 0x25, 0x58, 0x38, 0x3a, 0xcb, 0xd0, 0x52, 0x2e},
		ClosePkScript: []uint8{0x0, 0x20, 0xea, 0xb1, 0xb2, 0x1b, 0x12, 0x27, 0xf9, 0xa, 0x54, 0xc8, 0xb, 0xd0, 0xac, 0x99, 0x2f, 0xf5, 0x7d, 0xac, 0xa6, 0xb8, 0x95, 0x10, 0xc6, 0x82, 0x81, 0xbe, 0x97, 0xa9, 0x95, 0xbd, 0x2a, 0xc7},
		RevLock:       "6dad4b8de4ea7765e89a69def72048c6bd1b9f2fb143aec83d0b924c42adc074",
		RevSecret:     "6dad4b8de4ea7765e89a69def72048c6bd1b9f2fb143aec83d0b924c42adc074",
		CustClosePk:   "027160fb5e48252f02a00066dfa823d15844ad93e04f9c9b746e1f28ed4a1eaddb",
		Amount:        9990,
	}

	disputeKeyPriv, disputeKeyPub := btcec.PrivKeyFromBytes(btcec.S256(),
		alicesPrivKey)

	disputePk := hex.EncodeToString(disputeKeyPub.SerializeCompressed())
	disputeSk := hex.EncodeToString(disputeKeyPriv.Serialize())

	s := ""
	//var m map[string]interface{}
	var mm = map[string]interface{}{}

	merchState := libzkchannels.MerchState{
		Id:          &s,
		PkM:         &disputePk,
		SkM:         &disputeSk,
		HmacKey:     "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
		HmacKeyR:    "00000000000000000000000000000000",
		PayoutSk:    &disputeSk,
		PayoutPk:    &disputePk,
		DisputeSk:   &disputeSk,
		DisputePk:   &disputePk,
		ActivateMap: &mm,
		CloseTxMap:  &mm,
		NetConfig:   nil,
		DbUrl:       s,
	}

	channelState := libzkchannels.ChannelState{
		DustLimit:  330,
		KeyCom:     disputeSk,
		Name:       "name",
		ThirdParty: false,
		SelfDelay:  uint16(1487),
	}

	testDB, _ := zkchanneldb.SetupDB(brar.cfg.DBPath)

	_ = zkchanneldb.AddMerchState(testDB, merchState)
	_ = zkchanneldb.AddMerchField(testDB, channelState, "channelStateKey")

	testDB.Close()

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

	state := alice.State()
	err = state.CloseChannel(&channeldb.ChannelCloseSummary{
		ChanPoint:               state.FundingOutpoint,
		ChainHash:               state.ChainHash,
		RemotePub:               state.IdentityPub,
		CloseType:               channeldb.BreachClose,
		Capacity:                state.Capacity,
		IsPending:               true,
		ShortChanID:             state.ShortChanID(),
		RemoteCurrentRevocation: state.RemoteCurrentRevocation,
		RemoteNextRevocation:    state.RemoteNextRevocation,
		LocalChanConfig:         state.LocalChanCfg,
	})
	if err != nil {
		t.Fatalf("unable to close channel: %v", err)
	}

	// // After exiting, the breach arbiter should have persisted the
	// // retribution information and the channel should be shown as pending
	// // force closed.
	// assertArbiterBreach(t, brar, chanPoint)

	// // Assert that the database sees the channel as pending close, otherwise
	// // the breach arbiter won't be able to fully close it.
	// assertPendingClosed(t, alice)

	// Notify that the breaching transaction is confirmed, to trigger the
	// retribution logic.

	notifier := brar.cfg.Notifier.(*mockSpendNotifier)
	notifier.confChannel <- &chainntnfs.TxConfirmation{}

	// The breach arbiter should attempt to sweep all outputs on the
	// breached commitment. We'll pretend that the HTLC output has been
	// spent by the channel counter party's second level tx already.
	var tx *wire.MsgTx
	select {
	case tx = <-publTx:
	case <-time.After(5 * time.Second):
		t.Fatalf("tx was not published")
	}

	//// All outputs should initially spend from the force closed txn.
	//forceTxID := forceCloseTxid
	//for _, txIn := range tx.TxIn {
	//	if txIn.PreviousOutPoint.Hash.String() != forceTxID {
	//		t.Fatalf("og justice tx not spending commitment %v", forceTxID)
	//	}
	//}

	// localOutpoint := retribution.LocalOutpoint
	// remoteOutpoint := retribution.RemoteOutpoint
	// htlcOutpoint := retribution.HtlcRetributions[0].OutPoint

	// // Construct a map from outpoint on the force close to the transaction
	// // we want it to be spent by. As the test progresses, this map will be
	// // updated to contain only the set of commitment or second level
	// // outpoints that remain to be spent.
	// inputs := map[wire.OutPoint]*wire.MsgTx{
	// 	htlcOutpoint:   htlc2ndLevlTx,
	// 	localOutpoint:  commitSpendTx,
	// 	remoteOutpoint: commitSpendTx,
	// }

	// // Until no more inputs to spend remain, deliver the spend events and
	// // process the assertions prescribed by the test case.
	// for len(inputs) > 0 {
	// 	var (
	// 		op      wire.OutPoint
	// 		spendTx *wire.MsgTx
	// 	)

	// 	// Pick an outpoint at random from the set of inputs.
	// 	for op, spendTx = range inputs {
	// 		delete(inputs, op)
	// 		break
	// 	}

	// 	// Deliver the spend notification for the chosen transaction.
	// 	notifier.Spend(&op, 2, spendTx)

	// 	// When the second layer transfer is detected, add back the
	// 	// outpoint of the second layer tx so that we can spend it
	// 	// again. Only do so if the test requests this behavior.
	// 	spendTxID := spendTx.TxHash()
	// 	if test.spend2ndLevel && spendTxID == htlc2ndLevlTx.TxHash() {
	// 		// Create the second level outpoint that will be spent,
	// 		// the index is always zero for these 1-in-1-out txns.
	// 		spendOp := wire.OutPoint{Hash: spendTxID}
	// 		inputs[spendOp] = htlcSpendTx
	// 	}

	// 	if len(inputs) > 0 {
	// 		test.whenNonZeroInputs(t, inputs, publTx)
	// 	} else {
	// 		// Reset the publishing error so that any publication,
	// 		// made by the breach arbiter, if any, will succeed.
	// 		publMtx.Lock()
	// 		publErr = nil
	// 		publMtx.Unlock()
	// 		test.whenZeroInputs(t, inputs, publTx)
	// 	}
	// }

	// Deliver confirmation of sweep if the test expects it.
	if test.sendFinalConf {
		notifier.confChannel <- &chainntnfs.TxConfirmation{}
	}

	// // Assert that the channel is fully resolved.
	// assertBrarCleanup(t, brar, alice.ChanPoint, alice.State().Db)
}

// // findInputIndex returns the index of the input that spends from the given
// // outpoint. This method fails if the outpoint is not found.
// func findInputIndex(t *testing.T, op wire.OutPoint, tx *wire.MsgTx) int {
// 	t.Helper()

// 	inputIdx := -1
// 	for i, txIn := range tx.TxIn {
// 		if txIn.PreviousOutPoint == op {
// 			inputIdx = i
// 		}
// 	}
// 	if inputIdx == -1 {
// 		t.Fatalf("input %v in not found", op)
// 	}

// 	return inputIdx
// }

// // assertArbiterBreach checks that the breach arbiter has persisted the breach
// // information for a particular channel.
// func assertArbiterBreach(t *testing.T, brar *breachArbiter,
// 	chanPoint *wire.OutPoint) {

// 	t.Helper()

// 	isBreached, err := brar.IsBreached(chanPoint)
// 	if err != nil {
// 		t.Fatalf("unable to determine if channel is "+
// 			"breached: %v", err)
// 	}

// 	if !isBreached {
// 		t.Fatalf("channel %v was never marked breached",
// 			chanPoint)
// 	}

// }

// // assertNoArbiterBreach checks that the breach arbiter has not persisted the
// // breach information for a particular channel.
// func assertNoArbiterBreach(t *testing.T, brar *breachArbiter,
// 	chanPoint *wire.OutPoint) {

// 	t.Helper()

// 	isBreached, err := brar.IsBreached(chanPoint)
// 	if err != nil {
// 		t.Fatalf("unable to determine if channel is "+
// 			"breached: %v", err)
// 	}

// 	if isBreached {
// 		t.Fatalf("channel %v was marked breached",
// 			chanPoint)
// 	}
// }

// // assertBrarCleanup blocks until the given channel point has been removed the
// // retribution store and the channel is fully closed in the database.
// func assertBrarCleanup(t *testing.T, brar *breachArbiter,
// 	chanPoint *wire.OutPoint, db *channeldb.DB) {

// 	t.Helper()

// 	err := wait.NoError(func() error {
// 		isBreached, err := brar.IsBreached(chanPoint)
// 		if err != nil {
// 			return err
// 		}

// 		if isBreached {
// 			return fmt.Errorf("channel %v still breached",
// 				chanPoint)
// 		}

// 		closedChans, err := db.FetchClosedChannels(false)
// 		if err != nil {
// 			return err
// 		}

// 		for _, channel := range closedChans {
// 			switch {
// 			// Wrong channel.
// 			case channel.ChanPoint != *chanPoint:
// 				continue

// 			// Right channel, fully closed!
// 			case !channel.IsPending:
// 				return nil
// 			}

// 			// Still pending.
// 			return fmt.Errorf("channel %v still pending "+
// 				"close", chanPoint)
// 		}

// 		return fmt.Errorf("channel %v not closed", chanPoint)

// 	}, time.Second)
// 	if err != nil {
// 		t.Fatalf(err.Error())
// 	}
// }

// // assertPendingClosed checks that the channel has been marked pending closed in
// // the channel database.
// func assertPendingClosed(t *testing.T, c *lnwallet.LightningChannel) {
// 	t.Helper()

// 	closedChans, err := c.State().Db.FetchClosedChannels(true)
// 	if err != nil {
// 		t.Fatalf("unable to load pending closed channels: %v", err)
// 	}

// 	for _, chanSummary := range closedChans {
// 		if chanSummary.ChanPoint == *c.ChanPoint {
// 			return
// 		}
// 	}

// 	t.Fatalf("channel %v was not marked pending closed", c.ChanPoint)
// }

// // assertNotPendingClosed checks that the channel has not been marked pending
// // closed in the channel database.
// func assertNotPendingClosed(t *testing.T, c *lnwallet.LightningChannel) {
// 	t.Helper()

// 	closedChans, err := c.State().Db.FetchClosedChannels(true)
// 	if err != nil {
// 		t.Fatalf("unable to load pending closed channels: %v", err)
// 	}

// 	for _, chanSummary := range closedChans {
// 		if chanSummary.ChanPoint == *c.ChanPoint {
// 			t.Fatalf("channel %v was marked pending closed",
// 				c.ChanPoint)
// 		}
// 	}
// }

// createTestZkArbiter instantiates a breach arbiter with a failing retribution
// store, so that controlled failures can be tested.
func createTestZkArbiter(t *testing.T, custContractBreaches chan *ZkContractBreachEvent,
	db *channeldb.DB) (*zkBreachArbiter, func(), error) {

	// // Create a failing retribution store, that wraps a normal one.
	// store := newFailingRetributionStore(func() RetributionStore {
	// 	return newRetributionStore(db)
	// })

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
		PublishTransaction:   func(_ *wire.MsgTx) error { return nil },
		// Store:              store,
	})

	if err := ba.Start(); err != nil {
		return nil, nil, err
	}

	// The caller is responsible for closing the database.
	cleanUp := func() {
		ba.Stop()
	}

	return ba, cleanUp, nil
}

// // createInitChannels creates two initialized test channels funded with 10 BTC,
// // with 5 BTC allocated to each side. Within the channel, Alice is the
// // initiator.
// func createInitChannels(revocationWindow int) (*lnwallet.LightningChannel, *lnwallet.LightningChannel, func(), error) {

// 	aliceKeyPriv, aliceKeyPub := btcec.PrivKeyFromBytes(btcec.S256(),
// 		alicesPrivKey)
// 	bobKeyPriv, bobKeyPub := btcec.PrivKeyFromBytes(btcec.S256(),
// 		bobsPrivKey)

// 	channelCapacity, err := btcutil.NewAmount(10)
// 	if err != nil {
// 		return nil, nil, nil, err
// 	}

// 	channelBal := channelCapacity / 2
// 	aliceDustLimit := btcutil.Amount(200)
// 	bobDustLimit := btcutil.Amount(1300)
// 	csvTimeoutAlice := uint32(5)
// 	csvTimeoutBob := uint32(4)

// 	prevOut := &wire.OutPoint{
// 		Hash:  chainhash.Hash(testHdSeed),
// 		Index: 0,
// 	}
// 	fundingTxIn := wire.NewTxIn(prevOut, nil, nil)

// 	aliceCfg := channeldb.ChannelConfig{
// 		ChannelConstraints: channeldb.ChannelConstraints{
// 			DustLimit:        aliceDustLimit,
// 			MaxPendingAmount: lnwire.MilliSatoshi(rand.Int63()),
// 			ChanReserve:      0,
// 			MinHTLC:          0,
// 			MaxAcceptedHtlcs: uint16(rand.Int31()),
// 			CsvDelay:         uint16(csvTimeoutAlice),
// 		},
// 		MultiSigKey: keychain.KeyDescriptor{
// 			PubKey: aliceKeyPub,
// 		},
// 		RevocationBasePoint: keychain.KeyDescriptor{
// 			PubKey: aliceKeyPub,
// 		},
// 		PaymentBasePoint: keychain.KeyDescriptor{
// 			PubKey: aliceKeyPub,
// 		},
// 		DelayBasePoint: keychain.KeyDescriptor{
// 			PubKey: aliceKeyPub,
// 		},
// 		HtlcBasePoint: keychain.KeyDescriptor{
// 			PubKey: aliceKeyPub,
// 		},
// 	}
// 	bobCfg := channeldb.ChannelConfig{
// 		ChannelConstraints: channeldb.ChannelConstraints{
// 			DustLimit:        bobDustLimit,
// 			MaxPendingAmount: lnwire.MilliSatoshi(rand.Int63()),
// 			ChanReserve:      0,
// 			MinHTLC:          0,
// 			MaxAcceptedHtlcs: uint16(rand.Int31()),
// 			CsvDelay:         uint16(csvTimeoutBob),
// 		},
// 		MultiSigKey: keychain.KeyDescriptor{
// 			PubKey: bobKeyPub,
// 		},
// 		RevocationBasePoint: keychain.KeyDescriptor{
// 			PubKey: bobKeyPub,
// 		},
// 		PaymentBasePoint: keychain.KeyDescriptor{
// 			PubKey: bobKeyPub,
// 		},
// 		DelayBasePoint: keychain.KeyDescriptor{
// 			PubKey: bobKeyPub,
// 		},
// 		HtlcBasePoint: keychain.KeyDescriptor{
// 			PubKey: bobKeyPub,
// 		},
// 	}

// 	bobRoot, err := chainhash.NewHash(bobKeyPriv.Serialize())
// 	if err != nil {
// 		return nil, nil, nil, err
// 	}
// 	bobPreimageProducer := shachain.NewRevocationProducer(*bobRoot)
// 	bobFirstRevoke, err := bobPreimageProducer.AtIndex(0)
// 	if err != nil {
// 		return nil, nil, nil, err
// 	}
// 	bobCommitPoint := input.ComputeCommitmentPoint(bobFirstRevoke[:])

// 	aliceRoot, err := chainhash.NewHash(aliceKeyPriv.Serialize())
// 	if err != nil {
// 		return nil, nil, nil, err
// 	}
// 	alicePreimageProducer := shachain.NewRevocationProducer(*aliceRoot)
// 	aliceFirstRevoke, err := alicePreimageProducer.AtIndex(0)
// 	if err != nil {
// 		return nil, nil, nil, err
// 	}
// 	aliceCommitPoint := input.ComputeCommitmentPoint(aliceFirstRevoke[:])

// 	aliceCommitTx, bobCommitTx, err := lnwallet.CreateCommitmentTxns(
// 		channelBal, channelBal, &aliceCfg, &bobCfg, aliceCommitPoint,
// 		bobCommitPoint, *fundingTxIn, channeldb.SingleFunderTweaklessBit,
// 	)
// 	if err != nil {
// 		return nil, nil, nil, err
// 	}

// 	alicePath, err := ioutil.TempDir("", "alicedb")
// 	if err != nil {
// 		return nil, nil, nil, err
// 	}

// 	dbAlice, err := channeldb.Open(alicePath)
// 	if err != nil {
// 		return nil, nil, nil, err
// 	}

// 	bobPath, err := ioutil.TempDir("", "bobdb")
// 	if err != nil {
// 		return nil, nil, nil, err
// 	}

// 	dbBob, err := channeldb.Open(bobPath)
// 	if err != nil {
// 		return nil, nil, nil, err
// 	}

// 	estimator := chainfee.NewStaticEstimator(12500, 0)
// 	feePerKw, err := estimator.EstimateFeePerKW(1)
// 	if err != nil {
// 		return nil, nil, nil, err
// 	}

// 	// TODO(roasbeef): need to factor in commit fee?
// 	aliceCommit := channeldb.ChannelCommitment{
// 		CommitHeight:  0,
// 		LocalBalance:  lnwire.NewMSatFromSatoshis(channelBal),
// 		RemoteBalance: lnwire.NewMSatFromSatoshis(channelBal),
// 		FeePerKw:      btcutil.Amount(feePerKw),
// 		CommitFee:     8688,
// 		CommitTx:      aliceCommitTx,
// 		CommitSig:     bytes.Repeat([]byte{1}, 71),
// 	}
// 	bobCommit := channeldb.ChannelCommitment{
// 		CommitHeight:  0,
// 		LocalBalance:  lnwire.NewMSatFromSatoshis(channelBal),
// 		RemoteBalance: lnwire.NewMSatFromSatoshis(channelBal),
// 		FeePerKw:      btcutil.Amount(feePerKw),
// 		CommitFee:     8688,
// 		CommitTx:      bobCommitTx,
// 		CommitSig:     bytes.Repeat([]byte{1}, 71),
// 	}

// 	var chanIDBytes [8]byte
// 	if _, err := io.ReadFull(crand.Reader, chanIDBytes[:]); err != nil {
// 		return nil, nil, nil, err
// 	}

// 	shortChanID := lnwire.NewShortChanIDFromInt(
// 		binary.BigEndian.Uint64(chanIDBytes[:]),
// 	)

// 	aliceChannelState := &channeldb.OpenChannel{
// 		LocalChanCfg:            aliceCfg,
// 		RemoteChanCfg:           bobCfg,
// 		IdentityPub:             aliceKeyPub,
// 		FundingOutpoint:         *prevOut,
// 		ShortChannelID:          shortChanID,
// 		ChanType:                channeldb.SingleFunderTweaklessBit,
// 		IsInitiator:             true,
// 		Capacity:                channelCapacity,
// 		RemoteCurrentRevocation: bobCommitPoint,
// 		RevocationProducer:      alicePreimageProducer,
// 		RevocationStore:         shachain.NewRevocationStore(),
// 		LocalCommitment:         aliceCommit,
// 		RemoteCommitment:        aliceCommit,
// 		Db:                      dbAlice,
// 		Packager:                channeldb.NewChannelPackager(shortChanID),
// 		FundingTxn:              testTx,
// 	}
// 	bobChannelState := &channeldb.OpenChannel{
// 		LocalChanCfg:            bobCfg,
// 		RemoteChanCfg:           aliceCfg,
// 		IdentityPub:             bobKeyPub,
// 		FundingOutpoint:         *prevOut,
// 		ShortChannelID:          shortChanID,
// 		ChanType:                channeldb.SingleFunderTweaklessBit,
// 		IsInitiator:             false,
// 		Capacity:                channelCapacity,
// 		RemoteCurrentRevocation: aliceCommitPoint,
// 		RevocationProducer:      bobPreimageProducer,
// 		RevocationStore:         shachain.NewRevocationStore(),
// 		LocalCommitment:         bobCommit,
// 		RemoteCommitment:        bobCommit,
// 		Db:                      dbBob,
// 		Packager:                channeldb.NewChannelPackager(shortChanID),
// 	}

// 	aliceSigner := &mockSigner{aliceKeyPriv}
// 	bobSigner := &mockSigner{bobKeyPriv}

// 	alicePool := lnwallet.NewSigPool(1, aliceSigner)
// 	channelAlice, err := lnwallet.NewLightningChannel(
// 		aliceSigner, aliceChannelState, alicePool,
// 	)
// 	if err != nil {
// 		return nil, nil, nil, err
// 	}
// 	alicePool.Start()

// 	bobPool := lnwallet.NewSigPool(1, bobSigner)
// 	channelBob, err := lnwallet.NewLightningChannel(
// 		bobSigner, bobChannelState, bobPool,
// 	)
// 	if err != nil {
// 		return nil, nil, nil, err
// 	}
// 	bobPool.Start()

// 	addr := &net.TCPAddr{
// 		IP:   net.ParseIP("127.0.0.1"),
// 		Port: 18556,
// 	}
// 	if err := channelAlice.State().SyncPending(addr, 101); err != nil {
// 		return nil, nil, nil, err
// 	}

// 	addr = &net.TCPAddr{
// 		IP:   net.ParseIP("127.0.0.1"),
// 		Port: 18555,
// 	}
// 	if err := channelBob.State().SyncPending(addr, 101); err != nil {
// 		return nil, nil, nil, err
// 	}

// 	cleanUpFunc := func() {
// 		dbBob.Close()
// 		dbAlice.Close()
// 		os.RemoveAll(bobPath)
// 		os.RemoveAll(alicePath)
// 	}

// 	// Now that the channel are open, simulate the start of a session by
// 	// having Alice and Bob extend their revocation windows to each other.
// 	err = initRevocationWindows(channelAlice, channelBob, revocationWindow)
// 	if err != nil {
// 		return nil, nil, nil, err
// 	}

// 	return channelAlice, channelBob, cleanUpFunc, nil
// }

// // initRevocationWindows simulates a new channel being opened within the p2p
// // network by populating the initial revocation windows of the passed
// // commitment state machines.
// //
// // TODO(conner) remove code duplication
// func initRevocationWindows(chanA, chanB *lnwallet.LightningChannel, windowSize int) error {
// 	aliceNextRevoke, err := chanA.NextRevocationKey()
// 	if err != nil {
// 		return err
// 	}
// 	if err := chanB.InitNextRevocation(aliceNextRevoke); err != nil {
// 		return err
// 	}

// 	bobNextRevoke, err := chanB.NextRevocationKey()
// 	if err != nil {
// 		return err
// 	}
// 	if err := chanA.InitNextRevocation(bobNextRevoke); err != nil {
// 		return err
// 	}

// 	return nil
// }

// // createHTLC is a utility function for generating an HTLC with a given
// // preimage and a given amount.
// // TODO(conner) remove code duplication
// func createHTLC(data int, amount lnwire.MilliSatoshi) (*lnwire.UpdateAddHTLC, [32]byte) {
// 	preimage := bytes.Repeat([]byte{byte(data)}, 32)
// 	paymentHash := sha256.Sum256(preimage)

// 	var returnPreimage [32]byte
// 	copy(returnPreimage[:], preimage)

// 	return &lnwire.UpdateAddHTLC{
// 		ID:          uint64(data),
// 		PaymentHash: paymentHash,
// 		Amount:      amount,
// 		Expiry:      uint32(5),
// 	}, returnPreimage
// }

// // forceStateTransition executes the necessary interaction between the two
// // commitment state machines to transition to a new state locking in any
// // pending updates.
// // TODO(conner) remove code duplication
// func forceStateTransition(chanA, chanB *lnwallet.LightningChannel) error {
// 	aliceSig, aliceHtlcSigs, _, err := chanA.SignNextCommitment()
// 	if err != nil {
// 		return err
// 	}
// 	if err = chanB.ReceiveNewCommitment(aliceSig, aliceHtlcSigs); err != nil {
// 		return err
// 	}

// 	bobRevocation, _, err := chanB.RevokeCurrentCommitment()
// 	if err != nil {
// 		return err
// 	}
// 	bobSig, bobHtlcSigs, _, err := chanB.SignNextCommitment()
// 	if err != nil {
// 		return err
// 	}

// 	_, _, _, _, err = chanA.ReceiveRevocation(bobRevocation)
// 	if err != nil {
// 		return err
// 	}
// 	if err := chanA.ReceiveNewCommitment(bobSig, bobHtlcSigs); err != nil {
// 		return err
// 	}

// 	aliceRevocation, _, err := chanA.RevokeCurrentCommitment()
// 	if err != nil {
// 		return err
// 	}
// 	_, _, _, _, err = chanB.ReceiveRevocation(aliceRevocation)
// 	if err != nil {
// 		return err
// 	}

// 	return nil
// }
