package zkchannels

import (
	"fmt"

	"github.com/lightningnetwork/lnd/libzkchannels"
	"github.com/lightningnetwork/lnd/zkchanneldb"
)

// UpdateCustChannelState updates the channel status
func UpdateCustChannelState(DBPath string, zkChannelName string, newStatus string) error {

	zkCustDB, err := zkchanneldb.OpenZkChannelBucket(zkChannelName, DBPath)
	if err != nil {
		return fmt.Errorf("zkchanneldb.OpenZkChannelBucket, %v", err)
	}
	defer zkCustDB.Close()

	custState, err := zkchanneldb.GetCustState(zkCustDB, zkChannelName)
	if err != nil {
		return fmt.Errorf("zkchanneldb.GetCustState, %v", err)
	}

	switch newStatus {
	case "Open":
		custState, err = libzkchannels.CustomerChangeChannelStatusToOpen(custState)
		if err != nil {
			return fmt.Errorf("CustomerChangeChannelStatusToOpen, %v", err)

		}
	case "PendingClose":
		custState, err = libzkchannels.CustomerChangeChannelStatusToPendingClose(custState)
		if err != nil {
			return fmt.Errorf("CustomerChangeChannelStatusToPendingClose, %v", err)

		}
	case "ConfirmedClose":
		custState, err = libzkchannels.CustomerChangeChannelStatusToConfirmedClose(custState)
		if err != nil {
			return fmt.Errorf("CustomerChangeChannelStatusToConfirmedClose, %v", err)

		}
	default:
		return fmt.Errorf("unrecognised status: %v", newStatus)
	}

	err = zkchanneldb.AddCustState(zkCustDB, zkChannelName, custState)
	if err != nil {
		return fmt.Errorf("zkchanneldb.AddCustState, %v", err)

	}
	return nil
}

func GetCustChannelState(DBPath string, zkChannelName string) (status string, err error) {

	zkCustDB, err := zkchanneldb.OpenZkChannelBucket(zkChannelName, DBPath)
	if err != nil {
		return "", fmt.Errorf("zkchanneldb.OpenZkChannelBucket, %v", err)

	}
	defer zkCustDB.Close()

	custState, err := zkchanneldb.GetCustState(zkCustDB, zkChannelName)
	if err != nil {
		return "", fmt.Errorf("zkchanneldb.GetCustState, %v", err)

	}

	return custState.ChannelStatus, nil
}

func UpdateMerchChannelState(DBPath string, escrowTxid string, newStatus string) error {

	zkMerchDB, err := zkchanneldb.OpenMerchBucket(DBPath)
	if err != nil {
		return fmt.Errorf("zkchanneldb.OpenMerchBucket, %v", err)

	}
	defer zkMerchDB.Close()

	merchState, err := zkchanneldb.GetMerchState(zkMerchDB)
	if err != nil {
		return fmt.Errorf("zkchanneldb.GetMerchState, %v", err)

	}

	switch newStatus {
	case "Open":
		merchState, err = libzkchannels.MerchantChangeChannelStatusToOpen(escrowTxid, merchState)
		if err != nil {
			return fmt.Errorf("MerchantChangeChannelStatusToOpen, %v", err)
		}
	case "PendingClose":
		merchState, err = libzkchannels.MerchantChangeChannelStatusToPendingClose(escrowTxid, merchState)
		if err != nil {
			return fmt.Errorf("MerchantChangeChannelStatusToPendingClose, %v", err)

		}
	case "ConfirmedClose":
		merchState, err = libzkchannels.MerchantChangeChannelStatusToConfirmedClose(escrowTxid, merchState)
		if err != nil {
			return fmt.Errorf("MerchantChangeChannelStatusToConfirmedClose, %v", err)

		}
	default:
		return fmt.Errorf("unrecognised status: %v", newStatus)
	}

	err = zkchanneldb.AddMerchState(zkMerchDB, merchState)
	if err != nil {
		return fmt.Errorf("zkchanneldb.AddMerchState, %v", err)

	}
	return nil
}

func GetMerchChannelState(DBPath string, escrowTxid string) (status string, err error) {

	zkMerchDB, err := zkchanneldb.OpenMerchBucket(DBPath)
	if err != nil {
		return "", fmt.Errorf("zkchanneldb.OpenMerchBucket, %v", err)

	}
	defer zkMerchDB.Close()

	merchState, err := zkchanneldb.GetMerchState(zkMerchDB)
	if err != nil {
		return "", fmt.Errorf("zkchanneldb.GetMerchState, %v", err)

	}

	// Flip bytes from Little Endiand to Big Endian
	// This works because hex strings are of even size
	s := ""
	for i := 0; i < len(escrowTxid)/2; i++ {
		s = escrowTxid[i*2:i*2+2] + s
	}
	escrowTxidBigEn := s

	status, ok := (*merchState.ChannelStatusMap)[escrowTxidBigEn].(string)
	if ok != true {
		return "", fmt.Errorf("error in getMerchChannelState")
	}
	return status, nil
}
