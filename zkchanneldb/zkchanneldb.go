package zkchanneldb

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/lightningnetwork/lnd/libzkchannels"

	"github.com/boltdb/bolt"
)

var (
	// MerchBucket contains the merch information
	MerchBucket = "merch-bucket"
)

// SetupMerchDB creates the zkchanneldb for the merchant
func SetupMerchDB(path string) (*bolt.DB, error) {
	return SetupWithBucketNameDB(path, MerchBucket)
}

func SetupWithBucketNameDB(path string, bucketName string) (*bolt.DB, error) {
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("could not open db, %v", err)
	}
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucketName))
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

// InitDB creates the zkchanneldb at path
func InitDB(path string) error {
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return fmt.Errorf("could not open db, %v", err)
	}
	return db.Close()
}

// Buckets returns a list of all buckets.
func Buckets(path string) ([]string, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Println(err)
		return nil, err
	}

	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		fmt.Println(err)
		return nil, err
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
		return nil, err
	}
	return bucketList, err
}

// OpenMerchBucket opens the zkchanneldb for the merchant
func OpenMerchBucket(path string) (*bolt.DB, error) {
	return OpenZkChannelBucket(MerchBucket, path)
}

// CreateZkChannelBucket opens the bucket for a zkchannel
func CreateZkChannelBucket(zkChannelName string, dbPath string) (*bolt.DB, error) {
	// make sure the db already exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("tried to access bucket %v in %v but the db does not exist", zkChannelName, dbPath)
	}

	BucketName := []byte(zkChannelName)

	db, err := bolt.Open(dbPath, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("could not open db, %v", err)
	}
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucket(BucketName)
		if err != nil {
			return fmt.Errorf("could not create customer bucket: %v", err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("Create could not set up zk cust buckets, %v", err)
	}
	return db, nil
}

// OpenZkChannelBucket opens the bucket for a zkchannel
func OpenZkChannelBucket(zkChannelName string, dbPath string) (*bolt.DB, error) {
	// make sure the db already exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("tried to access bucket %v in %v but the db does not exist", zkChannelName, dbPath)
	}

	BucketName := []byte(zkChannelName)

	db, err := bolt.Open(dbPath, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("could not open db, %v", err)
	}
	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(BucketName)
		if b == nil {
			return fmt.Errorf("%v bucket in %v does not exist", zkChannelName, dbPath)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("Error opening bucket: %v", err)
	}
	return db, nil
}

// OpenZkClaimBucket opens or creates the bucket for a zkchannel
func OpenZkClaimBucket(escrowTxid string, dbPath string) (*bolt.DB, error) {
	return SetupWithBucketNameDB(dbPath, escrowTxid)
}

// AddMerchState adds merchState to the zkMerchDB
func AddMerchState(db *bolt.DB, merchState libzkchannels.MerchState) error {
	merchStateBytes, err := json.Marshal(merchState)
	if err != nil {
		return err
	}
	err = db.Update(func(tx *bolt.Tx) error {
		err := tx.Bucket([]byte(MerchBucket)).Put([]byte("merchStateKey"), merchStateBytes)
		if err != nil {
			return fmt.Errorf("could not insert entry: %v", err)
		}

		return nil
	})
	return err
}

// AddCustState adds custState to the zkCustDB
func AddCustState(db *bolt.DB, zkChannelName string, custState libzkchannels.CustState) error {
	custStateBytes, err := json.Marshal(custState)
	if err != nil {
		return err
	}
	BucketName := []byte(zkChannelName)

	err = db.Update(func(tx *bolt.Tx) error {
		err := tx.Bucket(BucketName).Put([]byte("custStateKey"), custStateBytes)
		if err != nil {
			return fmt.Errorf("could not insert entry: %v", err)
		}

		return nil
	})
	return err
}

// AddMerchField adds arbitrary field to the zkMerchDB
func AddMerchField(db *bolt.DB, field interface{}, fieldName string) error {
	return AddField(db, MerchBucket, field, fieldName)
}

// AddField adds arbitrary field to the DB
func AddField(db *bolt.DB, zkChannelName string, field interface{}, fieldName string) error {

	fieldBytes, err := json.Marshal(field)
	if err != nil {
		return err
	}
	BucketName := []byte(zkChannelName)

	err = db.Update(func(tx *bolt.Tx) error {
		err := tx.Bucket(BucketName).Put([]byte(fieldName), fieldBytes)
		if err != nil {
			return fmt.Errorf("could not insert entry: %v", err)
		}

		return nil
	})
	return err
}

// AddStringField adds arbitrary field to the DB
func AddStringField(db *bolt.DB, zkChannelName string, field string, fieldName string) error {
	BucketName := []byte(zkChannelName)

	err := db.Update(func(tx *bolt.Tx) error {
		err := tx.Bucket(BucketName).Put([]byte(fieldName), []byte(field))
		if err != nil {
			return fmt.Errorf("could not insert entry: %v", err)
		}

		return nil
	})
	return err
}

// GetCustState custState from zkCustDB
func GetCustState(db *bolt.DB, zkChannelName string) (libzkchannels.CustState, error) {
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
	var custState libzkchannels.CustState
	err = json.Unmarshal(fieldBytes, &custState)
	return custState, err
}

// GetMerchState gets merchState from zkMerchDB
func GetMerchState(db *bolt.DB) (libzkchannels.MerchState, error) {

	var fieldBytes []byte
	err := db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket([]byte(MerchBucket)).Cursor()
		_, v := c.Seek([]byte("merchStateKey"))
		fieldBytes = v
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	var merchState libzkchannels.MerchState
	err = json.Unmarshal(fieldBytes, &merchState)
	return merchState, err
}

// GetField gets a field from DB (works for zkCustDB and zkMerchDB)
func GetField(db *bolt.DB, bucketName string, fieldName string, out interface{}) error {
	fieldBytes, err := getFieldInBytes(db, bucketName, fieldName)
	if err != nil {
		return err
	}

	return json.Unmarshal(fieldBytes, &out)
}

// GetStringField gets a field from DB (works for zkCustDB and zkMerchDB)
func GetStringField(db *bolt.DB, bucketName string, fieldName string) (string, error) {
	fieldBytes, err := getFieldInBytes(db, bucketName, fieldName)
	return string(fieldBytes), err
}

func getFieldInBytes(db *bolt.DB, bucketName string, fieldName string) ([]byte, error) {
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
func GetMerchField(db *bolt.DB, fieldName string, out interface{}) error {
	return GetField(db, MerchBucket, fieldName, out)
}
