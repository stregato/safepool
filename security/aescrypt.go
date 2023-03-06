package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/code-to-go/safepool/core"
	"github.com/uhthomas/seekctr"

	"github.com/zenazn/pkcs7pad"
)

func GenerateBytesKey(size int) []byte {
	key := make([]byte, size)
	_, err := rand.Read(key)
	if err != nil {
		panic(err)
	}
	return key
}

func EncryptBlock(key []byte, nonce []byte, data []byte) ([]byte, error) {
	block, err := newBlock(key)
	if err != nil {
		return nil, err
	}

	data = pkcs7pad.Pad(data, aes.BlockSize)
	cipherdata := make([]byte, len(data))

	mode := cipher.NewCBCEncrypter(block, nonce)
	mode.CryptBlocks(cipherdata, data)
	return cipherdata, nil
}

func DecryptBlock(key []byte, nonce []byte, cipherdata []byte) ([]byte, error) {
	block, err := newBlock(key)
	if err != nil {
		return nil, err
	}

	data := make([]byte, len(cipherdata))
	mode := cipher.NewCBCDecrypter(block, nonce)
	mode.CryptBlocks(data, cipherdata)

	data, err = pkcs7pad.Unpad(data)
	if core.IsErr(err, "invalid padding in AES decrypted data: %v") {
		return nil, err
	}
	return data, nil
}

type StreamReader struct {
	loc    int64
	header []byte
	r      *seekctr.Reader
}

const AESHeaderSize = 8 + aes.BlockSize

func (sr *StreamReader) Read(p []byte) (n int, err error) {
	if sr.loc < AESHeaderSize {
		m := copy(p[sr.loc:], sr.header)
		n, err = sr.r.Read(p[m:])
		if err == nil {
			sr.loc += int64(m + n)
		}
		return m + n, err
	} else {
		n, err := sr.r.Read(p)
		sr.loc += int64(n)
		return n, err
	}
}

func (sr *StreamReader) Close() error {
	return sr.r.Close()
}

func (sr *StreamReader) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		sr.loc = offset
	case io.SeekCurrent:
		sr.loc += offset

	case io.SeekEnd:
		loc, err := sr.r.Seek(offset, whence)
		if err == nil {
			sr.loc = loc + AESHeaderSize
			return sr.loc, nil
		} else {
			return 0, err
		}
	}

	if sr.loc < AESHeaderSize {
		_, err := sr.r.Seek(0, 0)
		return offset, err
	} else {
		_, err := sr.r.Seek(sr.loc-AESHeaderSize, 0)
		return sr.loc - AESHeaderSize, err
	}
}

// EncryptedWriter wraps w with an OFB cipher stream.
func EncryptingReader(keyId uint64, keyFunc func(uint64) []byte, r io.ReadSeekCloser) (io.ReadSeekCloser, error) {

	header := make([]byte, AESHeaderSize)
	binary.LittleEndian.PutUint64(header, keyId)

	// generate random initial value
	if _, err := io.ReadFull(rand.Reader, header[8:]); err != nil {
		return nil, err
	}

	key := keyFunc(keyId)
	if key == nil {
		return nil, fmt.Errorf("unknown encryption key %d in #EncryptingReader", keyId)
	}

	iv := header[8:]
	reader, err := seekctr.NewReader(r, key, iv)
	if core.IsErr(err, "cannot create encryption reader: %v") {
		return nil, err
	}

	return &StreamReader{
		header: header,
		r:      reader,
	}, nil
}

type StreamWriter struct {
	loc     int
	header  []byte
	keyFunc func(uint64) []byte
	ew      *seekctr.Writer
	w       io.Writer
}

func (sr *StreamWriter) Write(p []byte) (n int, err error) {
	if sr.ew == nil {
		m := copy(sr.header[sr.loc:], p)
		sr.loc += m

		if sr.loc == 8+aes.BlockSize {
			keyId := binary.LittleEndian.Uint64(sr.header)
			key := sr.keyFunc(keyId)
			if key == nil {
				return 0, fmt.Errorf("unknown encryption key %d in #StreamWriter.Write", keyId)
			}

			iv := sr.header[8:]
			sr.ew, err = seekctr.NewWriter(sr.w, key, iv)
			if err != nil {
				return 0, err
			}
		}
		n, err := sr.ew.Write(p[m:])
		return n + m, err
	} else {
		return sr.ew.Write(p)
	}
}

// EncryptedWriter wraps w with an OFB cipher stream.
func DecryptingWriter(keyFunc func(uint64) []byte, w io.Writer) (*StreamWriter, error) {
	return &StreamWriter{
		keyFunc: keyFunc,
		header:  make([]byte, 8+aes.BlockSize),
		w:       w,
	}, nil
}

func newBlock(key []byte) (cipher.Block, error) {
	sh := sha256.Sum256(key)
	hash := md5.Sum(sh[:])
	block, err := aes.NewCipher(hash[:])
	if err != nil {
		return nil, err
	}
	return block, nil
}
