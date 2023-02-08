package security

import (
	"hash"
	"io"
	"os"

	"github.com/code-to-go/safepool/core"

	"golang.org/x/crypto/blake2b"
)

//type Hash256 [blake2b.Size256]byte

type HashStream struct {
	r     io.Reader
	w     io.Writer
	size  int64
	block hash.Hash
}

func NewHash() hash.Hash {
	h, err := blake2b.New256(nil)
	if err != nil {
		panic(err)
	}
	return h
}

func NewHashStream(r io.Reader, w io.Writer) (*HashStream, error) {
	b, err := blake2b.New256(nil)
	if core.IsErr(err, "cannot create black hash: %v") {
		return nil, err
	}
	return &HashStream{
		block: b,
		r:     r,
		w:     w,
	}, nil
}

func (s *HashStream) Read(p []byte) (n int, err error) {
	if s.r == nil {
		return 0, os.ErrClosed
	}

	n, err = s.r.Read(p)
	if err == nil && n > 0 {
		_, err = s.block.Write(p[0:n])
	}
	s.size += int64(n)
	return n, err
}

func (s *HashStream) Write(p []byte) (n int, err error) {
	if s.w == nil {
		return 0, os.ErrClosed
	}

	n, err = s.w.Write(p)
	if err == nil && n > 0 {
		_, err = s.block.Write(p[0:n])
	}
	s.size += int64(n)
	return n, err
}

func (s *HashStream) Hash() []byte {
	return s.block.Sum(nil)
}

func (s *HashStream) Size() int64 {
	return s.size
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
