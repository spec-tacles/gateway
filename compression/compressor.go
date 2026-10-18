package compression

// Compressor is something that can decompress data
type Compressor interface {
	Decompress([]byte) ([]byte, error)
}
