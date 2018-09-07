package urkel

var (
	internalPrefix = []byte{0x01}
	leafPrefix     = []byte{0x00}
)

type node interface {
	hash(Hasher) []byte
}

type storeValues struct {
	index int
	flags int
	data  []byte
}

func (node *storeValues) getIndex() int  { return node.index }
func (node *storeValues) setIndex(v int) { node.index = v }
func (node *storeValues) getPos() int    { return node.flags >> 1 }
func (node *storeValues) setPos(pos int) { node.flags = pos*2 + node.getLeaf() }
func (node *storeValues) getLeaf() int   { return node.flags & 1 }
func (node *storeValues) setLeaf(bit int) {
	node.flags = (node.flags &^ 1) >> 0
	node.flags += bit
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
		key    []byte
		value  []byte
		vIndex int
		vPos   int
		vSize  int
	}
	internalNode struct {
		params storeValues
		left   node
		right  node
	}
)

// impl node
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

func (n *leafNode) copy() *leafNode         { copy := *n; return &copy }
func (n *internalNode) copy() *internalNode { copy := *n; return &copy }

func intoHashNode(n node, h Hasher) *hashNode {
	switch nt := n.(type) {
	case *nullNode:
		return &hashNode{params: storeValues{index: 0, flags: 0, data: nt.hash(h)}}
	case *hashNode:
		return nt
	case *leafNode:
		return &hashNode{params: storeValues{
			index: nt.params.index,
			flags: nt.params.flags,
			data:  nt.params.data}}
	case *internalNode:
		return &hashNode{params: storeValues{
			index: nt.params.index,
			flags: nt.params.flags,
			data:  nt.params.data}}
	default:
		return &hashNode{params: storeValues{index: 0, flags: 0, data: nt.hash(h)}}
	}
}

func leafHashValue(hasher Hasher, k, v []byte) []byte {
	valueHash := hasher.Hash(v)
	return hasher.Hash(leafPrefix, k, valueHash)
}
