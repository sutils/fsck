package main

import (
	"io"
)

// buffered output

// BufferedWriter implements buffering for an io.BufferedWriter object.
// If an error occurs writing to a BufferedWriter, no more data will be
// accepted and all subsequent writes, and Flush, will return the error.
// After all data has been written, the client should call the
// Flush method to guarantee all data has been forwarded to
// the underlying io.BufferedWriter.
type BufferedWriter struct {
	err error
	buf []byte
	n   int
	wr  io.Writer
}

const defaultBufSize = 1024

// NewBufferedWriterSize returns a new BufferedWriter whose buffer has at least the specified
// size. If the argument io.BufferedWriter is already a BufferedWriter with large enough
// size, it returns the underlying BufferedWriter.
func NewBufferedWriterSize(w io.Writer, size int) *BufferedWriter {
	// Is it already a BufferedWriter?
	b, ok := w.(*BufferedWriter)
	if ok && len(b.buf) >= size {
		return b
	}
	if size <= 0 {
		size = defaultBufSize
	}
	return &BufferedWriter{
		buf: make([]byte, size),
		wr:  w,
	}
}

func (b *BufferedWriter) Bytes() []byte {
	return b.buf[0:b.n]
}

// Flush writes any buffered data to the underlying io.BufferedWriter.
func (b *BufferedWriter) Flush() error {
	if b.err != nil {
		return b.err
	}
	if b.n == 0 {
		return nil
	}
	n, err := b.wr.Write(b.buf[0:b.n])
	if n < b.n && err == nil {
		err = io.ErrShortWrite
	}
	if err != nil {
		if n > 0 && n < b.n {
			copy(b.buf[0:b.n-n], b.buf[n:b.n])
		}
		b.n -= n
		b.err = err
		return err
	}
	b.n = 0
	return nil
}

// Available returns how many bytes are unused in the buffer.
func (b *BufferedWriter) Available() int { return len(b.buf) - b.n }

// Buffered returns the number of bytes that have been written into the current buffer.
func (b *BufferedWriter) Buffered() int { return b.n }

// Write writes the contents of p into the buffer.
// It returns the number of bytes written.
// If nn < len(p), it also returns an error explaining
// why the write is short.
func (b *BufferedWriter) Write(p []byte) (nn int, err error) {
	for len(p) > b.Available() && b.err == nil {
		var n int
		if b.Buffered() == 0 {
			// Large write, empty buffer.
			// Write directly from p to avoid copy.
			n, b.err = b.wr.Write(p)
		} else {
			n = copy(b.buf[b.n:], p)
			b.n += n
			b.Flush()
		}
		nn += n
		p = p[n:]
	}
	if b.err != nil {
		return nn, b.err
	}
	n := copy(b.buf[b.n:], p)
	b.n += n
	nn += n
	return nn, nil
}
