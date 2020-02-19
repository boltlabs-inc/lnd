package zkchanneldb

import (
	"fmt"

	"github.com/boltdb/bolt"
)

var (
	// MerchBucket contains all zkChannel information stored on the merchant's node
	MerchBucket = []byte("merchant-bucket")

	// CustBucket contains all zkChannel information stored on the customer's node
	CustBucket = []byte("customer-bucket")
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
