package data

import "github.com/grailbio/base/simd"

func accumulate(bs []byte) int {
	return simd.Accumulate8(bs)
}
