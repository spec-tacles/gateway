package compression

// ChanWriter is a writer than sends all writes to a channel
type ChanWriter struct {
	C chan []byte
}

func (w *ChanWriter) Write(d []byte) (int, error) {
	w.C <- d
	return len(d), nil
}

// Close this writer
func (w *ChanWriter) Close() error {
	close(w.C)
	return nil
}
