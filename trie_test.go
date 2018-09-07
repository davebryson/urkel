package urkel

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

var hasher = &Sha256{}

func makeKey(k string) []byte {
	return hasher.Hash([]byte(k))
}

func TestShouldInsertAndGet(t *testing.T) {
	assert := assert.New(t)

	h256 := &Sha256{}
	tree := New(h256, &nullNode{})

	// Test insert
	for i := 0; i < 10; i++ {
		k := fmt.Sprintf("name-%v", i)
		key := makeKey(k)
		v := fmt.Sprintf("value-%v", i)
		tree.Insert(key, []byte(v))
	}

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
	assert.Equal("59542bf6c400689c6162ef64d452f87faf4fe2177b7f557b84db3b31fdca2a8d", rh)
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
