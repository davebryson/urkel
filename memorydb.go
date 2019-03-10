package urkel

import (
	"encoding/hex"
	"fmt"
	"sync"
)

var _ Db = (*MemoryDb)(nil)

type MemoryDb struct {
	sync.RWMutex
	cache map[string][]byte
}

func OpenDb() *MemoryDb {
	return &MemoryDb{
		cache: make(map[string][]byte),
	}
}

func (db *MemoryDb) Set(key []byte, value []byte) {
	db.Lock()
	defer db.Unlock()
	k := hex.EncodeToString(key)
	db.cache[k] = value
}

func (db *MemoryDb) Get(key []byte) (node, error) {
	db.RLock()
	defer db.RUnlock()
	k := hex.EncodeToString(key)
	n := db.cache[k]
	return DecodeNode(n)
}

func (db *MemoryDb) ReadValue(key []byte) []byte {
	db.RLock()
	defer db.RUnlock()
	k := hex.EncodeToString(key)
	return db.cache[k]
}

func (db *MemoryDb) Commit(root node, h Hasher) node {
	db.Lock()
	defer db.Unlock()
	switch nn := root.(type) {
	case *hashNode:
		db.cache[RootKey] = nn.data
		break
	default:
		panic("Trying to commit the wrong node")
	}
	return root
}

func (db *MemoryDb) GetRoot() (node, error) {
	db.RLock()
	defer db.RUnlock()
	raw := db.cache[RootKey]
	k := hex.EncodeToString(raw)
	rootBits := db.cache[k]
	return DecodeNode(rootBits)
}

func (db *MemoryDb) Close() {
	db.cache = nil
	fmt.Println(" %%%%% CLOSE %%%%%")
}
