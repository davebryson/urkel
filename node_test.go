package urkel

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Encoded internal node
const encodedInternal = "0200a00500009ad22cf03d7906fa4d0420bbafa751f891cd4ebd3843427249c9ab5dae13f86801002c0d0000d5c3e3d7a2ccf624c4e208a4ff54c73f92879e2f98da82d81ac5835af070c122"
const encodedLeaf = "03003a230e000a0095f0f3d5d1efe58f4e620cc781a2fa4b2d910267ec03cb8a3966962af27cbbf5"

func TestNodeCodec(t *testing.T) {
	assert := assert.New(t)

	internalbits, err := hex.DecodeString(encodedInternal)
	assert.Nil(err)
	leafbits, err := hex.DecodeString(encodedLeaf)
	assert.Nil(err)
	assert.Equal(76, len(internalbits))
	assert.Equal(40, len(leafbits))

	n, err := DecodeNode(internalbits, false)
	assert.Nil(err)
	assert.NotNil(n)
	assert.False(n.isLeaf())

	n, err = DecodeNode(leafbits, true)
	assert.Nil(err)
	assert.NotNil(n)
	assert.True(n.isLeaf())
}
