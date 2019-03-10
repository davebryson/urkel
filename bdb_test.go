package urkel

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDbBasic(t *testing.T) {
	assert := assert.New(t)
	kv, err := OpenBadger("data")
	defer kv.Close()
	assert.Nil(err)
	assert.NotNil(kv)

	kv.Set([]byte("dave"), []byte("bryson"))
	result, err := kv.Get([]byte("dave"))
	assert.Nil(err)
	assert.Equal([]byte("bryson"), result)

}
