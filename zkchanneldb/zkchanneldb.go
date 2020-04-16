package zkchanneldb

import (
	"fmt"
	"log"
	"os"

	"github.com/boltdb/bolt"
)

var (
	// MerchBucket contains the merch information
	MerchBucket = []byte("merch-bucket")
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

	return db, nil
}

// Buckets returns a list of all buckets.
func Buckets(path string) []string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Println(err)
		return nil
	}

	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	defer db.Close()

	var bucketList []string
	err = db.View(func(tx *bolt.Tx) error {
		return tx.ForEach(func(name []byte, _ *bolt.Bucket) error {
			// fmt.Println(string(name))
			bucketList = append(bucketList, string(name))
			return nil
		})
	})
	if err != nil {
		fmt.Println(err)
		return nil
	}
	return bucketList
}

// OpenZkChannelBucket opens or creates the bucket for a zkchannel
func OpenZkChannelBucket(zkChannelName string) (*bolt.DB, error) {
	BucketName := []byte(zkChannelName)

	db, err := bolt.Open("zkcust.db", 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("could not open db, %v", err)
	}
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(BucketName)
		if err != nil {
			return fmt.Errorf("could not create customer bucket: %v", err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("could not set up zk cust buckets, %v", err)
	}
	return db, nil
}

// OpenZkClaimBucket opens or creates the bucket for a zkchannel
func OpenZkClaimBucket(escrowTxid string) (*bolt.DB, error) {
	BucketName := []byte(escrowTxid)

	db, err := bolt.Open("zkclaim.db", 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("could not open db, %v", err)
	}
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(BucketName)
		if err != nil {
			return fmt.Errorf("could not create customer claim bucket: %v", err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("could not set up zk cust claim buckets, %v", err)
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
	return err
}

// AddCustState adds custState to the zkCustDB
func AddCustState(db *bolt.DB, zkChannelName string, custStateBytes []byte) error {
	BucketName := []byte(zkChannelName)

	err := db.Update(func(tx *bolt.Tx) error {
		err := tx.Bucket(BucketName).Put([]byte("custStateKey"), custStateBytes)
		if err != nil {
			return fmt.Errorf("could not insert entry: %v", err)
		}

		return nil
	})
	return err
}

// AddMerchField adds arbitrary field to the zkMerchDB
func AddMerchField(db *bolt.DB, fieldBytes []byte, fieldName string) error {

	err := db.Update(func(tx *bolt.Tx) error {
		err := tx.Bucket(MerchBucket).Put([]byte(fieldName), fieldBytes)
		if err != nil {
			return fmt.Errorf("could not insert entry: %v", err)
		}

		return nil
	})
	return err
}

// AddCustField adds arbitrary field to the zkCustDB
func AddCustField(db *bolt.DB, zkChannelName string, fieldBytes []byte, fieldName string) error {
	BucketName := []byte(zkChannelName)

	err := db.Update(func(tx *bolt.Tx) error {
		err := tx.Bucket(BucketName).Put([]byte(fieldName), fieldBytes)
		if err != nil {
			return fmt.Errorf("could not insert entry: %v", err)
		}

		return nil
	})
	return err
}

// GetCustState custState from zkCustDB
func GetCustState(db *bolt.DB, zkChannelName string) ([]byte, error) {
	BucketName := []byte(zkChannelName)
	var fieldBytes []byte
	err := db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(BucketName).Cursor()
		_, v := c.Seek([]byte("custStateKey"))
		fieldBytes = v
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	return fieldBytes, err
}

// GetMerchState gets merchState from zkMerchDB
func GetMerchState(db *bolt.DB) ([]byte, error) {

	var fieldBytes []byte
	err := db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(MerchBucket).Cursor()
		_, v := c.Seek([]byte("merchStateKey"))
		fieldBytes = v
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	return fieldBytes, err
}

// GetField gets a field from DB (works for zkCustDB and zkMerchDB)
func GetField(db *bolt.DB, bucketName string, fieldName string) ([]byte, error) {
	BucketName := []byte(bucketName)

	var fieldBytes []byte
	err := db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(BucketName).Cursor()
		_, v := c.Seek([]byte(fieldName))
		fieldBytes = v
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	return fieldBytes, err
}

// GetMerchField gets a field from zkMerchDB
func GetMerchField(db *bolt.DB, fieldName string) ([]byte, error) {

	var fieldBytes []byte
	err := db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(MerchBucket).Cursor()
		_, v := c.Seek([]byte(fieldName))
		fieldBytes = v
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	return fieldBytes, err
}
