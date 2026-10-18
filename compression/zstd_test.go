package compression

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/klauspost/compress/zstd"
)

// stream mimics the gateway's zstd-stream: one frame, flushed at the end of every message
type stream struct {
	buf bytes.Buffer
	enc *zstd.Encoder
}

func newStream(t *testing.T) *stream {
	s := &stream{}
	enc, err := zstd.NewWriter(&s.buf)
	if err != nil {
		t.Fatal(err)
	}
	s.enc = enc
	return s
}

func (s *stream) message(t *testing.T, d []byte) []byte {
	s.buf.Reset()
	if _, err := s.enc.Write(d); err != nil {
		t.Fatal(err)
	}
	if err := s.enc.Flush(); err != nil {
		t.Fatal(err)
	}
	return bytes.Clone(s.buf.Bytes())
}

func TestZstdDecompress(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	random := func(n int) []byte {
		b := make([]byte, n)
		rng.Read(b)
		return b
	}
	repeated := func(n int) []byte {
		return bytes.Repeat([]byte(`{"id":"123","name":"channel"},`), n/30+1)[:n]
	}

	messages := [][]byte{
		repeated(1000),
		repeated(5_000_000),      // large and highly compressible, e.g. GUILD_CREATE
		repeated(100),            // references history from previous messages
		random(1_000_000),        // compressed form spans many blocks
		repeated(maxBlockSize),   // exactly one block
		random(3 * maxBlockSize), // exact multiple of the block size
		repeated(10),
	}

	s := newStream(t)
	z := NewZstd()
	for i, msg := range messages {
		out, err := z.Decompress(s.message(t, msg))
		if err != nil {
			t.Fatalf("message %d (%d bytes): %s", i, len(msg), err)
		}
		if !bytes.Equal(out, msg) {
			t.Fatalf("message %d: got %d bytes, want %d", i, len(out), len(msg))
		}
	}
}
