package urkel

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

const (
	MetaMagic   = 1836215148
	MetaSize    = 36
	MaxFileSize = 2147479552 // 2gb
	KeySize     = 32
)

// TODO:  Need to maintain a 'pool' of files and they should
// be kept open while running...

type meta struct {
	metaIndex  uint16
	metaPos    uint32
	rootIndex  uint16
	rootPos    uint32
	rootIsLeaf bool
}

func DecodeMeta(data []byte) (*meta, error) {

	if len(data) != MetaSize {
		return nil, fmt.Errorf("Expecting meta data size of 36")
	}

	hasher := Sha256{}
	expectedCheckSum := hasher.Hash(data[:16])[:20]

	buf := bytes.NewReader(data)
	var mmagic uint32
	var mindex uint16
	var mpos uint32
	var rindex uint16
	var rpos uint32
	actualChecksum := make([]byte, 20)

	err := binary.Read(buf, binary.LittleEndian, &mmagic)
	if err != nil {
		return nil, err
	}

	if mmagic != MetaMagic {
		return nil, fmt.Errorf("Not the right magic")
	}

	err = binary.Read(buf, binary.LittleEndian, &mindex)
	if err != nil {
		return nil, err
	}
	err = binary.Read(buf, binary.LittleEndian, &mpos)
	if err != nil {
		return nil, err
	}
	err = binary.Read(buf, binary.LittleEndian, &rindex)
	if err != nil {
		return nil, err
	}
	err = binary.Read(buf, binary.LittleEndian, &rpos)
	if err != nil {
		return nil, err
	}
	_, err = buf.Read(actualChecksum)
	if err != nil {
		return nil, err
	}

	if bytes.Compare(expectedCheckSum, actualChecksum) != 0 {
		return nil, fmt.Errorf("Checksum don't match")
	}

	isLeaf := false
	if rpos&1 == 1 {
		isLeaf = true
	}
	rpos >>= 1

	return &meta{
		metaIndex:  mindex,
		metaPos:    mpos,
		rootIndex:  rindex,
		rootPos:    rpos,
		rootIsLeaf: isLeaf,
	}, nil
}

func (m *meta) Encode() []byte {
	b := make([]byte, MetaSize)
	offset := 0

	binary.LittleEndian.PutUint32(b[offset:], MetaMagic)
	offset += 4
	binary.LittleEndian.PutUint16(b[offset:], m.metaIndex)
	offset += 2
	binary.LittleEndian.PutUint32(b[offset:], m.metaPos)
	offset += 4
	binary.LittleEndian.PutUint16(b[offset:], m.rootIndex)
	offset += 2
	binary.LittleEndian.PutUint32(b[offset:], m.rootPos)
	offset += 4

	hasher := Sha256{}
	hashed := hasher.Hash(b[:16])

	// TODO: This should use the meta random key...
	copy(b[offset:], hashed[:20]) // We all use the first 20 bytes
	return b
}

type Store struct {
	buf         bytes.Buffer
	pos         uint32
	index       uint16
	dir         string
	currentFile *os.File
	state       *meta
}

func OpenDb(dir string) *Store {
	// Scan the latest file in dir
	// read the meta for index, etc...
	// bootup
	// IF no files, set index to 1
	/*files, err := loadFiles(dir)
	if err != nil {
		panic(err)
	}
	if len(files) == 0 {
		// First file in the DB!

	}*/

	// Test file index
	i := uint16(1)

	// For testing
	f, err := getOrCreateFile(dir, i)
	if err != nil {
		panic(err)
	}

	metaState, err := recoverState(f)
	if err != nil {
		// New DB
		metaState = &meta{
			metaIndex: 1,
			rootIndex: 1,
		}

		return &Store{
			pos:         0,
			index:       i,
			dir:         dir,
			currentFile: f,
			state:       metaState,
		}
	}

	return &Store{
		pos:         0,
		index:       i,
		dir:         dir,
		currentFile: f,
		state:       metaState,
	}
}

