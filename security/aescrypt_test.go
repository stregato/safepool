package security

import (
	"bytes"
	"crypto/rand"
	"io"
	"testing"

	"github.com/code-to-go/safepool/core"
	"github.com/stretchr/testify/assert"
)

func TestAESCrypt(t *testing.T) {

	b := make([]byte, 1024)
	rand.Read(b)

	b = []byte("Hello")
	key := GenerateBytesKey(32)
	keyFunc := func(_ uint64) []byte {
		return key
	}

	r := core.NewBytesReader(b)
	er, _ := EncryptingReader(0, keyFunc, r)
	w := &bytes.Buffer{}

	io.Copy(w, er)

	w2 := &bytes.Buffer{}
	ew, _ := DecryptingWriter(keyFunc, w2)
	io.Copy(ew, w)

	assert.Equal(t, w2.Bytes(), b)
}

func TestAESCryptAndHash(t *testing.T) {

	b := make([]byte, 1024)
	rand.Read(b)

	key := GenerateBytesKey(32)
	keyFunc := func(_ uint64) []byte {
		return key
	}

	r := core.NewBytesReader(b)
	s1, _ := NewHashReader(r)
	er, _ := EncryptingReader(0, keyFunc, s1)
	w := &bytes.Buffer{}

	io.Copy(w, er)

	w2 := &bytes.Buffer{}
	s2, _ := NewHashWriter(w)
	ew, _ := DecryptingWriter(keyFunc, s2)
	io.Copy(ew, w)

	assert.Equal(t, w2.Bytes(), b)
	assert.Equal(t, s1.Hash.Sum(nil), s2.Hash.Sum(nil))
}
