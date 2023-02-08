package core

import (
	"encoding/binary"
	"io"
)

func WriteBytes(data []byte, w io.Writer) error {
	lenB := make([]byte, 4)
	binary.BigEndian.PutUint32(lenB, uint32(len(data)))
	n, err := w.Write(lenB)
	if err == nil && n != len(lenB) {
		return io.ErrShortWrite
	}
	if IsErr(err, "cannot write data to stream: %v") {
		return err
	}

	n, err = w.Write(data)
	if err == nil && n != len(data) {
		return io.ErrShortWrite
	}
	IsErr(err, "cannot write data to stream: %v")
	return err
}

func ReadBytes(r io.Reader) ([]byte, error) {
	lenB := make([]byte, 4)
	n, err := r.Read(lenB)
	if err == io.EOF || IsErr(err, "cannot read data from stream: %v") {
		return nil, err
	}
	if n != len(lenB) {
		return nil, io.ErrNoProgress
	}

	data := make([]byte, binary.BigEndian.Uint32(lenB))
	n = 0
	cnt := 4 + len(data)/64
	for n < len(data) {
		m, err := r.Read(data[n:])
		if err == io.EOF || IsErr(err, "cannot read data from stream: %v") {
			return nil, err
		}
		n += m
		cnt--
		if cnt == 0 {
			return nil, io.ErrNoProgress
		}
	}

	return data, nil
}
