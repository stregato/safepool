package security

import (
	"bytes"
	"crypto/rand"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHashStream(t *testing.T) {

	b := make([]byte, 1024)
	rand.Read(b)

	r := bytes.NewBuffer(b)
	s, _ := NewHashStream(r, nil)
	w := &bytes.Buffer{}

	io.Copy(w, s)
	hash := s.Hash()

	s, _ = NewHashStream(nil, &bytes.Buffer{})
	io.Copy(s, w)

	assert.Equal(t, hash, s.Hash())
}
