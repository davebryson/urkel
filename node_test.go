package urkel

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

var sample = "0200a00500009ad22cf03d7906fa4d0420bbafa751f891cd4ebd3843427249c9ab5dae13f86801002c0d0000d5c3e3d7a2ccf624c4e208a4ff54c73f92879e2f98da82d81ac5835af070c122"

func TestNodeBasics(t *testing.T) {
	assert := assert.New(t)
	bits, err := hex.DecodeString(sample)
	assert.Nil(err)
	assert.Equal(76, len(bits))
	n, err := DecodeNode(bits, false)
	assert.Nil(err)
	assert.NotNil(n)
}
