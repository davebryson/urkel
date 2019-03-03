package urkel

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	TestFile = "hello.txt"
)

func TestListFiles(t *testing.T) {
	assert := assert.New(t)
	/*files, err := loadFiles("sample")
	assert.Nil(err)
	assert.NotNil(files)
	assert.Equal(3, len(files))

	assert.Equal("sample/0000000003", files[0])*/

	// Check the filename validation
	assert.Equal(int64(0), validateFilename("meta"))
	assert.Equal(int64(3), validateFilename("0000000003"))
	assert.Equal(int64(1), validateFilename("0000000001"))
	assert.Equal(int64(0), validateFilename("00000001"))

	files, err := loadFiles("data")
	assert.Nil(err)
	assert.Equal(0, len(files))
}

func TestMetaFile(t *testing.T) {
	assert := assert.New(t)
	hashFn := &Sha256{}

	m := &meta{
		metaIndex: 1,
		metaPos:   100,
		rootIndex: 1,
		rootPos:   64*2 + 1, // leaf
	}

	encoded := m.Encode(hashFn)
	assert.NotNil(encoded)
	b, err := DecodeMeta(encoded)
	assert.Nil(err)
	assert.NotNil(b)
	assert.True(b.rootIsLeaf)
	assert.Equal(uint32(64), b.rootPos)
	assert.Equal(uint32(100), b.metaPos)

}
