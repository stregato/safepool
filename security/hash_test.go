package security

import (
	"bytes"
	"crypto/rand"
	"io"
	"testing"

	"github.com/code-to-go/safepool/core"
	"github.com/stretchr/testify/assert"
)

func TestHashStream(t *testing.T) {

	b := make([]byte, 1024)
	rand.Read(b)

	r := core.NewBytesReader(b)
	hr, _ := NewHashReader(r)
	w := &bytes.Buffer{}

	io.Copy(w, hr)
	hash := hr.Hash.Sum(nil)

	hw, _ := NewHashWriter(&bytes.Buffer{})
	io.Copy(hw, w)

	assert.Equal(t, hash, hw.Hash.Sum(nil))
}
