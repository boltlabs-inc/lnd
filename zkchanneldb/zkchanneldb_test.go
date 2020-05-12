package zkchanneldb

import (
	"github.com/boltdb/bolt"
	"io/ioutil"
	"os"
	"path"
	"testing"
)

func setupTestDB(t *testing.T, filename string) *bolt.DB {
	testDir, err := ioutil.TempDir("", "zkchanneldb")
	if err != nil {
		t.Fatalf("unable to create temp directory: %v", err)
	}
	db, err := SetupDB(path.Join(testDir, filename))
	if err != nil {
		t.Fatalf("unable to create temp directory: %v", err)
	}

	return db
}

func tearDownTestDB(db *bolt.DB) {
	db.Close()
	os.RemoveAll(db.Path())
}

func TestSetupDB(t *testing.T) {
	db := setupTestDB(t, "zkmerch.db")
	tearDownTestDB(db)
}
