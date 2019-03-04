package urkel

import (
	"bytes"
	"fmt"
)

// HasBit is used throughout the Tree and Proof to decide whether
// to go left or right based on the key when walking the tree
func HasBit(key []byte, depth uint) bool {
	oct := depth >> 3
	bit := depth & 7
	result := (key[oct] >> (7 - bit)) & 1
	return result != 0
}

type Tree struct {
	Root   node
	hashFn Hasher
	store  *FileStore
}

func UrkelTree(dir string, h Hasher) *Tree {
	store := &FileStore{}
	err := store.Open(dir, h)
	if err != nil {
		panic(err)
	}

	rootNode, err := store.GetRootNode()
	if err != nil {
		// Log and use the nullNode
		return &Tree{
			Root:   &nullNode{},
			hashFn: h,
			store:  store,
		}

	}

	return &Tree{
		Root:   rootNode,
		hashFn: h,
		store:  store,
	}
}

func (t *Tree) RootHash() []byte {
	return t.Root.hash(t.hashFn)
}

func (t *Tree) Insert(key, value []byte) {
	t.Root = t.insert(t.Root, key, value)
}

func (t *Tree) insert(root node, key, value []byte) node {
	leaf := leafHashValue(t.hashFn, key, value)
	nodes := make([]node, 0)
	var depth uint

loop:
	for {
		switch nn := root.(type) {
		case *nullNode:
			break loop
		case *hashNode:
			n, err := t.store.GetNode(nn.getIndex(), nn.getPos(), nn.isLeaf())
			if err != nil {
				return nil
			}
			root = n
		case *leafNode:
			if bytes.Compare(key, nn.key) == 0 {
				if bytes.Compare(leaf, nn.data) == 0 {
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
			if depth == KeySizeInBits {
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
	newRoot = newLeafNode(key, value, leaf)
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

func (t *Tree) Get(key []byte) []byte {
	return t.get(t.Root, key)
}

func (t *Tree) get(root node, key []byte) []byte {
	var depth uint

	for {
		switch nn := root.(type) {
		case *nullNode:
			return nil
		case *hashNode:
			n, err := t.store.GetNode(nn.getIndex(), nn.getPos(), nn.isLeaf())
			if err != nil {
				return nil
			}
			root = n
		case *internalNode:
			if depth == KeySizeInBits {
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
			return t.store.GetValue(nn.vIndex, nn.vSize, nn.vPos)
		default:
			return nil
		}
	}
}

// Commit nodes to storage
func (t *Tree) Commit() {
	result := t.writeNode(t.Root)
	if result == nil {
		panic("Got nil root on commit")
	}

	err := t.store.Commit(result)
	if err != nil {
		panic(err)
	}

	t.Root = result
}

func (t *Tree) writeNode(root node) node {
	switch nn := root.(type) {
	case *nullNode:
		return nn
	case *internalNode:
		// Walk down the tree
		nn.left = t.writeNode(nn.left)
		nn.right = t.writeNode(nn.right)

		if nn.getIndex() == 0 {
			// 0 means we haven't saved it yet, so do that...
			encoded := nn.Encode(t.hashFn)
			i, pos, err := t.store.WriteNode(encoded)
			if err != nil {
				panic(err)
			}
			nn.setPos(pos)
			nn.setIndex(i)
		}

		return nn.toHashNode(t.hashFn)
	case *leafNode:
		if nn.getIndex() == 0 {
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
			nn.setIndex(i)
			nn.setPos(pos)
		}
		return nn.toHashNode(t.hashFn)
	case *hashNode:
		return nn
	}
	return nil
}

// TEMP
func (t *Tree) Close() {
	t.store.Close()
}

func (t *Tree) Prove(key []byte) *Proof {
	proof := NewProof()
	var depth uint
	root := t.Root
loop:
	for {
		switch nn := root.(type) {
		case *nullNode:
			break loop
		case *internalNode:
			if depth == KeySizeInBits {
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
