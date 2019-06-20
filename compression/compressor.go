package compression

// Compressor is something that can de/compress data
type Compressor interface {
	Compress([]byte) []byte
	Decompress([]byte) ([]byte, error)
}
