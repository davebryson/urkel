package urkel

import (
	"bytes"
	"encoding/binary"
)

var (
	internalPrefix = []byte{0x01}
	leafPrefix     = []byte{0x00}
	leafSize       = 2 + 4 + 2 + 32
	internalSize   = (2 + 4 + 32) * 2
)

type storeValues struct {
	index uint16
	flags uint32
	data  []byte
}

// These flags settings are the biggest challenge with
// the node interface.  If I add these calls to the interace
func (n *storeValues) getPos() uint32    { return n.flags >> 1 }
func (n *storeValues) setPos(pos uint32) { n.flags = pos*2 + n.getLeaf() }
func (n *storeValues) getLeaf() uint32   { return n.flags & 1 }
func (n *storeValues) setLeaf(bit uint32) {
	n.flags = (n.flags &^ 1) >> 0
	n.flags += bit
}

type node interface {
	hash(Hasher) []byte
	toHashNode(Hasher) *hashNode
	getParams() storeValues
}

type (
	nullNode struct {
		params storeValues
	}
	hashNode struct {
		params storeValues
	}
	leafNode struct {
		params storeValues
		// Value specific stuff
		key    []byte
		value  []byte
		vIndex uint16
		vPos   uint32
		vSize  uint16
	}
	internalNode struct {
		params storeValues
		left   node
		right  node
	}
)

func NewHashNode(index uint16, flags uint32, data []byte) *hashNode {
	return &hashNode{
		params: storeValues{
			index: index,
			flags: flags,
			data:  data,
		},
	}
}

func NewLeafNode(key, value, leafHash []byte) *leafNode {
	s := storeValues{data: leafHash}
	s.setLeaf(1)
	return &leafNode{key: key, value: value, params: s}
}

func (n *nullNode) getParams() storeValues     { return n.params }
func (n *hashNode) getParams() storeValues     { return n.params }
func (n *leafNode) getParams() storeValues     { return n.params }
func (n *internalNode) getParams() storeValues { return n.params }

// Impl hash for node
func (n *nullNode) hash(h Hasher) []byte { return h.ZeroHash() }
func (n *hashNode) hash(h Hasher) []byte { return n.params.data }
func (n *leafNode) hash(h Hasher) []byte { return n.params.data }
func (n *internalNode) hash(h Hasher) []byte {
	if n.params.data == nil {
		lh := n.left.hash(h)
		rh := n.right.hash(h)
		n.params.data = h.Hash(internalPrefix, lh, rh)
	}
	return n.params.data
}

// impl ToHashNode() for node
func (n *hashNode) toHashNode(h Hasher) *hashNode { return n }
func (n *nullNode) toHashNode(h Hasher) *hashNode {
	return NewHashNode(0, 0, n.hash(h))
}
func (n *leafNode) toHashNode(h Hasher) *hashNode {
	return NewHashNode(n.params.index, n.params.flags, n.params.data)
}
func (n *internalNode) toHashNode(h Hasher) *hashNode {
	hashed := n.hash(h)
	return NewHashNode(n.params.index, n.params.flags, hashed)
}

// Codec for nodes only Leaf and Internal are effected

func (n *leafNode) Encode() []byte {
	size := 2 + 4 + 2 + 32 // From node.GetSize - 32 assumes a 32 byte key from the hasher
	b := make([]byte, size)
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
func (n *internalNode) Encode(h Hasher) []byte {
	size := (2 + 4 + 32) * 2 // From node.GetSize - 32 assumes a 32 byte key from the hasher
	b := make([]byte, size)

	// Encode left
	leftParams := n.left.getParams()
	offset := 0
	binary.LittleEndian.PutUint16(b[offset:], uint16(leftParams.index*2))
	offset += 2
	binary.LittleEndian.PutUint32(b[offset:], uint32(leftParams.flags))
	offset += 4
	copy(b[offset:], n.left.hash(h))
	offset += 32

	// Right
	rightParams := n.right.getParams()
	binary.LittleEndian.PutUint16(b[offset:], uint16(rightParams.index))
	offset += 2
	binary.LittleEndian.PutUint32(b[offset:], uint32(rightParams.flags))
	offset += 4
	copy(b[offset:], n.right.hash(h))

	return b
}

// Decode a leaf or internal node
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

		key = make([]byte, 32)
		_, err = buf.Read(key[0:32])
		if err != nil {
			return nil, err
		}

		index >>= 1
		params := storeValues{}
		params.setLeaf(1)
		return &leafNode{
			key:    key,
			vIndex: index,
			vPos:   pos,
			vSize:  size,
			params: params}, nil
	}

	// Decode Internal
	var lindex uint16
	var lflags uint32
	var lkey []byte
	var rindex uint16
	var rflags uint32
	var rkey []byte

	// how to handle errors on Read
	err := binary.Read(buf, binary.LittleEndian, &lindex)
	if err != nil {
		return nil, err
	}
	err = binary.Read(buf, binary.LittleEndian, &lflags)
	if err != nil {
		return nil, err
	}
	// TODO: This size should be dependent on the hasher
	lkey = make([]byte, 32)
	_, err = buf.Read(lkey[0:32])
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

	rkey = make([]byte, 32)
	_, err = buf.Read(rkey[0:32])
	if err != nil {
		return nil, err
	}

	//Left/Right nodes should be hash node
	lhashnode := &hashNode{
		params: storeValues{index: lindex, flags: lflags, data: lkey},
	}
	rhashnode := &hashNode{
		params: storeValues{index: rindex, flags: rflags, data: rkey},
	}
	return &internalNode{
		left:  lhashnode,
		right: rhashnode,
	}, nil
}

func leafHashValue(hasher Hasher, k, v []byte) []byte {
	valueHash := hasher.Hash(v)
	return hasher.Hash(leafPrefix, k, valueHash)
}
