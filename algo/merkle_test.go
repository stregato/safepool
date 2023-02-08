package algo

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/blake2b"
)

func Test_MerkleTree(t *testing.T) {
	rn := make([]byte, 40000)
	rand.Seed(1975)
	rand.Read(rn)

	rn[0] = 16
	m1, err := MerkleTreeFromReader(bytes.NewBuffer(rn), 13)
	assert.NoErrorf(t, err, "Cannot create tree: %v", err)
	assert.Equal(t, uint32(len(rn)), m1.DataLength, "unexpected length")

	rn[0] = 8
	m2, err := MerkleTreeFromReader(bytes.NewBuffer(rn), 13)
	assert.NoErrorf(t, err, "Cannot create tree: %v", err)
	assert.Equal(t, uint32(len(rn)), m2.DataLength, "unexpected length")

	assert.NotEqualValues(t, m1.Blocks[0].Hash, m2.Blocks[0].Hash, "Unexpected same hash for first block")
	assert.EqualValues(t, m1.Blocks[1].Hash, m2.Blocks[1].Hash, "Unexpected different hash for second block")
	assert.NotEqualValues(t, MerkleTreeHash(m1), MerkleTreeHash(m2), "Unexpected same hash")

}

func Benchmark_Merkle(b *testing.B) {
	rn := make([]byte, 1000*1000)
	rand.Seed(1975)
	rand.Read(rn)

	for i := 0; i < 512; i++ {
		MerkleTreeFromReader(bytes.NewBuffer(rn), 13)
	}
}

func Benchmark_Blake(b *testing.B) {
	blake, _ := blake2b.New256(nil)
	rn := make([]byte, 1000*1000)

	for i := 0; i < 512; i++ {
		blake.Reset()
		blake.Sum(rn)
	}

}
