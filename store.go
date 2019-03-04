package urkel

// Store - methods that should be implemented by any store
type Store interface {
	// OpenStore
	Open(dir string, hashFn Hasher) error

	// Close the underlying file,etc..
	Close()

	// GetRootNode - either the cached one or pulled from meta
	GetRootNode() (node, error)

	// GetValue returns the value associated with a leaf key
	GetValue(index uint16, size uint16, pos uint32) []byte

	// GetNode returns a leaf, internal node
	GetNode(index uint16, pos uint32, isLeaf bool) (node, error)

	// WriteNode stores an encoded node
	WriteNode(encodedNode []byte) (uint16, uint32, error)

	// WriteValue stores a the actual value associated with a leaf node
	WriteValue(val []byte) (uint16, uint32, error)

	// Commit the root node. Writes out nodes and meta
	Commit(root node) error
}
