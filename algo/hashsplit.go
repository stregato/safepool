package algo

import (
	"bytes"
	"fmt"
	"hash"
	"io"
	"strconv"
	"strings"

	"github.com/code-to-go/safepool/core"

	"github.com/chmduquesne/rollinghash/buzhash32"
	"golang.org/x/crypto/blake2b"
)

const windowSize = 32

type HashBlock struct {
	Hash   []byte
	Length uint32
}

func getHashBlock(hashFun hash.Hash, length uint32, bufs ...[]byte) HashBlock {
	hashFun.Reset()
	for _, buf := range bufs {
		hashFun.Write(buf)
	}
	block := HashBlock{
		Length: length,
	}
	copy(block.Hash[:], hashFun.Sum(nil))
	return block
}

func HashSplit(r io.Reader, splitBits uint, hashFun hash.Hash) (blocks []HashBlock, err error) {
	var zeroes [windowSize]byte

	if hashFun == nil {
		hashFun, err = blake2b.New256(nil)
		if core.IsErr(err, "cannot create blake2b hash function: %v") {
			return nil, err
		}
	}

	h := buzhash32.New()
	h.Write(zeroes[:])
	mask := uint32(0xffffffff)
	mask = mask >> uint32(32-splitBits)

	buf := make([]byte, 0, mask*2)
	inp := make([]byte, 1024)

	for {
		n, err := r.Read(inp)
		if err == io.EOF {
			if len(buf) > 0 {
				blocks = append(blocks, getHashBlock(hashFun, uint32(len(buf)), buf))
			}
			break
		} else if err != nil {
			return nil, err
		}

		step := 1
		for i := 0; i < n; i += step {
			b := inp[i]
			h.Roll(inp[i])
			buf = append(buf, b)

			sum32 := h.Sum32()
			if sum32&mask == mask {
				blocks = append(blocks, getHashBlock(hashFun, uint32(len(buf)), buf))
				buf = buf[:0]
			}
		}
	}

	return blocks, err
}

type EditOp int

const (
	EditOpInsert EditOp = iota
	EditOpDelete
)

type Range struct {
	Start  uint32
	Length uint32
}

type Edit struct {
	Slice Range
	With  Range
}

func (h *HashBlock) String() string {
	return fmt.Sprintf("%d", h.Length)
}

//ab b

func HashDiff2(source, dest []HashBlock) []Edit {
	sLen := len(source)
	dLen := len(dest)
	column := make([]int, sLen+1)
	actions := make([][]Edit, sLen)

	var diffs []Edit
	var sOffset, dOffset uint32

	for y := 1; y <= sLen; y++ {
		column[y] = y
	}

	for x := 1; x <= dLen; x++ {
		column[0] = x
		lastkey := x - 1
		for y := 1; y <= sLen; y++ {
			oldkey := column[y]
			var incr int

			if bytes.Compare(source[y-1].Hash[:], dest[x-1].Hash[:]) != 0 {
				incr = 1
			}

			insert := column[y] + 1
			delete := column[y-1] + 1
			if insert <= delete && insert <= lastkey+incr {
				column[y] = insert
			} else if delete <= lastkey+incr {
				column[y] = delete
			} else {
				column[y] = lastkey + incr
				if incr > 0 {
				}
			}
			lastkey = oldkey
			sOffset += source[y-1].Length
		}
		dOffset += dest[x-1].Length
	}
	println(actions)
	return diffs
}

func sameBlock(a, b HashBlock) bool {
	return bytes.Equal(a.Hash[:], b.Hash[:])
}

func HashDiff(source, dest []HashBlock) []Edit {
	var i, j int
	sLen := len(source)
	dLen := len(dest)
	// ln := min(sLen, dLen)

	// for i < ln && bytes.Compare(source[i].Hash[:], dest[i].Hash[:]) == 0 {
	// 	i++
	// }
	// for j < ln && bytes.Compare(source[sLen-j-1].Hash[:], dest[dLen-j-1].Hash[:]) == 0 {
	// 	j++
	// }

	var sOffset, dOffset uint32
	var edits []Edit
	for i < sLen && j < dLen {
		s := source[i]
		d := dest[j]
		switch {
		case sameBlock(s, d):
			i++
			j++
			sOffset += s.Length
			dOffset += d.Length
		case i+1 < sLen && sameBlock(source[i+1], d):
			//Delete operation
			edits = append(edits, Edit{
				Slice: Range{sOffset, s.Length},
				With:  Range{dOffset, 0},
			})
			j++
			sOffset += s.Length
		case j+1 < dLen && sameBlock(s, dest[j+1]):
			edits = append(edits, Edit{
				Slice: Range{sOffset, 0},
				With:  Range{dOffset, d.Length},
			})
			i++
			dOffset += d.Length
		default:
			edits = append(edits, Edit{
				Slice: Range{sOffset, s.Length},
				With:  Range{dOffset, d.Length},
			})
			i++
			j++
			sOffset += s.Length
			dOffset += d.Length
		}
	}

	return edits

}

const (
	traceMatch = iota
	traceReplace
	traceInsert
	traceDelete
)

func traceMatrixToString(edits [][]int) string {
	b := strings.Builder{}

	for i := 1; i < len(edits); i++ {
		b.WriteRune('|')
		for j := 1; j < len(edits[i]); j++ {
			b.WriteString(strconv.Itoa(edits[i][j]))
			b.WriteRune(' ')
		}
		b.WriteRune('|')
		b.WriteRune('\n')
	}
	return b.String()
}

func levenshteinEditDistance(dest, source []HashBlock) []Edit {
	sLen := len(source)
	dLen := len(dest)
	column := make([]int, sLen+1)
	trace := make([][]int, dLen+1)

	var sOffset, dOffset uint32

	for y := 1; y <= sLen; y++ {
		column[y] = y
	}

	for x := 1; x <= dLen; x++ {
		trace[x] = make([]int, sLen+1)
		column[0] = x
		lastkey := x - 1
		for y := 1; y <= sLen; y++ {
			oldkey := column[y]
			i := 0

			tr := traceMatch
			if bytes.Compare(source[y-1].Hash[:], dest[x-1].Hash[:]) != 0 {
				i = 1
				tr = traceReplace
			}

			cost := lastkey + i
			tr = traceReplace
			if column[y]+1 < cost {
				cost = column[y] + 1
				tr = traceInsert
			}
			if column[y-1]+1 < cost {
				cost = column[y-1] + 1
				tr = traceDelete
			}

			column[y] = cost
			trace[x][y] = tr

			lastkey = oldkey
			sOffset += source[y-1].Length
		}
		dOffset += dest[x-1].Length
	}
	print(traceMatrixToString(trace))
	return reconstructEdit(source, dest, trace)
}

func reconstructEdit(source, dest []HashBlock, trace [][]int) []Edit {
	i := len(trace) - 1
	j := len(trace[i]) - 1

	var edits []Edit
	for i > 0 && j > 0 {
		switch trace[i][j] {
		case traceMatch:
			i, j = i-1, j-1
			println("skip", i, j)
		case traceInsert:
			j -= 1
			println("insert", i, j)
			edits = append(edits, Edit{})
		case traceDelete:
			i -= 1
			println("delete", i, j)
			edits = append(edits, Edit{})

		case traceReplace:
			i, j = i-1, j-1
			println("replace", i, j)
			edits = append(edits, Edit{})
		default:
			i = 0
		}
	}
	return edits
}
