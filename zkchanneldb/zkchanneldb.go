package zkchanneldb

import (
	"encoding/json"
	"fmt"
	"time"

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

// Config type
type Config struct {
	Height   float64   `json:"height"`
	Birthday time.Time `json:"birthday"`
}

// Entry type
type Entry struct {
	Calories int    `json:"calories"`
	Food     string `json:"food"`
}

// func main() {
// 	db, err := SetupZkChannelDB()
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	defer db.Close()

// 	conf := Config{Height: 186.0, Birthday: time.Now()}
// 	err = setConfig(db, conf)
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	err = addWeight(db, "85.0", time.Now())
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	err = addEntry(db, 100, "apple", time.Now())
// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	err = addEntry(db, 100, "orange", time.Now().AddDate(0, 0, -2))
// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	err = db.View(func(tx *bolt.Tx) error {
// 		conf := tx.Bucket([]byte("DB")).Get([]byte("CONFIG"))
// 		fmt.Printf("Config: %s\n", conf)
// 		return nil
// 	})
// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	err = db.View(func(tx *bolt.Tx) error {
// 		b := tx.Bucket([]byte("DB")).Bucket([]byte("WEIGHT"))
// 		b.ForEach(func(k, v []byte) error {
// 			fmt.Println(string(k), "hey", string(v))
// 			return nil
// 		})
// 		return nil
// 	})
// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	err = db.View(func(tx *bolt.Tx) error {
// 		c := tx.Bucket([]byte("DB")).Bucket([]byte("ENTRIES")).Cursor()
// 		min := []byte(time.Now().AddDate(0, 0, -7).Format(time.RFC3339))
// 		max := []byte(time.Now().AddDate(0, 0, 0).Format(time.RFC3339))
// 		for k, v := c.Seek(min); k != nil && bytes.Compare(k, max) <= 0; k, v = c.Next() {
// 			fmt.Println(string(k), string(v))
// 		}
// 		return nil
// 	})
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// }

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
	fmt.Println("zkmerch db Setup Done")
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
			return fmt.Errorf("could not create merchState bucket: %v", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("could not set up zk merch buckets, %v", err)
	}
	fmt.Println("zkmerch db Setup Done")
	return db, nil
}

func setConfig(db *bolt.DB, config Config) error {
	confBytes, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("could not marshal config json: %v", err)
	}
	err = db.Update(func(tx *bolt.Tx) error {
		err = tx.Bucket([]byte("DB")).Put([]byte("CONFIG"), confBytes)
		if err != nil {
			return fmt.Errorf("could not set config: %v", err)
		}
		return nil
	})
	fmt.Println("Set Config")
	return err
}

func addWeight(db *bolt.DB, weight string, date time.Time) error {
	err := db.Update(func(tx *bolt.Tx) error {
		err := tx.Bucket([]byte("DB")).Bucket([]byte("WEIGHT")).Put([]byte(date.Format(time.RFC3339)), []byte(weight))
		if err != nil {
			return fmt.Errorf("could not insert weight: %v", err)
		}
		return nil
	})
	fmt.Println("Added Weight")
	return err
}

// AddmerchState adds merchState to the zkMerchDB
func AddmerchState(db *bolt.DB, merchStateBytes []byte) error {

	err := db.Update(func(tx *bolt.Tx) error {
		err := tx.Bucket(MerchBucket).Put([]byte("merchStateKey"), merchStateBytes)
		if err != nil {
			return fmt.Errorf("could not insert entry: %v", err)
		}

		return nil
	})
	fmt.Println("Added Entry")
	return err
}

func addEntry(db *bolt.DB, calories int, food string, date time.Time) error {
	entry := Entry{Calories: calories, Food: food}
	entryBytes, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("could not marshal entry json: %v", err)
	}
	err = db.Update(func(tx *bolt.Tx) error {
		err := tx.Bucket([]byte("DB")).Bucket([]byte("ENTRIES")).Put([]byte(date.Format(time.RFC3339)), entryBytes)
		if err != nil {
			return fmt.Errorf("could not insert entry: %v", err)
		}

		return nil
	})
	fmt.Println("Added Entry")
	return err
}
