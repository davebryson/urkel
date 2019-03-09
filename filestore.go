package urkel

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
)

// TODO:  Need to maintain a small 'pool' of files for reading and 1 file for writing

var _ Store = (*FileStore)(nil)

type FileStore struct {
	sync.RWMutex
	buf    bytes.Buffer
	pos    uint32
	index  uint16
	dir    string
	file   *os.File
	state  *meta
	hashFn Hasher
}

func (db *FileStore) Open(dir string, hashfn Hasher) error {
	dirExists, err := exists(dir)
	if !dirExists {
		if err := os.Mkdir(dir, 0700); err != nil {
			return err
		}
	}

	db.hashFn = hashfn
	db.dir = dir

	// Get the current file index
	currenFileindex := uint16(1) // Default
	files, err := loadFiles(dir)
	if err != nil {
		return err
	}
	if len(files) > 0 {
		// If we have valid files - the 'len' of the file list
		// is the current file to use.
		currenFileindex = uint16(len(files))
	}

	db.index = currenFileindex

	// Get the file
	f, fileSize, err := db.getFileHandle()
	if err != nil {
		return err
	}

	db.file = f
	db.pos = uint32(fileSize) // This might be a problem

	// Try to recover the latest state from the meta
	metaState, err := recoverState(f, fileSize, hashfn)
	if err != nil {
		// no state found - maybe the first file
		db.state = &meta{
			metaIndex:  currenFileindex,
			rootIndex:  currenFileindex,
			rootIsLeaf: false,
		}
	} else {
		db.state = metaState
	}

	fmt.Printf("Meta: %v\n", db.state)

	return nil
}

func (db *FileStore) GetRootNode() (node, error) {
	rPos := db.state.rootPos
	isLeaf := db.state.rootIsLeaf
	rIndex := db.state.rootIndex

	fmt.Println("GetRootNode")
	fmt.Printf("rPos: %v\n", rPos)
	fmt.Printf("isLeaf: %v\n", isLeaf)
	fmt.Printf("rIndex: %v\n", rIndex)
	fmt.Println("-------")

	n, err := db.GetNode(rIndex, rPos, isLeaf)
	if err != nil {
		return nil, err
	}
	// If it a leaf - retreive the value, take the leafHashValue of it
	// and set the node.data = hash
	if isLeaf {
		nv := n.(*leafNode)
		key := nv.key
		value := db.GetValue(nv.vIndex, nv.vSize, nv.vPos)
		nv.data = leafHashValue(db.hashFn, key, value)
		return nv, nil
	}
	fmt.Printf("GetRootNode is %v\n", n)

	return n.toHashNode(db.hashFn), nil
}

// Temp for testing...

func (db *FileStore) Close() error {
	if db.file != nil {
		db.file.Sync()
		db.file.Close()
		db.file = nil
		db.buf.Reset()
	}
	// Do I care about returning the error here??
	return nil
}

// WriteNode to the buffer
func (db *FileStore) WriteNode(encodedNode []byte) (uint16, uint32, error) {
	//db.Lock()
	//defer db.Unlock()

	writePos := db.pos
	db.buf.Write(encodedNode)
	db.pos += uint32(len(encodedNode))
	return db.index, writePos, nil
}

// WriteValue to the buffer
func (db *FileStore) WriteValue(val []byte) (uint16, uint32, error) {
	//db.Lock()
	//defer db.Unlock()

	vpos := db.pos
	db.buf.Write(val)
	db.pos += uint32(len(val))
	return db.index, vpos, nil
}

// writeMeta to the buffer
func (db *FileStore) writeMeta(i uint16, p uint32, isleaf bool) {
	//db.Lock()
	//defer db.Unlock()

	db.state.rootPos = p
	db.state.rootIndex = i
	db.state.metaIndex = db.index
	db.state.rootIsLeaf = isleaf

	padSize := MetaSize - (db.pos % MetaSize)
	padding := pad(padSize)
	db.buf.Write(padding)

	db.pos += uint32(padSize)
	db.state.metaPos = db.pos

	encodedMeta := db.state.Encode(db.hashFn)
	db.buf.Write(encodedMeta)

	db.pos += uint32(MetaSize)
}

// Commit - write to file
func (db *FileStore) Commit(i uint16, p uint32, isleaf bool) error {
	if db.file == nil {
		fmt.Println("Store commit - file is closed")
		// TODO: This is where we should check file size in the future...
		f, _, err := db.getFileHandle()
		if err != nil {
			return err
		}
		db.file = f
	}

	// Add lock

	// 1. Write meta
	db.writeMeta(i, p, isleaf)

	// 2. dump to file
	//db.Lock()
	//defer db.Unlock()

	if _, err := db.buf.WriteTo(db.file); err != nil {
		return err
	}

	//db.pos += uint32(n)

	if err := db.file.Sync(); err != nil {
		return err
	}
	db.buf.Reset()

	return nil
}

// GetValue - file read from the store
func (db *FileStore) GetValue(index uint16, size uint16, pos uint32) []byte {
	//db.RLock()
	//defer db.RUnlock()
	// params should be the value location and size
	// read from the file and return the value
	bits := make([]byte, size)
	fmt.Printf("Try to read value @ %v\n", pos)
	_, err := db.file.ReadAt(bits, int64(pos))
	if err != nil {
		fmt.Printf("Error reading value %s", err)
		return nil
	}
	return bits
}

func (db *FileStore) GetNode(index uint16, pos uint32, isLeaf bool) (node, error) {
	//db.RLock()
	//defer db.RUnlock()

	bitSize := internalSize
	if isLeaf {
		bitSize = leafSize
	}
	bits := make([]byte, bitSize)
	_, err := db.file.ReadAt(bits, int64(pos))
	if err != nil {
		return nil, err
	}

	n, err := DecodeNode(bits, isLeaf)
	if err != nil {
		return nil, err
	}
	return n, nil
}

// ----- TODO Below for real app ------ //
func (db *FileStore) createFilename(index uint16) string {
	n := fmt.Sprintf("%010d", index)
	return filepath.Join(db.dir, n)
}

// File management
func (db *FileStore) getFileHandle() (*os.File, int64, error) {
	fn := db.createFilename(db.index)
	f, err := os.OpenFile(fn, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, 0, err
	}

	info, err := f.Stat()
	if err != nil {
		return nil, 0, err
	}
	fileSize := info.Size()

	return f, fileSize, nil
}

func validateFilename(fn string) int64 {
	if len(fn) < 10 {
		return 0
	}
	i, err := strconv.ParseInt(fn, 10, 64)
	if err != nil {
		return 0
	}
	return i
}

// loadFiles scans the given 'dir' for files
// with the proper filename format
func loadFiles(dir string) ([]string, error) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	cachedList := make([]string, 0)
	for _, fi := range files {
		// Check for a validfilename in the form '000000000X' - ignore others
		val := validateFilename(fi.Name())
		if val > int64(0) {
			fullFn := filepath.Join(dir, fi.Name())
			cachedList = append(cachedList, fullFn)
		}
	}
	// Sort in descending order so the latest files is [0]
	sort.Sort(sort.Reverse(sort.StringSlice(cachedList)))
	return cachedList, nil
}

// Zero fill
func pad(size uint32) []byte {
	d := make([]byte, size)
	for i := range d {
		d[i] = 0
	}
	return d
}

func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}
