package zkchanneldb

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/boltdb/bolt"
	"github.com/lightningnetwork/lnd/libzkchannels"
	"github.com/stretchr/testify/assert"
)

func setupTestDB(t *testing.T, filename string, bucketName string) *bolt.DB {
	testDir, err := ioutil.TempDir("", "zkchanneldb")
	if err != nil {
		t.Fatalf("unable to create temp directory: %v", err)
	}
	dbPath := path.Join(testDir, filename)
	db, err := SetupWithBucketNameDB(dbPath, bucketName)
	if err != nil {
		t.Fatalf("unable to open db: %v", err)
	}
	assert.Equal(t, dbPath, db.Path())

	return db
}

func tearDownTestDB(db *bolt.DB) {
	path := db.Path()
	db.Close()
	os.RemoveAll(path)
}

func TestSetupDB(t *testing.T) {
	db := setupTestDB(t, "zkmerch.db", "bucket")
	tearDownTestDB(db)
}

func TestCreateZkChannelBucket(t *testing.T) {
	testDir, err := ioutil.TempDir("", "zkchanneldb")
	if err != nil {
		t.Fatalf("unable to create temp directory: %v", err)
	}
	dbPath := path.Join(testDir, "zkcust.db")
	InitDB(dbPath)
	db, err := CreateZkChannelBucket("channelName", dbPath)
	if err != nil {
		t.Fatalf("unable to open zk channel bucket: %v", err)
	}
	assert.Equal(t, dbPath, db.Path())

	db.Close()
	os.RemoveAll(dbPath)
}

func TestOpenZkChannelBucket(t *testing.T) {
	testDir, err := ioutil.TempDir("", "zkchanneldb")
	if err != nil {
		t.Fatalf("unable to create temp directory: %v", err)
	}
	dbPath := path.Join(testDir, "zkcust.db")
	InitDB(dbPath)
	db, err := CreateZkChannelBucket("channelName", dbPath)
	if err != nil {
		t.Fatalf("unable to open zk channel bucket: %v", err)
	}
	db.Close()
	db, err = OpenZkChannelBucket("channelName", dbPath)
	if err != nil {
		t.Fatalf("unable to open zk channel bucket: %v", err)
	}
	assert.Equal(t, dbPath, db.Path())

	db.Close()
	os.RemoveAll(dbPath)
}

func TestOpenZkClaimBucket(t *testing.T) {
	testDir, err := ioutil.TempDir("", "zkchanneldb")
	if err != nil {
		t.Fatalf("unable to create temp directory: %v", err)
	}
	dbPath := path.Join(testDir, "zkclaim.db")
	db, err := OpenZkClaimBucket("txid", dbPath)
	if err != nil {
		t.Fatalf("unable to open zk claim bucket: %v", err)
	}
	assert.Equal(t, dbPath, db.Path())
	db.Close()

	buckets, err := Buckets(dbPath)
	if err != nil {
		t.Fatalf("unable to create temp directory: %v", err)
	}
	assert.Equal(t, 1, len(buckets))
	assert.Equal(t, "txid", buckets[0])

	db.Close()
	os.RemoveAll(dbPath)
}

func TestInitDB(t *testing.T) {
	testDir, err := ioutil.TempDir("", "zkchanneldb")
	if err != nil {
		t.Fatalf("unable to create temp directory: %v", err)
	}
	dbPath := path.Join(testDir, "zkclaim.db")
	err = InitDB(dbPath)
	if err != nil {
		t.Fatalf("unable to open zk claim bucket: %v", err)
	}

	db, err := OpenZkClaimBucket("txid", dbPath)
	if err != nil {
		t.Fatalf("unable to open zk claim bucket: %v", err)
	}
	assert.Equal(t, dbPath, db.Path())
	db.Close()
	os.RemoveAll(dbPath)
}

func TestAddField(t *testing.T) {
	db := setupTestDB(t, "zkmerch.db", "bucket")
	someObject := map[string]string{"test": "test"}
	err := AddField(db, "bucket", someObject, "someobject")
	assert.Nil(t, err)
	var actual map[string]string
	err = GetField(db, "bucket", "someobject", &actual)
	assert.Nil(t, err)
	assert.Equal(t, someObject, actual)
	tearDownTestDB(db)
}

func TestAddStringField(t *testing.T) {
	db := setupTestDB(t, "zkmerch.db", "bucket")
	someString := "test"
	err := AddStringField(db, "bucket", someString, "somestring")
	assert.Nil(t, err)
	actual, err := GetStringField(db, "bucket", "somestring")
	assert.Nil(t, err)
	assert.Equal(t, someString, actual)
	tearDownTestDB(db)
}

func TestAddMerchField(t *testing.T) {
	db := setupTestDB(t, "zkmerch.db", "merch-bucket")
	someObject := map[string]string{"test": "test"}
	err := AddMerchField(db, someObject, "someobject")
	assert.Nil(t, err)
	var actual map[string]string
	err = GetMerchField(db, "someobject", &actual)
	assert.Nil(t, err)
	assert.Equal(t, someObject, actual)
	tearDownTestDB(db)
}

func TestAddMerchState(t *testing.T) {
	db := setupTestDB(t, "zkmerch.db", "merch-bucket")
	merchState := libzkchannels.MerchState{}
	err := AddMerchState(db, merchState)
	assert.Nil(t, err)
	actual, err := GetMerchState(db)
	assert.Nil(t, err)
	assert.Equal(t, merchState, actual)
	tearDownTestDB(db)
}

func TestAddCustState(t *testing.T) {
	db := setupTestDB(t, "zkmerch.db", "channel")
	custState := libzkchannels.CustState{}
	err := AddCustState(db, "channel", custState)
	assert.Nil(t, err)
	actual, err := GetCustState(db, "channel")
	assert.Nil(t, err)
	assert.Equal(t, custState, actual)
	tearDownTestDB(db)
}
