package urkel

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	TestFile = "hello.txt"
)

func TestMetaFile(t *testing.T) {
	assert := assert.New(t)
	m := &meta{
		metaIndex: 1,
		metaPos:   100,
		rootIndex: 1,
		rootPos:   64*2 + 1, // leaf
	}

	encoded := m.Encode()
	assert.NotNil(encoded)
	b, err := DecodeMeta(encoded)
	assert.Nil(err)
	assert.NotNil(b)
	assert.True(b.rootIsLeaf)
	assert.Equal(uint32(64), b.rootPos)
	assert.Equal(uint32(100), b.metaPos)

}
