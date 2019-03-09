package urkel

// Db is the interface for all tree storage
type Db interface {
	Set(key []byte, value []byte)
	Get(key []byte) (node, error)
	GetRoot() (node, error)
	ReadValue(key []byte) []byte
	Commit(root node, h Hasher) node
}
