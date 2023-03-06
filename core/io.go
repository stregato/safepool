package core

import (
	"bytes"
	"io"
)

type BytesReader struct {
	r *bytes.Reader
}

func NewBytesReader(bs []byte) io.ReadSeekCloser {
	return BytesReader{bytes.NewReader(bs)}
}

func NewStringReader(s string) io.ReadSeekCloser {
	return BytesReader{bytes.NewReader([]byte(s))}
}

func (r BytesReader) Close() error {
	return nil
}

func (r BytesReader) Read(p []byte) (n int, err error) {
	return r.r.Read(p)
}

func (r BytesReader) WriteTo(w io.Writer) (n int64, err error) {
	return r.r.WriteTo(w)
}

func (r BytesReader) Seek(offset int64, whence int) (int64, error) {
	return r.r.Seek(offset, whence)
}
