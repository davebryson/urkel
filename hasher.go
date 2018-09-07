package urkel

import "crypto/sha256"

// Hasher interface to support different hash implementations
type Hasher interface {
	Hash(content ...[]byte) []byte
	GetSize() uint
	ZeroHash() []byte
}

// Sha256 implementation
type Sha256 struct{}

// GetSize return the size of the hash output in bytes
func (h *Sha256) GetSize() uint { return uint(32) }

// Hash do the hash()
func (h *Sha256) Hash(data ...[]byte) []byte {
	hash := sha256.New()
	for _, d := range data {
		hash.Write(d)
	}
	return hash.Sum(nil)
}

// ZeroHash  Based on the urkel spec - return zeros as a deadend marker
func (h *Sha256) ZeroHash() []byte {
	return []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
}
