package urkel

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// node is the generic interface for all tree nodes.  There are 4 possible concrete nodes:
// nullNode, hashNode, leafNode, internalNode.  Because the internalNode contains embedded
// nodes that can be any of the above, the interface has the additional methods to provide
// generic access to each throughout the tree
type node interface {
	// return the hash of the node.data
	hash(Hasher) []byte

	// shapeshift to a hashnode
	toHashNode(Hasher) *hashNode

	// is the node a leafNode
	isLeaf() bool

	// Helpers when we don't know what specific type of node we
	// are working with
	getIndex() uint16
	getPos() uint32

	setData(d []byte)
}

// Check implementations
var _ node = (*nullNode)(nil)
var _ node = (*hashNode)(nil)
var _ node = (*leafNode)(nil)
var _ node = (*internalNode)(nil)

// ********** nullNode ************

// Sentinal node
type nullNode struct {
	index uint16
	pos   uint32
	data  []byte
}

func (n *nullNode) hash(h Hasher) []byte { return h.ZeroHash() }
func (n *nullNode) isLeaf() bool         { return false }
func (n *nullNode) toHashNode(h Hasher) *hashNode {
	return newHashNode(0, 0, n.hash(h), 0)
}
func (n *nullNode) getIndex() uint16 { return n.index }
func (n *nullNode) getPos() uint32   { return n.pos }
func (n *nullNode) setData(d []byte) { n.data = d }

// ********** hashNode **********

// Used to represent nodes after they've been stored
type hashNode struct {
	index uint16
	pos   uint32
	data  []byte
	leaf  uint8
}

func newHashNode(index uint16, pos uint32, data []byte, leaf uint8) *hashNode {
	return &hashNode{
		index: index,
		pos:   pos,
		data:  data,
		leaf:  leaf,
	}
}

func (n *hashNode) hash(h Hasher) []byte          { return n.data }
func (n *hashNode) isLeaf() bool                  { return n.leaf == 1 }
func (n *hashNode) toHashNode(h Hasher) *hashNode { return n }
func (n *hashNode) getIndex() uint16              { return n.index }
func (n *hashNode) getPos() uint32                { return n.pos }
func (n *hashNode) setData(d []byte)              { n.data = d }

// ********** leafNode **********

// Leaf of the tree. Contains the values
type leafNode struct {
	index uint16
	pos   uint32
	data  []byte
	// Value specific stuff
	key    []byte
	value  []byte
	vIndex uint16
	vPos   uint32
	vSize  uint16
}

func newLeafNode(key, value, leafHash []byte) *leafNode {
	l := &leafNode{key: key, value: value}
	l.data = leafHash
	return l
}

func (n *leafNode) hash(h Hasher) []byte { return n.data }
func (n *leafNode) isLeaf() bool         { return true }
func (n *leafNode) toHashNode(h Hasher) *hashNode {
	return newHashNode(n.index, n.pos, n.data, 1)
}
func (n *leafNode) getIndex() uint16 { return n.index }
func (n *leafNode) getPos() uint32   { return n.pos }
func (n *leafNode) setData(d []byte) { n.data = d }

// ********** internalNode **********

// Branch.  Contains other nodes via left/right
type internalNode struct {
	index uint16
	pos   uint32
	data  []byte
	left  node
	right node
}

func (n *internalNode) isLeaf() bool { return false }

func (n *internalNode) hash(h Hasher) []byte {
	if n.data == nil {
		lh := n.left.hash(h)
		rh := n.right.hash(h)
		n.data = h.Hash(internalNodeHashPrefix, lh, rh)
	}
	return n.data
}

func (n *internalNode) toHashNode(h Hasher) *hashNode {
	hashed := n.hash(h)
	return newHashNode(n.index, n.pos, hashed, 0)
}

func (n *internalNode) getIndex() uint16 { return n.index }
func (n *internalNode) getPos() uint32   { return n.pos }
func (n *internalNode) setData(d []byte) { n.data = d }

// ********** Codec **********

// We only store leaf/internal nodes.  However an internal node
// may contain other nodes represented by their hash

// Used in the encoder to 'tag' position so we can determine
// if the decoded bits are a leaf or internal node
func TagPosition(pos uint32, isLeaf bool) uint32 {
	if isLeaf {
		return pos*2 + 1
	}
	return pos * 2
}

