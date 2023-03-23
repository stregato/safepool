package security

import (
	"hash"
	"io"
	"os"

	"github.com/code-to-go/safepool/core"

	"golang.org/x/crypto/blake2b"
)

//type Hash256 [blake2b.Size256]byte

type HashReader struct {
	r io.ReadSeekCloser

	size int64
	Hash hash.Hash
}

type HashWriter struct {
	w    io.Writer
	size int64
	Hash hash.Hash
}

func NewHash() hash.Hash {
	h, err := blake2b.New256(nil)
	if err != nil {
		panic(err)
	}
	return h
}

func QuickHash(data []byte) []byte {
	h := NewHash()
	return h.Sum(data)
}

func NewHashReader(r io.ReadSeekCloser) (*HashReader, error) {
	b, err := blake2b.New256(nil)
	if core.IsErr(err, "cannot create black hash: %v") {
		return nil, err
	}
	return &HashReader{
		Hash: b,
		r:    r,
	}, nil
}

func NewHashWriter(w io.Writer) (*HashWriter, error) {
	b, err := blake2b.New256(nil)
	if core.IsErr(err, "cannot create black hash: %v") {
		return nil, err
	}
	return &HashWriter{
		Hash: b,
		w:    w,
	}, nil
}

func (s *HashReader) Read(p []byte) (n int, err error) {
	if s.r == nil {
		return 0, os.ErrClosed
	}

	n, err = s.r.Read(p)
	if err == nil && n > 0 {
		_, err = s.Hash.Write(p[0:n])
	}
	s.size += int64(n)
	return n, err
}

func (s *HashReader) Seek(offset int64, whence int) (int64, error) {
	if s.r == nil {
		return 0, os.ErrClosed
	} else {
		return s.r.Seek(offset, whence)
	}
}

func (s *HashReader) Close() error {
	return s.r.Close()
}

func (s *HashWriter) Write(p []byte) (n int, err error) {
	if s.w == nil {
		return 0, os.ErrClosed
	}

	n, err = s.w.Write(p)
	if err == nil && n > 0 {
		_, err = s.Hash.Write(p[0:n])
	}
	s.size += int64(n)
	return n, err
}

func FileHash(name string) ([]byte, error) {
	h := NewHash()

	f, err := os.Open(name)
	if core.IsErr(err, "cannot open file '%s': %v", name) {
		return nil, err
	}

	_, err = io.Copy(h, f)
	if core.IsErr(err, "cannot read file '%s': %v", name) {
		return nil, err
	}

	return h.Sum(nil), nil
}
