package security

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"strings"

	"github.com/code-to-go/safepool/core"
)

type PublicKey ed25519.PublicKey
type PrivateKey ed25519.PrivateKey

const (
	PublicKeySize  = ed25519.PublicKeySize
	PrivateKeySize = ed25519.PrivateKeySize
	SignatureSize  = ed25519.SignatureSize
)

type SignedData struct {
	Signature [SignatureSize]byte
	Signer    PublicKey
}

type Public struct {
	Id    PublicKey
	Nick  string
	Email string
}

func Sign(identity Identity, data []byte) ([]byte, error) {
	private := identity.SignatureKey.Private
	return ed25519.Sign(ed25519.PrivateKey(private), data), nil
}

func Verify(id string, data []byte, sig []byte) bool {
	id2 := strings.ReplaceAll(id, "_", "/")
	public, err := base64.StdEncoding.DecodeString(id2)
	if core.IsErr(err, "invalid id '%s': %v", id) {
		return false
	}

	for off := 0; off < len(sig); off += SignatureSize {
		if func() bool {
			defer func() { recover() }()
			return ed25519.Verify(ed25519.PublicKey(public[0:ed25519.PublicKeySize]), data, sig[off:off+SignatureSize])
		}() {
			return true
		}
	}
	return false
}

type SignedHashEvidence struct {
	Key       []byte `json:"k"`
	Signature []byte `json:"s"`
}

type SignedHash struct {
	Hash       []byte
	Signatures map[string][]byte
}

func NewSignedHash(hash []byte, i Identity) (SignedHash, error) {
	signature, err := Sign(i, hash)
	if core.IsErr(err, "cannot sign with identity %s: %v", base64.StdEncoding.EncodeToString(i.SignatureKey.Public)) {
		return SignedHash{}, err
	}

	return SignedHash{
		Hash:       hash,
		Signatures: map[string][]byte{i.Id(): signature},
	}, nil
}

func AppendToSignedHash(s SignedHash, i Identity) error {
	signature, err := Sign(i, s.Hash)
	if core.IsErr(err, "cannot sign with identity %s: %v", base64.StdEncoding.EncodeToString(i.SignatureKey.Public)) {
		return err
	}
	s.Signatures[i.Id()] = signature
	return nil
}

func VerifySignedHash(s SignedHash, trusts []Identity, hash []byte) bool {
	if !bytes.Equal(s.Hash, hash) {
		return false
	}

	for _, trust := range trusts {
		id := trust.Id()
		if signature, ok := s.Signatures[id]; ok {
			if Verify(id, hash, signature) {
				return true
			}
		}
	}
	return false
}
