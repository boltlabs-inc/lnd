// +build kvdb_etcd

package etcd

import (
	"bytes"
	"testing"

	"github.com/btcsuite/btcwallet/walletdb"
	"github.com/stretchr/testify/assert"
)

func TestCopy(t *testing.T) {
	t.Parallel()

	f := NewEtcdTestFixture(t)
	defer f.Cleanup()

	db, err := newEtcdBackend(f.BackendConfig())
	assert.NoError(t, err)

	err = db.Update(func(tx walletdb.ReadWriteTx) error {
		// "apple"
		apple, err := tx.CreateTopLevelBucket([]byte("apple"))
		assert.NoError(t, err)
		assert.NotNil(t, apple)

		assert.NoError(t, apple.Put([]byte("key"), []byte("val")))
		return nil
	})

	// Expect non-zero copy.
	var buf bytes.Buffer

	assert.NoError(t, db.Copy(&buf))
	assert.Greater(t, buf.Len(), 0)
	assert.Nil(t, err)

	expected := map[string]string{
		bkey("apple"):        bval("apple"),
		vkey("key", "apple"): "val",
	}
	assert.Equal(t, expected, f.Dump())
}
