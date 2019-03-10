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

// API

// ImmutableTree provides write access to the tree via tree.Transaction()
type ImmutableTree struct {
	Tree *Tree
	Root node
}

// MutableTree provides read-only access to the tree via tree.Snapshot()
type MutableTree struct {
	ImmutableTree
}

// Set a key/value in the tree
func (tx *MutableTree) Set(key, value []byte) {
	k := tx.Tree.hashFn.Hash(key)
	tx.Root = tx.Tree.insert(tx.Root, k, value)
}

// Remove a value via a key
func (tx *MutableTree) Remove(key []byte) {
	k := tx.Tree.hashFn.Hash(key)
	tx.Root, _ = tx.Tree.remove(tx.Root, k)
}

// Commit the tree and return the new root hash
func (tx *MutableTree) Commit() []byte {
	return tx.Tree.commit(tx.Root)
}

// Get a value for a given key
func (snap *ImmutableTree) Get(key []byte) []byte {
	k := snap.Tree.hashFn.Hash(key)
	return snap.Tree.get(snap.Root, k)
}

// RootHash return the current hash of the root
func (snap *ImmutableTree) RootHash() []byte {
	return snap.Tree.rootHash()
}

// Proof returns a proof of the key in the tree
func (snap *ImmutableTree) Proof(key []byte) *Proof {
	k := snap.Tree.hashFn.Hash(key)
	return snap.Tree.prove(k)
}

// **** Tree ****

// Tree is a base-2 merkle tree with consistent sized nodes and
// small proofs. This is based on the Handshake Urkel tree
type Tree struct {
	Root   node
	hashFn Hasher
	store  Db
}

// NewUrkelTree creates a new Tree
// Where Db: is the given store (see the Db interface). The Db should already be open.
func NewUrkelTree(store Db, h Hasher) *Tree {
	rootNode, err := store.GetRoot()
	if err != nil {
		nn := &nullNode{}
		nn.toHashNode(h)
		return &Tree{
			Root:   nn,
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

// Close the tree and store when your're done with it
func (t *Tree) Close() {
	t.store.Close()
}

// Transaction returns a MutableTree
func (t *Tree) Transaction() *MutableTree {
	tx := &MutableTree{}
	lastRoot, err := t.store.GetRoot()
	if err != nil {
		tx.Root = t.Root
		tx.Tree = t
		return tx
	}
	tx.Root = lastRoot
	tx.Tree = t
	return tx
}

// Snapshot returns an ImmutableTree
func (t *Tree) Snapshot() *ImmutableTree {
	lastRoot, err := t.store.GetRoot()
	if err != nil {
		fmt.Printf("Snap err: %v\n", err)
		panic("Error decoding last root")
	}
	s := &ImmutableTree{}
	s.Root = lastRoot
	s.Tree = t
	return s
}

func (t *Tree) rootHash() []byte {
	return t.Root.hash(t.hashFn)
}

// DONE: Store
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
			n, err := t.store.Get(nn.data)
			if err != nil {
				fmt.Printf("hashNode: error reading hashNode: %v\n", err)
				return nil
			}
			n.setData(nn.data)

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
			if depth == keySizeInBits {
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

// DONE: store
func (t *Tree) get(root node, key []byte) []byte {
	var depth uint
	for {
		switch nn := root.(type) {
		case *nullNode:
			return nil
		case *hashNode:
			n, err := t.store.Get(nn.data)
			if err != nil {
				fmt.Printf("hashNode: error reading hashNode: %v\n", err)
				return nil
			}
			n.setData(nn.data)
			root = n
		case *internalNode:
			if depth == keySizeInBits {
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
			if nn.value != nil && len(nn.value) > 0 {
				return nn.value
			}
			return t.store.ReadValue(nn.key)
		default:
			return nil
		}
	}
}

// Remove

func (t *Tree) remove(root node, key []byte) (node, error) {
	nodes := make([]node, 0)
	var depth uint
loop:
	for {
		switch nn := root.(type) {
		case *nullNode:
			return root, nil
		case *internalNode:
			if depth == keySizeInBits {
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
		case *hashNode:
			n, err := t.store.Get(nn.data)
			if err != nil {
				fmt.Printf("hashNode: error reading hashNode: %v\n", err)
				return nil, err
			}
			n.setData(nn.data)
			root = n
			break
		case *leafNode:
			if bytes.Compare(key, nn.key) != 0 {
				return root, nil
			}
			if depth == 0 {
				return &nullNode{}, nil
			}

			root = nodes[depth-1]
			if root.isLeaf() {
				// Pop<clap>off<clap> the last node
				nodes = nodes[:depth-1]
				depth--

				for depth > 0 {
					sideNode := nodes[depth-1]
					if _, isNullNode := sideNode.(*nullNode); !isNullNode {
						break
					}
				}
				nodes = nodes[:depth-1]
				depth--
			} else {
				root = &nullNode{}
			}
			break loop
		default:
			return nil, fmt.Errorf("remove: Unknown node type")
		}
	} // end for

	total := len(nodes) - 1
	for i := total; i >= 0; i-- {
		n := nodes[i]
		depth--

		if HasBit(key, depth) {
			// <- node root ->
			root = &internalNode{left: n, right: root}
		} else {
			// <- root node ->
			root = &internalNode{left: root, right: n}
		}
	}
	return root, nil
}

// Commit nodes to storage
func (t *Tree) commit(r node) []byte {
	newRoot := t.writeNode(r)
	if newRoot == nil {
		panic("Got nil root on commit")
	}

	t.Root = t.store.Commit(newRoot, t.hashFn)
	return t.rootHash()
}

func (t *Tree) writeNode(root node) node {
	switch nn := root.(type) {
	case *nullNode:
		return nn
	case *internalNode:
		// Walk down the tree
		nn.left = t.writeNode(nn.left)
		nn.right = t.writeNode(nn.right)

		if nn.saved == 0 {
			// 0 means we haven't saved it yet, so do that...
			encoded := nn.Encode(t.hashFn)
			k := nn.hash(t.hashFn)
			t.store.Set(k, encoded)
			nn.saved = 1
		}

		return nn.toHashNode(t.hashFn)
	case *leafNode:
		if nn.saved == 0 && len(nn.value) > 0 {
			// Write the value
			t.store.Set(nn.key, nn.value)
			nn.vSize = uint16(len(nn.value))

			// Now write the leaf
			encoded := nn.Encode()
			k := nn.hash(t.hashFn)
			t.store.Set(k, encoded)
			nn.saved = 1
		}
		return nn.toHashNode(t.hashFn)
	case *hashNode:
		return nn
	}
	return nil
}

// Done: store
func (t *Tree) prove(key []byte) *Proof {
	proof := NewProof()
	var depth uint
	root := t.Root
loop:
	for {
		switch nn := root.(type) {
		case *nullNode:
			break loop
		case *internalNode:
			if depth == keySizeInBits {
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
			value := t.store.ReadValue(nn.key)

			// Found a leaf down the alleged path:
			// it's either a match or a collision (doesn't match) what we're expecting.
			if bytes.Compare(key, nn.key) == 0 {
				proof.Type = EXISTS
				proof.Value = value
			} else {
				proof.Type = COLLISION
				proof.Key = nn.key
				proof.Hash = t.hashFn.Hash(value)
			}

			break loop
		case *hashNode:
			n, err := t.store.Get(nn.data)
			if err != nil {
				fmt.Printf("hashNode: error reading hashNode: %v\n", err)
				return nil
			}
			n.setData(nn.data)
			root = n
			break
		default:
			break loop
		}
	}
	return proof
}
