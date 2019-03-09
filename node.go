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

	setData([]byte)
}

// Check implementations
var _ node = (*nullNode)(nil)
var _ node = (*hashNode)(nil)
var _ node = (*leafNode)(nil)
var _ node = (*internalNode)(nil)

// ********** nullNode ************

// Sentinal node
type nullNode struct {
	data  []byte
	saved uint8
}

func (n *nullNode) hash(h Hasher) []byte { return h.ZeroHash() }
func (n *nullNode) isLeaf() bool         { return false }
func (n *nullNode) toHashNode(h Hasher) *hashNode {
	return newHashNode(n.hash(h), 0, n.saved)
}
func (n *nullNode) setData(d []byte) { n.data = d }

// ********** hashNode **********

// Used to represent nodes after they've been stored
type hashNode struct {
	data  []byte
	leaf  uint8
	saved uint8
}

func newHashNode(data []byte, leaf uint8, saved uint8) *hashNode {
	return &hashNode{
		data:  data,
		leaf:  leaf,
		saved: saved,
	}
}

func (n *hashNode) hash(h Hasher) []byte          { return n.data }
func (n *hashNode) isLeaf() bool                  { return n.leaf == 1 }
func (n *hashNode) toHashNode(h Hasher) *hashNode { return n }
func (n *hashNode) setData(d []byte)              { n.data = d }

// ********** leafNode **********

// Leaf of the tree. Contains the values
type leafNode struct {
	data  []byte
	saved uint8
	// Value specific stuff
	key   []byte
	value []byte
	vSize uint16
}

func newLeafNode(key, value, leafHash []byte) *leafNode {
	l := &leafNode{key: key, value: value}
	l.data = leafHash
	return l
}

func (n *leafNode) hash(h Hasher) []byte { return n.data }
func (n *leafNode) isLeaf() bool         { return true }
func (n *leafNode) toHashNode(h Hasher) *hashNode {
	return newHashNode(n.data, 1, n.saved)
}
func (n *leafNode) setData(d []byte) { n.data = d }

// ********** internalNode **********

// Branch.  Contains other nodes via left/right
type internalNode struct {
	data  []byte
	saved uint8
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
	return newHashNode(hashed, 0, n.saved)
}
func (n *internalNode) setData(d []byte) { n.data = d }

// ********** Codec **********

// We only store leaf/internal nodes.  However an internal node
// may contain other nodes represented by their hash

// Used in the encoder to 'tag' position so we can determine
// if the decoded bits are a leaf or internal node
func tagNodeType(isLeaf bool) uint8 {
	if isLeaf {
		return 1
	}
	return 0
}

/*func maybeLeafNodeTag(tag uint8) bool {
	if tag == 1 {
		return true
	}
	return false
}*/

// Encode a LeafNode  SIZE: 34
func (n *leafNode) Encode() []byte {
	b := make([]byte, leafSize)
	offset := 0
	binary.LittleEndian.PutUint16(b[offset:], uint16(n.vSize))
	offset += 2
	// Copy key to the remainder of the buffer
	copy(b[offset:], n.key)
	return b
}

// Encode an Internal node SIZE: 66
func (n *internalNode) Encode(h Hasher) []byte {
	b := make([]byte, internalSize)
	// Here we need to encode the l/r to tag if they're a leaf
	// 33
	offset := 0
	b[offset] = tagNodeType(n.left.isLeaf())
	offset++
	copy(b[offset:], n.left.hash(h))
	offset += keySizeInBytes

	// Right: 33
	b[offset] = tagNodeType(n.right.isLeaf())
	offset++
	copy(b[offset:], n.right.hash(h))
	return b
}

// DecodeNode - either a leaf or internal node
func DecodeNode(data []byte) (node, error) {
	numBits := len(data)
	isLeaf := numBits == leafSize
	isInternal := numBits == internalSize

	if !isLeaf && !isInternal {
		return nil, fmt.Errorf("Decode node: wrong size bits %v", numBits)
	}
	buf := bytes.NewReader(data)

	if isLeaf {
		var size uint16
		var key []byte
		if err := binary.Read(buf, binary.LittleEndian, &size); err != nil {
			return nil, err
		}

		key = make([]byte, keySizeInBytes)
		if _, err := buf.Read(key); err != nil {
			return nil, err
		}
		leafN := &leafNode{key: key, vSize: size}
		return leafN, nil
	}

	// Decode Internal
	var leftFlag uint8
	var lkey []byte
	var rightFlag uint8
	var rkey []byte

	// Left node
	// Read the flag
	if err := binary.Read(buf, binary.LittleEndian, &leftFlag); err != nil {
		return nil, err
	}
	lkey = make([]byte, keySizeInBytes)
	if _, err := buf.Read(lkey); err != nil {
		return nil, err
	}

	// Right node
	// Read the flag
	if err := binary.Read(buf, binary.LittleEndian, &rightFlag); err != nil {
		return nil, err
	}
	rkey = make([]byte, keySizeInBytes)
	if _, err := buf.Read(rkey); err != nil {
		return nil, err
	}

	result := &internalNode{
		left:  newHashNode(lkey, leftFlag, 1),
		right: newHashNode(rkey, rightFlag, 1),
	}
	return result, nil
}
