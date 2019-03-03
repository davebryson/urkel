package urkel

const (
	MetaMagic      = 1836215148
	MetaSize       = 36
	MaxFileSize    = 2147479552 // 2gb
	KeySizeInBytes = 32
	KeySizeInBits  = 256
)

var (
	// Node hash prefixes
	internalNodeHashPrefix = []byte{0x01}
	leafNodeHashPrefix     = []byte{0x00}
	// Storage size
	leafSize     = 2 + 4 + 2 + KeySizeInBytes   // 40
	internalSize = (2 + 4 + KeySizeInBytes) * 2 // 76
)
