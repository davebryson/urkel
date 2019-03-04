package urkel

const (
	MetaMagic      uint32 = 0x6d726b6c
	MetaSize              = 36
	MaxFileSize           = (1 << 30) * 2 // 2gb
	KeySizeInBytes        = 32
	KeySizeInBits         = 256
	leafSize       int    = 2 + 4 + 2 + KeySizeInBytes
	internalSize   int    = (2 + 4 + KeySizeInBytes) * 2
)

var (
	internalNodeHashPrefix = []byte{0x01}
	leafNodeHashPrefix     = []byte{0x00}
)
