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

func TestStore(t *testing.T) {
	defer func() {
		os.RemoveAll(testFile)
	}()

	assert := assert.New(t)

	// Create a new Tree and insert many K/Vs
	tree := UrkelTree(testDir, h256)

	for i := 0; i < 10000; i++ {
		k := fmt.Sprintf("name-%v", i)
		key := makeKey(k)
		v := fmt.Sprintf("value-%v", i)
		tree.Insert(key, []byte(v))
	}

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

	//fmt.Printf("Node: %v\n", nr)

	// Compare the root hash from the meta store to the original tree's rootHash
	haNode := nr.(*hashNode)
	assert.Equal(rootHash, haNode.data)
	st.Close()
}

func TestShouldDoProofs(t *testing.T) {
	assert := assert.New(t)

	// TODO: Move this once the Proof can read from storage

	tree := UrkelTree(testDir, h256)
	tree.Insert(makeKey("name-1"), []byte("value-1"))
	tree.Insert(makeKey("name-2"), []byte("value-2"))

	// Prove the key is there...
	prf1 := tree.Prove(makeKey("name-2"))
	assert.Equal(EXISTS, prf1.Type)
	assert.Equal([]byte("value-2"), prf1.Value)
	assert.Nil(prf1.Hash)
	assert.True(prf1.Depth() > 0)

	// Verify against the root
	r2 := prf1.Verify(tree.RootHash(), makeKey("name-2"), h256, 256)
	assert.Equal(OK, r2.Code)
	assert.Equal([]byte("value-2"), r2.Value)
}
