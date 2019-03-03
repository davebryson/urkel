package urkel

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

// TODO:  Need to maintain a 'pool' of files and they should
// be kept open while running...

var _ Store = (*FileStore)(nil)

type FileStore struct {
	buf         bytes.Buffer
	pos         uint32
	index       uint16
	dir         string
	currentFile *os.File
	state       *meta
	hashFn      Hasher
}

func (st *FileStore) Open(dir string, hashfn Hasher) {
	// Scan the latest file in dir
	// read the meta for index, etc...
	// bootup
	// IF no files, set index to 1

	currenFileindex := uint16(1)
	files, err := loadFiles(dir)
	if err != nil {
		panic(err)
	}
	if len(files) > 0 {
		// If we have valid files the len of the file list
		// is the current file to use.
		currenFileindex = uint16(len(files))
	}

	// Get the file
	f, fileSize, err := getOrCreateFile(dir, currenFileindex)
	if err != nil {
		panic(err)
	}

	// Try to recover the latest state from the meta
	metaState, err := recoverState(f, fileSize)
	if err != nil {
		// no state found - maybe the first file
		st.state = &meta{
			metaIndex: currenFileindex,
			rootIndex: currenFileindex,
		}
	} else {
		st.state = metaState
	}

	st.pos = uint32(fileSize)
	st.index = currenFileindex
	st.dir = dir
	st.currentFile = f
	st.hashFn = hashfn
}

func (st *FileStore) GetRootNode() (node, error) {
	rPos := st.state.rootPos
	isLeaf := st.state.rootIsLeaf
	rIndex := st.state.rootIndex

	n, err := st.GetNode(rIndex, rPos, isLeaf)
	if err != nil {
		return nil, err
	}
	// If it a leaf - retreive the value, take the leafHashValue of it
	// and set the node.data = hash
	if isLeaf {
		nv := n.(*leafNode)
		key := nv.key
		value := st.GetValue(nv.vIndex, nv.vSize, nv.vPos)
		nv.data = leafHashValue(st.hashFn, key, value)
		return nv, nil
	}
	return n.toHashNode(st.hashFn), nil
}

// Temp for testing...

func (st *FileStore) Close() {
	st.currentFile.Sync()
	st.currentFile.Close()
}

func (st *FileStore) WriteNode(encodedNode []byte) (uint16, uint32, error) {
	writePos := st.pos
	n, err := st.buf.Write(encodedNode)
	if err != nil {
		return 0, 0, err
	}
	st.pos += uint32(n)
	return st.index, writePos, nil
}

func (st *FileStore) WriteValue(val []byte) (uint16, uint32, error) {
	vpos := st.pos
	n, err := st.buf.Write(val)
	if err != nil {
		return 0, 0, err
	}
	st.pos += uint32(n)
	return st.index, vpos, nil
}

func (st *FileStore) writeMeta(root node) error {
	rPos := root.getFlags()
	rIndex := root.getIndex()

	st.state.rootPos = rPos
	st.state.rootIndex = rIndex
	st.state.metaIndex = st.index

	padSize := MetaSize - (st.pos % MetaSize)
	padding := pad(padSize)
	_, err := st.buf.Write(padding)
	if err != nil {
		return err
	}
	st.pos += uint32(padSize)
	st.state.metaPos = st.pos

	encodedMeta := st.state.Encode(st.hashFn)
	n, err := st.buf.Write(encodedMeta)
	if err != nil {
		return err
	}

	st.pos += uint32(n)
	return nil
}

func (st *FileStore) Commit(root node) {
	// 1. Write meta to buffer
	err := st.writeMeta(root)
	if err != nil {
		panic(err)
	}
	// 2. dump to file
	st.buf.WriteTo(st.currentFile)
	st.currentFile.Sync()
	st.buf.Reset()
}

// Retrieve a value from the file for a give leafNode
func (st *FileStore) GetValue(index uint16, size uint16, pos uint32) []byte {
	// params should be the value location and size
	// read from the file and return the value
	bits := make([]byte, size)
	_, err := st.currentFile.ReadAt(bits, int64(pos))
	if err != nil {
		fmt.Printf("Error reading value %s", err)
		return nil
	}
	return bits
}

// Resolve a given hashnode and shapeshift to leaf/internal
func (st *FileStore) GetNode(index uint16, pos uint32, isLeaf bool) (node, error) {
	bitSize := internalSize
	if isLeaf {
		bitSize = leafSize
	}
	bits := make([]byte, bitSize)
	_, err := st.currentFile.ReadAt(bits, int64(pos))
	if err != nil {
		fmt.Printf("Error reading value %s", err)
		return nil, err
	}
	fmt.Printf("Got %v bits\n", len(bits))
	fmt.Printf("%x\n", bits)
	return DecodeNode(bits, isLeaf)
}

// Temp function
func getOrCreateFile(dir string, index uint16) (*os.File, int64, error) {
	n := fmt.Sprintf("%010d", index)
	fn := filepath.Join(dir, n)

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

// ----- TODO Below for real app ------ //
func (st *FileStore) createFilename(index uint16) string {
	n := fmt.Sprintf("%010d", index)
	return filepath.Join(st.dir, n)
}

// File management
func (st *FileStore) getFileHandle(index uint16) *os.File {
	fn := st.createFilename(index)
	f, err := os.OpenFile(fn, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		panic(err)
	}
	return f
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
