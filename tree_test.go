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

	fillTree(tree, 10000)
	// Commit to store
	tree.Commit()

	// Check we can read from store
	key := makeKey("name-56")
	r1 := tree.Get(key)
	assert.Equal([]byte("value-56"), r1)

	key = makeKey("name-399")
	r1 = tree.Get(key)
	assert.Equal([]byte("value-399"), r1)

	key = makeKey("name-919")
	r1 = tree.Get(key)
	assert.Equal([]byte("value-919"), r1)

	key = makeKey("NOPE-399")
	r1 = tree.Get(key)
	assert.Nil(r1)

	// Check for a consistent root hash
	root := "6c7db9e553563e02e94cf906049935a2ba364106c89c369257194df2e40b00e7"
	rootHash := tree.RootHash()
	troot := fmt.Sprintf("%x", rootHash)
	assert.Equal(root, troot)

	// Now test we can read the meta from store and it matches the
	// last tree
	lastPos := tree.Root.getPos()
	tree.Close()

	// Reopen the store
	st := &FileStore{}
	st.Open(testDir, h256)
	assert.Equal(st.state.rootPos, lastPos)

	nr, err := st.GetRootNode()
	assert.Nil(err)
	assert.NotNil(nr)

	// Compare the root hash from the meta store to the original tree's rootHash
	haNode := nr.(*hashNode)
	assert.Equal(rootHash, haNode.data)
	st.Close()
}

func TestStoreRemove(t *testing.T) {
	defer removeTestFile()
	assert := assert.New(t)

	tree := UrkelTree(testDir, h256)
	fillTree(tree, 10)
	tree.Commit()

	key := makeKey("name-3")
	r1 := tree.Get(key)
	assert.Equal([]byte("value-3"), r1)

	tree.Remove(key)
	tree.Commit()

	r1 = tree.Get(key)
	assert.Nil(r1)
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
