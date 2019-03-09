package urkel

const (
	keySizeInBytes     = 32
	keySizeInBits      = 256
	leafSize       int = 34
	internalSize   int = 66
	// RootKey is the (temp) key used in storage for the root node
	RootKey string = "98abb2b2f00b87d1320caebec7540d3c2f35a7986d5977ef129a515a04ee7507"
)

var (
	internalNodeHashPrefix = []byte{0x01}
	leafNodeHashPrefix     = []byte{0x00}
)
