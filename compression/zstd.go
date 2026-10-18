package compression

import (
	"bytes"
	"errors"

	"github.com/klauspost/compress/zstd"
)

// maxBlockSize is the largest amount of data a single zstd block can decompress to
const maxBlockSize = 128 << 10

// Zstd represents a zstd-stream decompression context. Zero value is not valid.
//
// The gateway sends a single zstd frame over the lifetime of a connection and flushes it at the end
// of every message, so each message is a whole number of blocks that depend on the blocks before it.
type Zstd struct {
	src *bytes.Reader
	dec *zstd.Decoder

	// buf has room for more than a whole block, so each read decodes exactly one block and leaves
	// nothing buffered in the decoder
	buf []byte
}

// NewZstd creates a valid zstd context
func NewZstd() *Zstd {
	src := bytes.NewReader(nil)

	// a concurrency of 1 decodes synchronously, reading only as much input as each block needs
	dec, err := zstd.NewReader(src, zstd.WithDecoderConcurrency(1))
	if err != nil {
		panic(err)
	}

	return &Zstd{src, dec, make([]byte, maxBlockSize+1)}
}

// Decompress decompresses the given message and returns the decompressed form
func (z *Zstd) Decompress(d []byte) ([]byte, error) {
	if len(d) == 0 {
		return []byte{}, nil
	}

	z.src.Reset(d)
	var out []byte
	for z.src.Len() > 0 {
		n, err := z.dec.Read(z.buf)
		out = append(out, z.buf[:n]...)
		if err != nil {
			return out, err
		}
		if n == 0 {
			return out, errors.New("zstd: no progress decoding message")
		}
	}

	return out, nil
}
