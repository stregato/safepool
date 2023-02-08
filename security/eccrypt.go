package security

import (
	"github.com/code-to-go/safepool/core"

	eciesgo "github.com/ecies/go/v2"
)

func EcEncrypt(identity Identity, data []byte) ([]byte, error) {
	pk, err := eciesgo.NewPublicKeyFromBytes(identity.EncryptionKey.Public)
	if core.IsErr(err, "cannot convert bytes to secp256k1 public key: %v") {
		return nil, err
	}
	data, err = eciesgo.Encrypt(pk, data)
	if core.IsErr(err, "cannot encrypt with secp256k1: %v") {
		return nil, err
	}
	return data, err
}

func EcDecrypt(identity Identity, data []byte) ([]byte, error) {
	data, err := eciesgo.Decrypt(eciesgo.NewPrivateKeyFromBytes(identity.EncryptionKey.Private), data)
	if core.IsErr(err, "cannot decrypt with secp256k1: %v") {
		return nil, err
	}
	return data, nil
}
