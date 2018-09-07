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
}

func New(h Hasher, root node) *Trie {
	return &Trie{
		Root:    root,
		hashFn:  h,
		keySize: h.GetSize() * 8,
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
	newRoot = &leafNode{key: key, value: value, params: storeValues{data: leaf}}
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
			// TODO: return from store resolve
			return nil
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
				return nil
			}
			// TODO: Return from store.retrieve and
			return nn.value
		default:
			return nil
		}
	}
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
