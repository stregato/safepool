package algo

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Hashsplit(t *testing.T) {
	s := "This is a simple string test"
	blocks, err := HashSplit(bytes.NewBufferString(s), 13, nil)
	assert.NoErrorf(t, err, "Cannot split hash: %v", err)
	assert.Equal(t, len(blocks), 1, "unexpected hashes number")

	hash_str := hex.EncodeToString(blocks[0].Hash[:])
	assert.Equal(t, "5468697320697320612073696d706c6520737472696e6720746573740e5751c0", hash_str,
		"unexpected hash value")
	assert.Equal(t, uint32(0), blocks[0].Length)

	rn := make([]byte, 40000)
	rand.Seed(1975)
	rand.Read(rn)

	blocks, err = HashSplit(bytes.NewBuffer(rn), 13, nil)
	assert.NoErrorf(t, err, "Cannot split hash: %v", err)
	//	assert.Equal(t, len(blocks), 17, "unexpected hashes number")
	for idx, block := range blocks {
		fmt.Printf("Block [%d] %d\n", idx, block.Length)
	}

	blocks2, err := HashSplit(bytes.NewBuffer(rn), 8, nil)
	assert.NoErrorf(t, err, "Cannot split hash: %v", err)
	for idx, block := range blocks2 {
		fmt.Printf("Block [%d] %d\n", idx, block.Length)
	}
	assert.Equal(t, 18, len(blocks2), "unexpected hashes number")

}

func Test_HashDiff(t *testing.T) {
	a := HashBlock{
		Hash:   []byte{0},
		Length: 4,
	}
	b := HashBlock{
		Hash:   []byte{1},
		Length: 8,
	}
	c := HashBlock{
		Hash:   []byte{2},
		Length: 8,
	}

	//	diffs := HashDiff([]HashBlock{a}, []HashBlock{b})
	// assert.Len(t, diffs, 1)
	// assert.Equal(t, Diff{0, 0, 4, 0, 8}, diffs[0])

	diffs := HashDiff([]HashBlock{a, b, c}, []HashBlock{a, c})
	//assert.Len(t, diffs, 1)
	print(diffs)
}
