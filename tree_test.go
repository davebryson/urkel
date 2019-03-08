package urkel

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

const testDir = "data"
const testFile = "data/0000000001"

var h256 = &Sha256{}

func makeKey(k string) []byte {
	return h256.Hash([]byte(k))
}

func removeTestFile() {
	os.RemoveAll(testFile)
}

func fillTree(tree *Tree, num int) {
	for i := 0; i < num; i++ {
		k := fmt.Sprintf("name-%v", i)
		key := makeKey(k)
		v := fmt.Sprintf("value-%v", i)
		tree.Insert(key, []byte(v))
	}
}

func TestStoreBasics(t *testing.T) {
	defer removeTestFile()
	assert := assert.New(t)

	// Create a new Tree and insert many K/Vs
	tree := UrkelTree(testDir, h256)

	fillTree(tree, 5)
	// Commit to store
	tree.Commit()

	key := makeKey("name-3")
	r1 := tree.Get(key)
	assert.Equal([]byte("value-3"), r1)
}

func TestStoreRemove(t *testing.T) {
	defer removeTestFile()
	assert := assert.New(t)

	tree := UrkelTree(testDir, h256)
	fillTree(tree, 5)
	tree.Commit()

	key := makeKey("name-3")
	r1 := tree.Get(key)
	assert.Equal([]byte("value-3"), r1)

	tree.Remove(key)
	//tree.Commit()

	r1 = tree.Get(makeKey("name-3"))
	assert.Nil(r1)

	r1 = tree.Get(makeKey("name-2"))
	assert.Equal([]byte("value-2"), r1)
}

func TestStoreDoProofs(t *testing.T) {
	defer removeTestFile()
	assert := assert.New(t)

	tree := UrkelTree(testDir, h256)
	fillTree(tree, 10)
	tree.Commit()

	keyToProve := makeKey("name-4")
	expectedValue := []byte("value-4")

	// Prove the key is there...
	prf1 := tree.Prove(keyToProve)
	assert.NotNil(prf1)
	assert.Equal(EXISTS, prf1.Type)
	assert.Equal(expectedValue, prf1.Value)
	assert.Nil(prf1.Hash)
	assert.True(prf1.Depth() > 0)

	// Verify against the root
	r2 := prf1.Verify(tree.RootHash(), keyToProve, h256, 256)
	assert.Equal(OK, r2.Code)
	assert.Equal(expectedValue, r2.Value)

	// Add test to prove non-exist
}

func TestStoreRecovery(t *testing.T) {
	defer removeTestFile()
	assert := assert.New(t)

	tree := UrkelTree(testDir, h256)
	key1 := makeKey(fmt.Sprintf("name-%v", 1))
	tree.Insert(key1, []byte(fmt.Sprintf("value-%v", 1)))
	//tree.Commit()
	/*tree.Commit()

	r1 := tree.Get(key)
	assert.Equal([]byte("value-1"), r1)*/
	//tree.Close()

	//tree := UrkelTree(testDir, h256)
	key2 := makeKey(fmt.Sprintf("name-%v", 2))
	tree.Insert(key2, []byte(fmt.Sprintf("value-%v", 2)))
	tree.Commit()

	//r1 = tree.Get(key)
	//assert.Equal([]byte("value-2"), r1)
	//tree.Close()

	//tree = UrkelTree(testDir, h256)
	key3 := makeKey(fmt.Sprintf("name-%v", 3))
	tree.Insert(key3, []byte(fmt.Sprintf("value-%v", 3)))
	tree.Commit()

	r1 := tree.Get(key1)
	assert.Equal([]byte("value-1"), r1)
	tree.Close()
}
