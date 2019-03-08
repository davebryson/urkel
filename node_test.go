package urkel

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNodeParsing(t *testing.T) {
	assert := assert.New(t)
	h256 := &Sha256{}

	l := newLeafNode(makeKey("dave"), []byte("hello"), makeKey("blahblah"))
	l.index = 1
	l.pos = 10

	r := newLeafNode(makeKey("dave"), []byte("hello"), makeKey("blahblah"))
	r.index = 1
	r.pos = 40

	in := &internalNode{
		left:  l,
		right: r,
	}

	e := in.Encode(h256)
	n, err := DecodeNode(e, false)
	assert.Nil(err)
	assert.False(n.isLeaf())
	internal := n.(*internalNode)
	assert.Equal(uint16(1), internal.left.getIndex())
	assert.Equal(uint32(10), internal.left.getPos())
	assert.True(internal.left.isLeaf())

	assert.Equal(uint32(40), internal.right.getPos())
	assert.Equal(uint16(1), internal.right.getIndex())
	assert.True(internal.right.isLeaf())
}
