package urkel

import (
	"bytes"
)

type ProofType uint8

const (
	DEADEND ProofType = iota
	COLLISION
	EXISTS
	UNKNOWN // Used?
)

type ProofCode uint8

const (
	OK ProofCode = iota
	HASH_MISMATCH
	SAME_KEY
	UNKNOWN_ERROR
)

type ProofResult struct {
	Code  ProofCode
	Value []byte
}

func NewProofResult(t ProofCode, v []byte) *ProofResult {
	return &ProofResult{
		Code:  t,
		Value: v,
	}
}

type Proof struct {
	Type       ProofType
	NodeHashes [][]byte
	Key        []byte
	Hash       []byte
	Value      []byte
}

func NewProof() *Proof {
	return &Proof{
		Type:       DEADEND,
		NodeHashes: make([][]byte, 0),
		Key:        nil, // being explicit
		Hash:       nil,
		Value:      nil,
	}
}

func (p *Proof) Depth() int {
	return len(p.NodeHashes)
}

func (p *Proof) Push(n []byte) {
	p.NodeHashes = append(p.NodeHashes, n)
}

func (p *Proof) IsSane(hasher Hasher, bits int) bool {
	// TODO: Need other sanity checks
	if p.Depth() > bits {
		return false
	}

	switch p.Type {
	case DEADEND:
		if p.Key != nil || p.Hash != nil || p.Value != nil {
			return false
		}
		break
	case COLLISION:
		if p.Key == nil {
			return false
		}
		if p.Hash == nil {
			return false
		}
		if p.Value != nil {
			return false
		}
		if len(p.Key) != (bits >> 3) {
			return false
		}
		if len(p.Hash) != int(hasher.GetSize()) {
			return false
		}
		break
	case EXISTS:
		if p.Key != nil {
			return false
		}
		if p.Hash != nil {
			return false
		}
		if p.Value == nil {
			return false
		}
		if len(p.Value) > 0xffff {
			return false
		}
		break
	default:
		return false
	}
	return true
}

func (p *Proof) Verify(root, key []byte, hasher Hasher, bits int) *ProofResult {

	if !p.IsSane(hasher, bits) {
		return NewProofResult(UNKNOWN_ERROR, nil)
	}

	leaf := hasher.ZeroHash()
	switch p.Type {
	case DEADEND:
		//leaf = hasher.ZeroHash()
		break
	case COLLISION:
		if bytes.Compare(p.Key, key) == 0 {
			return NewProofResult(SAME_KEY, nil)
		}
		leaf = hasher.Hash(leafNodeHashPrefix, p.Key, p.Hash)
		break
	case EXISTS:
		leaf = leafHashValue(hasher, key, p.Value)
		break
	}

	next := leaf

	for i := p.Depth() - 1; i >= 0; i-- {
		n := p.NodeHashes[i]

		if HasBit(key, uint(i)) {
			next = hasher.Hash(internalNodeHashPrefix, n, next)
		} else {
			next = hasher.Hash(internalNodeHashPrefix, next, n)
		}
	}

	if bytes.Compare(next, root) != 0 {
		return NewProofResult(HASH_MISMATCH, nil)
	}

	return NewProofResult(OK, p.Value)
}