func (st *Store) GetRootNode() (node, error) {
	rPos := st.state.rootPos
	isLeaf := st.state.rootIsLeaf
	rIndex := st.state.rootIndex

	hashFn := &Sha256{}
	n, err := st.Resolve(rIndex, rPos, isLeaf)
	if err != nil {
		return nil, err
	}
	// If it a leaf - retreive the value, take the leafHashValue of it
	// and set the node.data = hash
	if isLeaf {
		nv := n.(*leafNode)
		key := nv.key
		value := st.Retrieve(nv.vIndex, nv.vSize, nv.vPos)
		hd := leafHashValue(hashFn, key, value)
		nv.params.data = hd
		return nv, nil
	}
	return n.toHashNode(hashFn), nil
}

// Temp for testing...

func (st *Store) Close() {
	st.currentFile.Sync()
	st.currentFile.Close()
}

func recoverState(currentFile *os.File) (*meta, error) {
	if currentFile == nil {
		return nil, fmt.Errorf("Current file is not open")
	}

	info, err := currentFile.Stat()
	if err != nil {
		fmt.Println("Stat error")
		return nil, err
	}
	fileSize := info.Size()
	fmt.Printf("File size: %v\n", fileSize)
	startPos := fileSize - (fileSize % MetaSize)

	metaBuffer := make([]byte, MetaSize)

	for {
		for {
			startPos -= MetaSize
			if startPos <= 0 {
				return nil, fmt.Errorf("Can't find meta - at <= 0")
			}
			_, err := currentFile.ReadAt(metaBuffer, startPos)
			if err != nil {
				fmt.Println("ReadAt error")
				return nil, err
			}

			buf := bytes.NewReader(metaBuffer)
			var mmagic uint32
			err = binary.Read(buf, binary.LittleEndian, &mmagic)
			if err != nil {
				fmt.Println("Magic parse error")
				return nil, err
			}

			if mmagic == MetaMagic {
				break
			}
		}

		// Found a meta header - try to decode
		m, err := DecodeMeta(metaBuffer)
		if err != nil {
			fmt.Println("Decode error")
			return nil, err
		}
		return m, nil
	}
}

func (st *Store) WriteNode(encodedNode []byte) (uint16, uint32, error) {
	writePos := st.pos
	n, err := st.buf.Write(encodedNode)
	if err != nil {
		return 0, 0, err
	}
	st.pos += uint32(n)
	return st.index, writePos, nil
}

func (st *Store) WriteValue(val []byte) (uint16, uint32, error) {
	vpos := st.pos
	n, err := st.buf.Write(val)
	if err != nil {
		return 0, 0, err
	}
	st.pos += uint32(n)
	return st.index, vpos, nil
}

func (st *Store) writeMeta(root node) error {
	rPos := root.getParams().flags //
	rIndex := root.getParams().index

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

	encodedMeta := st.state.Encode()
	n, err := st.buf.Write(encodedMeta)
	if err != nil {
		return err
	}

	st.pos += uint32(n)
	return nil
}

func (st *Store) Commit(root node) {
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
func (st *Store) Retrieve(index uint16, size uint16, pos uint32) []byte {
	// params should be the value location and size
	// read from the file and return the value
	fmt.Printf("Looking for %v bits @ pos %v\n", size, pos)
	bits := make([]byte, size)
	_, err := st.currentFile.ReadAt(bits, int64(pos))
	if err != nil {
		fmt.Printf("Error reading value %s", err)
		return nil
	}
	return bits
}

// Resolve a given hashnode and shapeshift to leaf/internal
func (st *Store) Resolve(index uint16, pos uint32, isLeaf bool) (node, error) {
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
func getOrCreateFile(dir string, index uint16) (*os.File, error) {
	n := fmt.Sprintf("%010d", index)
	fn := filepath.Join(dir, n)
	f, err := os.OpenFile(fn, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// ----- TODO Below for real app ------ //
func (st *Store) createFilename(index uint16) string {
	n := fmt.Sprintf("%010d", index)
	return filepath.Join(st.dir, n)
}

// File management
func (st *Store) getFileHandle(index uint16) *os.File {
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
