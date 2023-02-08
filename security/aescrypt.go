package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"

	"github.com/code-to-go/safe/safepool/core"

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
	loc    int
	header []byte
	r      cipher.StreamReader
}

func (sr *StreamReader) Read(p []byte) (n int, err error) {
	if sr.loc < 8+aes.BlockSize {
		m := copy(p[sr.loc:], sr.header)
		sr.loc += m
		n, err = sr.r.Read(p[m:])
		return m + n, err
	} else {
		return sr.r.Read(p)
	}
}

// EncryptedWriter wraps w with an OFB cipher stream.
func EncryptingReader(keyId uint64, keyFunc func(uint64) []byte, r io.Reader) (*StreamReader, error) {

	header := make([]byte, 8+aes.BlockSize)
	binary.LittleEndian.PutUint64(header, keyId)

	// generate random initial value
	if _, err := io.ReadFull(rand.Reader, header[8:]); err != nil {
		return nil, err
	}

	value := keyFunc(keyId)
	if value == nil {
		return nil, errors.New("unknown encryption key")
	}

	block, err := newBlock(value)
	if err != nil {
		return nil, err
	}

	stream := cipher.NewOFB(block, header[8:])
	return &StreamReader{
		header: header,
		r:      cipher.StreamReader{S: stream, R: r},
	}, nil
}

type StreamWriter struct {
	loc     int
	header  []byte
	keyFunc func(uint64) []byte
	w       *cipher.StreamWriter
}

func (sr *StreamWriter) Write(p []byte) (n int, err error) {
	if sr.w.S == nil {
		m := copy(sr.header[sr.loc:], p)
		sr.loc += m

		if sr.loc == 8+aes.BlockSize {
			keyId := binary.LittleEndian.Uint64(sr.header)
			value := sr.keyFunc(keyId)
			if value == nil {
				return 0, errors.New("unknown encryption key")
			}

			block, err := newBlock(value)
			if err != nil {
				return 0, err
			}

			iv := sr.header[8:]
			sr.w.S = cipher.NewOFB(block, iv)
		}
		return sr.w.Write(p[m:])
	} else {
		return sr.w.Write(p)
	}
}

// EncryptedWriter wraps w with an OFB cipher stream.
func DecryptingWriter(keyFunc func(uint64) []byte, w io.Writer) (*StreamWriter, error) {
	return &StreamWriter{
		keyFunc: keyFunc,
		header:  make([]byte, 8+aes.BlockSize),
		w:       &cipher.StreamWriter{S: nil, W: w},
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
