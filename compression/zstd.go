package compression

import (
	"io"

	"github.com/valyala/gozstd"
)

// Zstd represents a de/compression context. Zero value is not valid.
type Zstd struct {
	cw *gozstd.Writer
	cr *ChanWriter
	dw io.Writer
	dr *ChanWriter
}

// NewZstd creates a valid zstd context
func NewZstd() *Zstd {
	cr := &ChanWriter{make(chan []byte)}
	zw := gozstd.NewWriter(cr)

	dr, dw := io.Pipe()
	zr := gozstd.NewReader(dr)
	dChanWriter := &ChanWriter{make(chan []byte)}
	go zr.WriteTo(dChanWriter)
	return &Zstd{zw, cr, dw, dChanWriter}
}

// Compress compresses the given bytes and returns the compressed form
func (z *Zstd) Compress(d []byte) []byte {
	z.cw.Write(d)
	go z.cw.Flush()
	return <-z.cr.C
}

// Decompress decompresses the given bytes and returns the decompressed form
func (z *Zstd) Decompress(d []byte) ([]byte, error) {
	z.dw.Write(d)
	return <-z.dr.C, nil
}
