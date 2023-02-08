package algo

import (
	"hash"
	"io"
	"os"
)

type MerkleTree struct {
	DataLength uint32
	Blocks     []HashBlock
}

var UseSimd bool

func MerkleTreeFromFile(name string, splitBits uint) (MerkleTree, error) {
	f, err := os.Open(name)
	if err != nil {
		return MerkleTree{}, err
	}
	defer f.Close()

	return MerkleTreeFromReader(f, splitBits)
}

func MerkleTreeFromReader(r io.Reader, splitBits uint) (MerkleTree, error) {
	blocks, err := HashSplit(r, splitBits, nil)
	if err != nil {
		return MerkleTree{}, err
	}

	start := 0
	for start < len(blocks)-1 {
		//		blocks, start = buildMerkleRow(blake, length, blocks, start)
	}

	return MerkleTree{
		Blocks: blocks,
	}, nil
}

// buildMerkleRow calculates a new row of a Merkle tree starting from start position until the end of the
func buildMerkleRow(blake hash.Hash, fileLength uint32, blocks []HashBlock, start int) (blocks2 []HashBlock, start2 int) {
	l := len(blocks)
	if l%2 == 1 {
		blocks = append(blocks, getHashBlock(blake, fileLength))
		l++
	}

	for i := start; i < l; i += 2 {
		block := getHashBlock(blake, blocks[i].Length, blocks[i].Hash[:], blocks[i+1].Hash[:])
		blocks = append(blocks, block)
	}
	return blocks, l
}

func MerkleTreeRoot(m MerkleTree) (HashBlock, int) {
	l := len(m.Blocks) - 1
	return m.Blocks[l], l
}

func MerkleTreeHash(m MerkleTree) *[]byte {
	l := len(m.Blocks) - 1
	return &m.Blocks[l].Hash
}

func MerkleTreeAt(m MerkleTree, idx int) (block HashBlock, length uint32) {
	block = m.Blocks[idx]
	if idx+1 == len(m.Blocks) || m.Blocks[idx+1].Length <= m.Blocks[idx].Length {
		return block, m.DataLength - block.Length
	} else {
		return block, m.Blocks[idx+1].Length - block.Length
	}
}

func MerkleTreeChildren(m MerkleTree, idx int) (left int, right int) {
	return 0, 0
}

func min(a, b int) int {
	if a < b {
		return a
	} else {
		return b
	}
}
