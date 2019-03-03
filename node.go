package urkel

import (
	"bytes"
	"encoding/binary"
)

// node is the generic interface for all tree nodes.  There are 4 possible concrete nodes:
// nullNode, hashNode, leafNode, internalNode.  Because the internalNode contains embedded
// nodes that can be any of the above, the interface has the addition of methods to provide
// generic access to each.
type node interface {
	// return the hash of the node.data
	hash(Hasher) []byte

	// shapeshift to a hashnode
	toHashNode(Hasher) *hashNode

	// get/set the pos which which involves some calculations on node.flags
	getPos() uint32
	setPos(p uint32)

	// get/set the index which is the file number
	getIndex() uint16
	setIndex(i uint16)

	// the node position with some extra bit fiddling to help to distinquish the type of node
	getFlags() uint32

	// is the node a leafNode
	isLeaf() bool
}

// Check implementations
var _ node = (*nullNode)(nil)
var _ node = (*hashNode)(nil)
var _ node = (*leafNode)(nil)
var _ node = (*internalNode)(nil)

// Storage data used in each node.
// index: the file number
// flags: is the storage pos with some extra bit information (see above)
// The flag/pos work together to help determine the type of node when decoding
type storeValues struct {
	index uint16
	flags uint32
}

// if 'flags' ANDs to 1 - it's a leaf
func (n *storeValues) isLeaf() bool {
	if n.flags&1 == 1 {
		return true
	}
	return false
}

// Update the node based on the raw storage position (see tree.writeNode())
// we add a 1 to the pos of a leaf node so we can use flags to determine its type
func (n *storeValues) setPos(pos uint32) {
	if n.isLeaf() {
		n.flags = pos*2 + 1
	} else {
		n.flags = pos * 2
	}
}

// getters/setters that all nodes need access to
// getPos devided out the flags that are double above
func (n *storeValues) getPos() uint32    { return n.flags >> 1 }
func (n *storeValues) getIndex() uint16  { return n.index }
func (n *storeValues) setIndex(i uint16) { n.index = i }
func (n *storeValues) getFlags() uint32  { return n.flags }

// ********** nullNode ************

// Sentinal node
type nullNode struct {
	storeValues
	data []byte
}

func (n *nullNode) hash(h Hasher) []byte { return h.ZeroHash() }
func (n *nullNode) toHashNode(h Hasher) *hashNode {
	return newHashNode(0, 0, n.hash(h))
}

// ********** hashNode **********

// Used to represent nodes from after they've been stored
type hashNode struct {
	storeValues
	data []byte
}

func newHashNode(index uint16, flags uint32, data []byte) *hashNode {
	h := &hashNode{}
	h.index = index
	h.flags = flags
	h.data = data
	return h
}

func (n *hashNode) hash(h Hasher) []byte          { return n.data }
func (n *hashNode) toHashNode(h Hasher) *hashNode { return n }

// ********** leafNode **********

// Leaf of the tree. Contains the values
type leafNode struct {
	storeValues
	data []byte
	// Value specific stuff
	key    []byte
	value  []byte
	vIndex uint16
	vPos   uint32
	vSize  uint16
}

func newLeafNode(key, value, leafHash []byte) *leafNode {
	l := &leafNode{key: key, value: value}
	l.flags = 1
	l.data = leafHash
	return l
}

func (n *leafNode) hash(h Hasher) []byte { return n.data }
func (n *leafNode) toHashNode(h Hasher) *hashNode {
	return newHashNode(n.index, n.flags, n.data)
}

// ********** internalNode **********

// Branch.  Contains other nodes
type internalNode struct {
	storeValues
	data  []byte
	left  node
	right node
}

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
	return newHashNode(n.index, n.flags, hashed)
}

// ********** Codec **********

// We only store leaf/internal Nodes.  However an internal node
// may contain other nodes represented by their hash

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
	// Note: double the index
	binary.LittleEndian.PutUint16(b[offset:], n.left.getIndex()*2)
	offset += 2
	// Note: we use the raw flags (not pos)
	binary.LittleEndian.PutUint32(b[offset:], n.left.getFlags())
	offset += 4
	copy(b[offset:], n.left.hash(h))
	offset += KeySizeInBytes

	// Right
	// Note: we don't double this index
	binary.LittleEndian.PutUint16(b[offset:], n.right.getIndex())
	offset += 2
	binary.LittleEndian.PutUint32(b[offset:], n.right.getFlags())
	offset += 4
	copy(b[offset:], n.right.hash(h))

	return b
}

// DecodeNode - either a leaf or internal node
func DecodeNode(data []byte, isleaf bool) (node, error) {
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
		err = binary.Read(buf, binary.LittleEndian, &pos)
		if err != nil {
			return nil, err
		}
		err = binary.Read(buf, binary.LittleEndian, &size)
		if err != nil {
			return nil, err
		}

		key = make([]byte, KeySizeInBytes)
		_, err = buf.Read(key[0:KeySizeInBytes])
		if err != nil {
			return nil, err
		}

		// Note: divide out the index, since we double in encode
		index >>= 1
		leafN := &leafNode{key: key, vIndex: index, vPos: pos, vSize: size}
		// Set the flags to 1 here, so we tag it as a leaf node in 'flags'
		leafN.flags = 1
		return leafN, nil

	}

	// Decode Internal
	var lindex uint16
	var lflags uint32
	var lkey []byte
	var rindex uint16
	var rflags uint32
	var rkey []byte

	err := binary.Read(buf, binary.LittleEndian, &lindex)
	if err != nil {
		return nil, err
	}
	err = binary.Read(buf, binary.LittleEndian, &lflags)
	if err != nil {
		return nil, err
	}

	lkey = make([]byte, KeySizeInBytes)
	_, err = buf.Read(lkey[0:KeySizeInBytes])
	if err != nil {
		return nil, err
	}

	err = binary.Read(buf, binary.LittleEndian, &rindex)
	if err != nil {
		return nil, err
	}
	err = binary.Read(buf, binary.LittleEndian, &rflags)
	if err != nil {
		return nil, err
	}

	rkey = make([]byte, KeySizeInBytes)
	_, err = buf.Read(rkey[0:KeySizeInBytes])
	if err != nil {
		return nil, err
	}

	// Note: The decode internalnode contains hashnodes
	lhashnode := newHashNode(lindex, lflags, lkey)
	rhashnode := newHashNode(rindex, rflags, rkey)

	return &internalNode{
		left:  lhashnode,
		right: rhashnode,
	}, nil
}
