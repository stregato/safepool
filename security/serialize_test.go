package security

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSerialize(t *testing.T) {

	identity, err := NewIdentity("test")
	assert.NoErrorf(t, err, "cannot create identity")

	data, err := Marshal(identity, identity, SignatureField)
	assert.NoErrorf(t, err, "cannot marshal private identity")
	print(string(data))

	var i Identity
	Unmarshal(data, &i, SignatureField)
	assert.Equal(t, identity, i)
}
