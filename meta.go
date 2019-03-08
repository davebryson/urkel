package urkel

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
)

// meta is stored at the end of commit and contains information about the last
// root and the file it lives in.  On startup the current log file is read
// in reverse to search for the meta to bootstrap the tree.
type meta struct {
	metaIndex  uint16
	metaPos    uint32
	rootIndex  uint16
	rootPos    uint32
	rootIsLeaf bool
}

// DecodeMeta takes the bytes and thaws to the meta
func DecodeMeta(data []byte, hashFn Hasher) (*meta, error) {

	if len(data) != MetaSize {
		return nil, fmt.Errorf("Expecting meta data size of 36")
	}

	// TODO: This needs to refactored - passing the hashFn from the store
	// it also needs a 'key' from a separate 'meta' file
	expectedCheckSum := hashFn.Hash(data[:16])[:20]

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

	isLeaf, tpos := GetTagForPosition(rpos)

	return &meta{
		metaIndex:  mindex,
		metaPos:    mpos,
		rootIndex:  rindex,
		rootPos:    tpos,
		rootIsLeaf: isLeaf == 1,
	}, nil
}

func (m *meta) Encode(hashFn Hasher) []byte {
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

	tpos := TagPosition(m.rootPos, m.rootIsLeaf)
	binary.LittleEndian.PutUint32(b[offset:], tpos)
	offset += 4

	hashed := hashFn.Hash(b[:16])

	// TODO: This should use the meta random key...
	copy(b[offset:], hashed[:20]) // We all use the first 20 bytes
	return b
}

func recoverState(currentFile *os.File, fileSize int64, hashFn Hasher) (*meta, error) {
	if currentFile == nil {
		panic("recover state = file is not open")
	}

	if fileSize == 0 {
		return nil, fmt.Errorf("Log is empty")
	}

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
				return nil, err
			}

			buf := bytes.NewReader(metaBuffer)
			var mmagic uint32
			err = binary.Read(buf, binary.LittleEndian, &mmagic)
			if err != nil {
				return nil, err
			}

			if mmagic == MetaMagic {
				break
			}
		}

		// Found a meta header - try to decode
		m, err := DecodeMeta(metaBuffer, hashFn)
		if err != nil {
			fmt.Printf("Error decoding meta: %v\n", err)
			return nil, err
		}

		return m, nil
	}
}
