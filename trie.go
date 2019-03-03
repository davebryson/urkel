package urkel

import (
	"bytes"
	"fmt"
)

func HasBit(key []byte, depth uint) bool {
	oct := depth >> 3
	bit := depth & 7
	result := (key[oct] >> (7 - bit)) & 1
	return result != 0
}

type Trie struct {
	Root    node
	hashFn  Hasher
	keySize uint
	store   *Store
}

func New(h Hasher, root node) *Trie {
	// recover state
	// create store data
	// set root from state
	// openDb
	store := OpenDb("data")

	return &Trie{
		Root:    root,
		hashFn:  h,
		keySize: h.GetSize() * 8,
		store:   store,
	}
}

func (t *Trie) RootHash() []byte {
	return t.Root.hash(t.hashFn)
}

func (t *Trie) Insert(key, value []byte) {
	t.Root = t.insert(t.Root, key, value)
}

func (t *Trie) insert(root node, key, value []byte) node {
	leaf := leafHashValue(t.hashFn, key, value)
	nodes := make([]node, 0)
	var depth uint

loop:
	for {
		switch nn := root.(type) {
		case *nullNode:
			break loop
		case *hashNode:
			// TODO: reolve from store.  Hash is only the compact rep of Leaf/Internal
			break
		case *leafNode:
			if bytes.Compare(key, nn.key) == 0 {
				if bytes.Compare(leaf, nn.params.data) == 0 {
					return nn
				}
				break loop
			}

			for HasBit(key, depth) == HasBit(nn.key, depth) {
				// Child-less sibling.
				nodes = append(nodes, &nullNode{})
				depth++
			}

			nodes = append(nodes, nn)
			depth++
			break loop
		case *internalNode:
			if depth == t.keySize {
				v := fmt.Sprintf("Missing node @ depth: %v", depth)
				panic(v)
			}

			if HasBit(key, depth) {
				nodes = append(nodes, nn.left)
				root = nn.right
			} else {
				nodes = append(nodes, nn.right)
				root = nn.left
			}

			depth++
			break
		default:
			break loop
		}
	}

	var newRoot node
	newRoot = NewLeafNode(key, value, leaf)
	total := len(nodes) - 1

	// Build the tree: bottom -> top
	for i := total; i >= 0; i-- {
		n := nodes[i]
		depth--

		if HasBit(key, depth) {
			// <- node root ->
			newRoot = &internalNode{left: n, right: newRoot}
		} else {
			// <- root node ->
			newRoot = &internalNode{left: newRoot, right: n}
		}
	}

	return newRoot
}

func (t *Trie) Get(key []byte) []byte {
	return t.get(t.Root, key)
}

func (t *Trie) get(root node, key []byte) []byte {
	var depth uint

	for {
		switch nn := root.(type) {
		case *nullNode:
			return nil
		case *hashNode:
			fmt.Println("resolve hashnode")
			fmt.Printf("Node pos %v\n", nn.params.getPos())

			isLeaf := false
			if nn.params.getLeaf() == 1 {
				isLeaf = true
			}
			n, err := t.store.Resolve(nn.params.index, nn.params.getPos(), isLeaf)
			if err != nil {
				fmt.Printf("Trie Resolve %v", err)
				return nil
			}
			root = n
		case *internalNode:
			if depth == t.keySize {
				panic("Missing a node!")
			}
			if HasBit(key, depth) {
				root = nn.right
			} else {
				root = nn.left
			}
			depth++
			break
		case *leafNode:
			if bytes.Compare(key, nn.key) != 0 {
				// Prefix collision
				return nil
			}

			if nn.value != nil {
				return nn.value
			}

			fmt.Printf("retreive @ %v\n", nn.vPos)
			return t.store.Retrieve(nn.vIndex, nn.vSize, nn.vPos)
		default:
			return nil
		}
	}
}

func (t *Trie) Commit() {
	// Write nodes out to storage
	// Commit the result to meta

	result := t.writeNode(t.Root)
	if result == nil {
		panic("Got nil on commit")
	}

	t.store.Commit(result)

	t.Root = result
}

func (t *Trie) writeNode(root node) node {
	switch nn := root.(type) {
	case *nullNode:
		return nn
	case *internalNode:
		// Walk down the tree
		nn.left = t.writeNode(nn.left)
		nn.right = t.writeNode(nn.right)

		if nn.params.index == 0 {
			// 0 means we haven't saved it yet, so do that...
			encoded := nn.Encode(t.hashFn)
			i, pos, err := t.store.WriteNode(encoded)
			if err != nil {
				panic(err)
			}
			nn.params.setPos(pos)
			nn.params.index = i
		}

		return nn.toHashNode(t.hashFn)
	case *leafNode:
		if nn.params.index == 0 {
			// Write the value
			i, vpos, err := t.store.WriteValue(nn.value)
			if err != nil {
				panic(err)
			}
			nn.vPos = vpos
			nn.vSize = uint16(len(nn.value))
			nn.vIndex = i

			// Now write the leaf
			encoded := nn.Encode()
			i, pos, err := t.store.WriteNode(encoded)
			if err != nil {
				panic(err)
			}
			nn.params.index = i
			nn.params.setPos(pos)
		}
		return nn.toHashNode(t.hashFn)
	case *hashNode:
		return nn
	}
	return nil
}

// TEMP
func (t *Trie) Close() {
	t.store.Close()
}

func (t *Trie) Prove(key []byte) *Proof {
	proof := NewProof()
	var depth uint
	root := t.Root
loop:
	for {
		switch nn := root.(type) {
		case *nullNode:
			break loop
		case *internalNode:
			if depth == t.keySize {
				panic("Missing a node!")
			}
			if HasBit(key, depth) {
				h := nn.left.hash(t.hashFn)
				proof.Push(h)
				root = nn.right
			} else {
				h := nn.right.hash(t.hashFn)
				proof.Push(h)
				root = nn.left
			}
			depth++
			break
		case *leafNode:
			// TODO: retrieve value from store

			// Found a leaf down the alleged path
			// it's either a match or a collision - doesn't match
			// what we're expecting.
			if bytes.Compare(key, nn.key) == 0 {
				proof.Type = EXISTS
				proof.Value = nn.value
			} else {
				proof.Type = COLLISION
				proof.Key = nn.key
				proof.Hash = t.hashFn.Hash(nn.value)
			}
			break loop
		case *hashNode:
			// TODO: return  resolve from store
			break
		default:
			break loop
		}
	}
	return proof
}
