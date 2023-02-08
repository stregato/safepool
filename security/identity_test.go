package security

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIdentity(t *testing.T) {

	identity, err := NewIdentity("test")
	assert.NoErrorf(t, err, "cannot create identity")

	data, err := json.Marshal(identity.Public())
	assert.NoErrorf(t, err, "cannot marshal private identity")
	print(string(data))

}
