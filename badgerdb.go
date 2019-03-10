package urkel

import (
	"fmt"
	"sync"

	"github.com/dgraph-io/badger"
)

type dataCacheObj struct {
	value []byte
	dirty bool
}

var _ Db = (*KeyValueStore)(nil)

// KeyValueStore backed by badgerdb.  Writes from the tree are cached and flushed to the
// db on commit.  Reads check the cache first, and db second.
type KeyValueStore struct {
	sync.RWMutex
	db    *badger.DB
	cache map[string]dataCacheObj
}

func OpenBadger(dir string) (*KeyValueStore, error) {
	opts := badger.DefaultOptions
	opts.Dir = dir
	opts.ValueDir = dir
	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}
	return &KeyValueStore{
		db:    db,
		cache: make(map[string]dataCacheObj),
	}, nil
}

func (kv *KeyValueStore) Set(k, v []byte) {
	kv.cache[string(k)] = dataCacheObj{v, true}
}

// Get a node
func (kv *KeyValueStore) Get(k []byte) (node, error) {

	key := string(k)
	// check the cache
	data := kv.cache[key]
	if data.value != nil {
		return DecodeNode(data.value)
	}
	// Go to db
	bits, err := kv.fetchFromDb(k)
	if err != nil {
		return nil, err
	}

	// Cache and decode
	if bits != nil {
		kv.cache[key] = dataCacheObj{bits, false}
		return DecodeNode(bits)
	}
	return nil, fmt.Errorf("not found in cache or db")
}

func (kv *KeyValueStore) ReadValue(k []byte) []byte {

	key := string(k)
	data := kv.cache[key]
	if data.value != nil {
		return data.value
	}
	// Go to db
	bits, err := kv.fetchFromDb(k)
	if err != nil {
		return nil
	}

	if bits != nil {
		kv.cache[key] = dataCacheObj{bits, false}
		return bits
	}
	return nil
}

func (kv *KeyValueStore) GetRoot() (node, error) {
	bits, err := kv.fetchFromDb([]byte(RootKey))
	if err != nil {
		return nil, err
	}
	return kv.Get(bits)
}

func (kv *KeyValueStore) Commit(root node, h Hasher) node {
	// Write the cache to the db
	err := kv.flushCacheToDb()
	if err != nil {
		return nil
	}
	// Write the root
	return kv.writeRoot(root)
}

func (kv *KeyValueStore) Close() {
	kv.db.Close()
}

// **** Db access

func (kv *KeyValueStore) fetchFromDb(k []byte) ([]byte, error) {
	tx := kv.db.NewTransaction(false)
	defer func() {
		tx.Discard()
	}()

	item, err := tx.Get(k)
	if err != nil {
		return nil, err
	}
	var c []byte
	r, err := item.ValueCopy(c)
	if err != nil {
		return nil, err
	}
	return r, nil
}

// See commit
func (kv *KeyValueStore) flushCacheToDb() error {
	tx := kv.db.NewTransaction(true)
	defer func() {
		tx.Discard()
	}()
	for k, data := range kv.cache {
		if data.value == nil || !data.dirty {
			continue
		}
		if err := tx.Set([]byte(k), data.value); err != nil {
			fmt.Printf("Err writing Tx. %v\n", err)
			return err
		}
	}
	if err := tx.Commit(nil); err != nil {
		fmt.Printf("Err on Cache commit. %v\n", err)
		return nil
	}
	return nil
}

func (kv *KeyValueStore) writeRoot(root node) node {
	tx := kv.db.NewTransaction(true)
	defer func() {
		tx.Discard()
	}()
	switch nn := root.(type) {
	case *hashNode:
		if err := tx.Set([]byte(RootKey), nn.data); err != nil {
			fmt.Printf("Err on writing root. %v\n", err)
			return nil
		}
		if err := tx.Commit(nil); err != nil {
			fmt.Printf("Err on root commit. %v\n", err)
			return nil
		}
		break
	default:
		panic("Trying to commit the wrong node")
	}
	return root
}
