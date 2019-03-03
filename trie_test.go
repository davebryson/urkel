package urkel

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

var hasher = &Sha256{}

func makeKey(k string) []byte {
	return hasher.Hash([]byte(k))
}

var testFile = "data/0000000001"

func TestStore(t *testing.T) {
	defer func() {
		os.RemoveAll(testFile)
	}()

	assert := assert.New(t)
	h256 := &Sha256{}
	tree := New(h256, &nullNode{})

	for i := 0; i < 10000; i++ {
		k := fmt.Sprintf("name-%v", i)
		key := makeKey(k)
		v := fmt.Sprintf("value-%v", i)
		tree.Insert(key, []byte(v))
	}

	tree.Commit()

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

	root := "6c7db9e553563e02e94cf906049935a2ba364106c89c369257194df2e40b00e7"
	rootHash := tree.RootHash()
	troot := fmt.Sprintf("%x", rootHash)

	fmt.Printf("Tree root: %v\n", tree.Root)

	assert.Equal(root, troot)

	// Not test we can read the meta
	lastPos := tree.Root.getParams().flags >> 1
	tree.Close()

	st := OpenDb("data")
	assert.Equal(st.state.rootPos, lastPos)
	fmt.Printf("Meta pos: %v", st.state.metaPos)

	nr, err := st.GetRootNode()
	assert.Nil(err)
	assert.NotNil(nr)

	fmt.Printf("Node: %v\n", nr)

	assert.Equal(rootHash, nr.getParams().data)

	//fmt.Printf("Tree root: %x\n", tree.RootHash())

}

func TestShouldInsertAndGet(t *testing.T) {
	assert := assert.New(t)

	h256 := &Sha256{}
	tree := New(h256, &nullNode{})

	// Test insert
	for i := 0; i < 1000; i++ {
		k := fmt.Sprintf("name-%v", i)
		key := makeKey(k)
		v := fmt.Sprintf("value-%v", i)
		tree.Insert(key, []byte(v))
	}

	//tree.Commit()

	// Test they're in the tree
	for i := 0; i < 10; i++ {
		k := fmt.Sprintf("name-%v", i)
		key := makeKey(k)
		expectedValue := fmt.Sprintf("value-%v", i)
		r1 := tree.Get(key)
		assert.NotNil(r1)
		assert.Equal([]byte(expectedValue), r1)
	}

	rh := fmt.Sprintf("%x", tree.RootHash())
	assert.Equal("c30615331618881a39b04b51e5625243ec87a2b69fdefe319feedc0b0f96a7a0", rh)
}

func TestShouldDoProofs(t *testing.T) {
	assert := assert.New(t)

	h256 := &Sha256{}
	tree := New(h256, &nullNode{})
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
