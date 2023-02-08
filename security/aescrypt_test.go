package security

import (
	"bytes"
	"crypto/rand"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAESCrypt(t *testing.T) {

	b := make([]byte, 1024)
	rand.Read(b)

	key := GenerateBytesKey(32)
	keyFunc := func(_ uint64) []byte {
		return key
	}

	r := bytes.NewBuffer(b)
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

	r := bytes.NewBuffer(b)
	s1, _ := NewHashStream(r, nil)
	er, _ := EncryptingReader(0, keyFunc, s1)
	w := &bytes.Buffer{}

	io.Copy(w, er)

	w2 := &bytes.Buffer{}
	s2, _ := NewHashStream(nil, w)
	ew, _ := DecryptingWriter(keyFunc, s2)
	io.Copy(ew, w)

	assert.Equal(t, w2.Bytes(), b)
	assert.Equal(t, s1.Hash(), s2.Hash())
}
