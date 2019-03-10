package urkel

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

const testDir = "data"

var h256 = &Sha256{}

func makeCountingKey(num int) []byte {
	return []byte(fmt.Sprintf("name-%v", num))
}

func fillTree(tx *MutableTree, num int) {
	for i := 0; i < num; i++ {
		k := fmt.Sprintf("name-%v", i)
		//key := makeKey(k)
		v := fmt.Sprintf("value-%v", i)
		tx.Set([]byte(k), []byte(v))
	}
}

func TestStoreBasics(t *testing.T) {
	assert := assert.New(t)

	memdb := OpenDb()
	// Create a new Tree and insert many K/Vs
	tree := NewUrkelTree(memdb, h256)
	defer tree.Close()
	tx := tree.Transaction()

	fillTree(tx, 10)
	tx.Commit()

	snapshot := tree.Snapshot()
	r1 := snapshot.Get([]byte("name-5"))
	assert.Equal([]byte("value-5"), r1)
}

func TestStoreWithBadger(t *testing.T) {
	testDir := "data"
	defer func() {
		os.RemoveAll(testDir)
		os.Mkdir(testDir, 0700)
	}()

	assert := assert.New(t)
	kv, err := OpenBadger(testDir)
	assert.Nil(err)
	assert.NotNil(kv)

	tree := NewUrkelTree(kv, h256)
	tx := tree.Transaction()

	fillTree(tx, 10)
	tx.Commit()

	snapshot := tree.Snapshot()
	r1 := snapshot.Get([]byte("name-5"))
	assert.Equal([]byte("value-5"), r1)

	// Close/reopen and check the root hash
	firstHash := snapshot.RootHash()
	tree.Close()

	kv1, err := OpenBadger(testDir)
	assert.Nil(err)
	assert.NotNil(kv)
	tree1 := NewUrkelTree(kv1, h256)
	snapshot1 := tree1.Snapshot()
	secondHash := snapshot1.RootHash()

	assert.Equal(firstHash, secondHash)
}

func TestStoreRemove(t *testing.T) {
	assert := assert.New(t)

	memdb := OpenDb()
	tree := NewUrkelTree(memdb, h256)
	defer tree.Close()

	tx := tree.Transaction()
	fillTree(tx, 10)
	tx.Commit()

	tx1 := tree.Transaction()
	tx1.Remove([]byte("name-3"))
	tx1.Commit()

	snap := tree.Snapshot()
	r1 := snap.Get([]byte("name-3"))
	assert.Nil(r1)

	r1 = snap.Get([]byte("name-2"))
	assert.Equal([]byte("value-2"), r1)
}

func TestStoreDoProofs(t *testing.T) {
	assert := assert.New(t)

	memdb := OpenDb()
	tree := NewUrkelTree(memdb, h256)
	defer tree.Close()

	tx := tree.Transaction()
	fillTree(tx, 10)
	tx.Commit()

	keyToProve := []byte("name-4")
	expectedValue := []byte("value-4")

	// Prove the key is there...
	snap := tree.Snapshot()
	prf1 := snap.Proof(keyToProve)
	assert.NotNil(prf1)
	assert.Equal(EXISTS, prf1.Type)
	assert.Equal(expectedValue, prf1.Value)
	assert.Nil(prf1.Hash)
	assert.True(prf1.Depth() > 0)

	// Verify against the root
	r2 := prf1.Verify(snap.RootHash(), keyToProve, h256, 256)
	assert.Equal(OK, r2.Code)
	assert.Equal(expectedValue, r2.Value)

	// Add test to prove non-exist
}

func TestStoreSmallCommits(t *testing.T) {
	assert := assert.New(t)

	memdb := OpenDb()
	tree := NewUrkelTree(memdb, h256)
	defer tree.Close()
	tx := tree.Transaction()

	// Commit 1
	key1 := makeCountingKey(1)
	tx.Set(key1, []byte(fmt.Sprintf("value-%v", 1))) //W
	keya := makeCountingKey(55)
	tx.Set(keya, []byte(fmt.Sprintf("value-%v", 55))) //W
	tx.Commit()

	// Commit 2
	tx2 := tree.Transaction()
	key2 := makeCountingKey(2)
	tx2.Set(key2, []byte(fmt.Sprintf("value-%v", 2))) // W
	tx2.Commit()

	// Read
	snap1 := tree.Snapshot()
	r1 := snap1.Get(key1)
	assert.Equal([]byte("value-1"), r1)

	// Commit 3
	tx3 := tree.Transaction()
	key3 := makeCountingKey(3)
	tx3.Set(key3, []byte(fmt.Sprintf("value-%v", 3))) // W
	tx3.Commit()

	// Read
	snap2 := tree.Snapshot()
	r := snap2.Get(keya)
	assert.Equal([]byte("value-55"), r)

	r = snap2.Get(key2)
	assert.Equal([]byte("value-2"), r)

	r = snap2.Get(key3)
	assert.Equal([]byte("value-3"), r)

	r = snap2.Get(key1)
	assert.Equal([]byte("value-1"), r)
}
