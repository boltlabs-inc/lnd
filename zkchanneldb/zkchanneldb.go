package zkchanneldb

import (
	"fmt"

	"github.com/boltdb/bolt"
)

var (
	// MerchBucket contains all zkChannel information stored on the merchant's node
	MerchBucket = []byte("merchant-bucket")

	// MerchStateBucket contains the merch-state information
	MerchStateBucket = []byte("merch-state-bucket")

	// CustBucket contains all zkChannel information stored on the customer's node
	CustBucket = []byte("customer-bucket")

	// CustStateBucket contains the cust-state information. There is one
	// cust-state per zkChannel.
	CustStateBucket = []byte("cust-state-bucket")
)

// SetupZkMerchDB creates the zkchanneldb for the merchant
func SetupZkMerchDB() (*bolt.DB, error) {
	db, err := bolt.Open("zkmerch.db", 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("could not open db, %v", err)
	}
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(MerchBucket)
		if err != nil {
			return fmt.Errorf("could not create merchant bucket: %v", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("could not set up zk merch buckets, %v", err)
	}
	fmt.Println("ZKMerchDB opened")
	return db, nil
}

// SetupZkCustDB creates the zkchanneldb for the customer
func SetupZkCustDB() (*bolt.DB, error) {
	db, err := bolt.Open("zkcust.db", 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("could not open db, %v", err)
	}
	err = db.Update(func(tx *bolt.Tx) error {
		customer, err := tx.CreateBucketIfNotExists(CustBucket)
		if err != nil {
			return fmt.Errorf("could not create customer bucket: %v", err)
		}
		_, err = customer.CreateBucketIfNotExists(CustStateBucket)
		if err != nil {
			return fmt.Errorf("could not create custState bucket: %v", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("could not set up zk cust buckets, %v", err)
	}
	fmt.Println("ZkCustDB opened")
	return db, nil
}

// AddMerchState adds merchState to the zkMerchDB
func AddMerchState(db *bolt.DB, merchStateBytes []byte) error {

	err := db.Update(func(tx *bolt.Tx) error {
		err := tx.Bucket(MerchBucket).Put([]byte("merchStateKey"), merchStateBytes)
		if err != nil {
			return fmt.Errorf("could not insert entry: %v", err)
		}

		return nil
	})
	fmt.Println("Added merchState Entry")
	return err
}

// AddCustState adds custState to the zkCustDB
func AddCustState(db *bolt.DB, custStateBytes []byte) error {

	err := db.Update(func(tx *bolt.Tx) error {
		err := tx.Bucket(CustBucket).Put([]byte("custStateKey"), custStateBytes)
		if err != nil {
			return fmt.Errorf("could not insert entry: %v", err)
		}

		return nil
	})
	fmt.Println("Added custState Entry")
	return err
}

// AddField adds arbitrary field to the zkMerchDB
func AddMerchField(db *bolt.DB, fieldBytes []byte, fieldName string) error {

	err := db.Update(func(tx *bolt.Tx) error {
		err := tx.Bucket(MerchBucket).Put([]byte(fieldName), fieldBytes)
		if err != nil {
			return fmt.Errorf("could not insert entry: %v", err)
		}

		return nil
	})
	fmt.Println("Added merch Entry:", fieldName)
	return err
}

// AddField adds arbitrary field to the zkCustDB
func AddCustField(db *bolt.DB, fieldBytes []byte, fieldName string) error {

	err := db.Update(func(tx *bolt.Tx) error {
		err := tx.Bucket(CustBucket).Put([]byte(fieldName), fieldBytes)
		if err != nil {
			return fmt.Errorf("could not insert entry: %v", err)
		}

		return nil
	})
	fmt.Println("Added cust Entry:", fieldName)
	return err
}

// // Below are old functions for adding specific fields,
// // now replaced by AddMerchField and AddCustField

// // AddCustChannelToken adds channelToken to the zkCustDB
// func AddCustChannelToken(db *bolt.DB, channelTokenBytes []byte) error {

// 	err := db.Update(func(tx *bolt.Tx) error {
// 		err := tx.Bucket(CustBucket).Put([]byte("channelTokenKey"), channelTokenBytes)
// 		if err != nil {
// 			return fmt.Errorf("could not insert entry: %v", err)
// 		}

// 		return nil
// 	})
// 	fmt.Println("Added cust channelToken Entry")
// 	return err
// }

// // AddMerchChannelToken adds merchState to the zkMerchDB
// func AddMerchChannelToken(db *bolt.DB, channelTokenBytes []byte) error {

// 	err := db.Update(func(tx *bolt.Tx) error {
// 		err := tx.Bucket(MerchBucket).Put([]byte("channelTokenKey"), channelTokenBytes)
// 		if err != nil {
// 			return fmt.Errorf("could not insert entry: %v", err)
// 		}

// 		return nil
// 	})
// 	fmt.Println("Added merch channelToken Entry")
// 	return err
// }

// // AddCustChannelState adds channelState to the zkCustDB
// func AddCustChannelState(db *bolt.DB, channelStateBytes []byte) error {

// 	err := db.Update(func(tx *bolt.Tx) error {
// 		err := tx.Bucket(CustBucket).Put([]byte("channelStateKey"), channelStateBytes)
// 		if err != nil {
// 			return fmt.Errorf("could not insert entry: %v", err)
// 		}

// 		return nil
// 	})
// 	fmt.Println("Added cust channelState Entry")
// 	return err
// }

// // AddNewCustState adds newCustState to the zkCustDB
// func AddNewCustState(db *bolt.DB, newCustStateBytes []byte) error {

// 	err := db.Update(func(tx *bolt.Tx) error {
// 		err := tx.Bucket(CustBucket).Put([]byte("newCustStateKey"), newCustStateBytes)
// 		if err != nil {
// 			return fmt.Errorf("could not insert entry: %v", err)
// 		}

// 		return nil
// 	})
// 	fmt.Println("Added newCustState Entry")
// 	return err
// }

// // AddMerchChannelState adds ChannelState to the zkMerchDB
// func AddMerchChannelState(db *bolt.DB, channelStateBytes []byte) error {

// 	err := db.Update(func(tx *bolt.Tx) error {
// 		err := tx.Bucket(MerchBucket).Put([]byte("channelStateKey"), channelStateBytes)
// 		if err != nil {
// 			return fmt.Errorf("could not insert entry: %v", err)
// 		}

// 		return nil
// 	})
// 	fmt.Println("Added merch channelState Entry")
// 	return err
// }

// // AddZkChannelParams adds zkChannelParams to the zkMerchDB
// func AddZkChannelParams(db *bolt.DB, zkChanParamsBytes []byte) error {

// 	err := db.Update(func(tx *bolt.Tx) error {
// 		err := tx.Bucket(MerchBucket).Put([]byte("zkChannelParamsKey"), zkChanParamsBytes)
// 		if err != nil {
// 			return fmt.Errorf("could not insert entry: %v", err)
// 		}

// 		return nil
// 	})
// 	fmt.Println("Added merch zkMerchParams Entry")
// 	return err
// }