// Used in the decode to get the true position and whether the bits are a
// leaf or internal node
func GetTagForPosition(taggedPos uint32) (uint8, uint32) {
	isLeaf := uint8(taggedPos & 1)
	pos := taggedPos >> 1
	return isLeaf, pos
}

// Encode a Leaf
func (n *leafNode) Encode() []byte {
	b := make([]byte, leafSize)
	offset := 0
	binary.LittleEndian.PutUint16(b[offset:], uint16(n.vIndex*2+1))
	offset += 2
	binary.LittleEndian.PutUint32(b[offset:], uint32(n.vPos))
	offset += 4
	binary.LittleEndian.PutUint16(b[offset:], uint16(n.vSize))
	offset += 2

	// Copy key to the remainder of the buffer
	copy(b[offset:], n.key)

	return b
}

// Encode an Internal node
func (n *internalNode) Encode(h Hasher) []byte {
	b := make([]byte, internalSize)

	// Encode left
	offset := 0
	// Note: double the index.  We'll shift this out in the encoder to test for potential
	// file corruption.
	binary.LittleEndian.PutUint16(b[offset:], n.left.getIndex()*2)
	offset += 2
	// Note: tag the position
	lpos := TagPosition(n.left.getPos(), n.left.isLeaf())
	binary.LittleEndian.PutUint32(b[offset:], lpos)
	offset += 4
	copy(b[offset:], n.left.hash(h))
	offset += KeySizeInBytes

	// Right
	// Note: we don't double this index
	binary.LittleEndian.PutUint16(b[offset:], n.right.getIndex())
	offset += 2
	// Note: tag the position
	rpos := TagPosition(n.right.getPos(), n.right.isLeaf())
	binary.LittleEndian.PutUint32(b[offset:], rpos)
	offset += 4
	copy(b[offset:], n.right.hash(h))

	return b
}

// DecodeNode - either a leaf or internal node
func DecodeNode(data []byte, isleaf bool) (node, error) {

	fmt.Printf("Decode %v bits\n", len(data))

	buf := bytes.NewReader(data)
	if isleaf {
		var index uint16
		var pos uint32
		var size uint16
		var key []byte

		err := binary.Read(buf, binary.LittleEndian, &index)
		if err != nil {
			return nil, err
		}

		// Should == 1 as we added 1 in encode
		if index&1 != 1 {
			panic("Decoding leaf: Potentially corrupt database")
		}
		index >>= 1

		err = binary.Read(buf, binary.LittleEndian, &pos)
		if err != nil {
			return nil, err
		}
		err = binary.Read(buf, binary.LittleEndian, &size)
		if err != nil {
			return nil, err
		}

		key = make([]byte, KeySizeInBytes)
		_, err = buf.Read(key)
		if err != nil {
			return nil, err
		}

		leafN := &leafNode{key: key, vIndex: index, vPos: pos, vSize: size}
		return leafN, nil
	}

	// Decode Internal
	var lindex uint16
	var lpos uint32
	var lkey []byte
	var rindex uint16
	var rpos uint32
	var rkey []byte

	// Left node
	err := binary.Read(buf, binary.LittleEndian, &lindex)
	if err != nil {
		return nil, err
	}
	err = binary.Read(buf, binary.LittleEndian, &lpos)
	if err != nil {
		return nil, err
	}

	// Should == 0 as it's a doubled number
	if lindex&1 != 0 {
		panic("Decoding internal: Potentially corrupt database")
	}
	lindex >>= 1

	// Get the real position back and whether it's a leaf or not
	leftIsLeaf, leftPos := GetTagForPosition(lpos)

	lkey = make([]byte, KeySizeInBytes)
	_, err = buf.Read(lkey)
	if err != nil {
		return nil, err
	}

	// Right node
	err = binary.Read(buf, binary.LittleEndian, &rindex)
	if err != nil {
		return nil, err
	}
	err = binary.Read(buf, binary.LittleEndian, &rpos)
	if err != nil {
		return nil, err
	}

	rightIsLeaf, rightPos := GetTagForPosition(rpos)

	rkey = make([]byte, KeySizeInBytes)
	_, err = buf.Read(rkey)
	if err != nil {
		return nil, err
	}

	result := &internalNode{
		left:  newHashNode(lindex, leftPos, lkey, leftIsLeaf),
		right: newHashNode(rindex, rightPos, rkey, rightIsLeaf),
	}
	return result, nil
}
