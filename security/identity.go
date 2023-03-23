package security

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"

	"github.com/code-to-go/safepool/core"

	eciesgo "github.com/ecies/go/v2"
)

var ErrInvalidSignature = errors.New("signature is invalid")

const (
	Secp256k1 = "secp256k1"
	Ed25519   = "ed25519"
)

type Key struct {
	Public  []byte `json:"pu"`
	Private []byte `json:"pr,omitempty"`
}

type Identity struct {
	Nick  string `json:"n"`
	Email string `json:"m"`

	SignatureKey  Key `json:"s"`
	EncryptionKey Key `json:"e"`

	Trusted []string `json:"t"`
	Avatar  []byte   `json:"a"`
}

func NewIdentity(nick string) (Identity, error) {
	var identity Identity

	identity.Nick = nick
	privateCrypt, err := eciesgo.GenerateKey()
	if core.IsErr(err, "cannot generate secp256k1 key: %v") {
		return identity, err
	}
	identity.EncryptionKey = Key{
		Public:  privateCrypt.PublicKey.Bytes(true),
		Private: privateCrypt.Bytes(),
	}

	publicSign, privateSign, err := ed25519.GenerateKey(rand.Reader)
	if core.IsErr(err, "cannot generate ed25519 key: %v") {
		return identity, err
	}
	identity.SignatureKey = Key{
		Public:  publicSign[:],
		Private: privateSign[:],
	}
	return identity, nil
}

func (i Identity) Public() Identity {
	return Identity{
		Nick:  i.Nick,
		Email: i.Email,
		EncryptionKey: Key{
			Public: i.EncryptionKey.Public,
		},
		SignatureKey: Key{
			Public: i.SignatureKey.Public,
		},
	}
}

func IdentityFromBase64(b64 string) (Identity, error) {
	var i Identity
	data, err := base64.StdEncoding.DecodeString(b64)
	if core.IsErr(err, "cannot decode Identity string in base64: %v") {
		return i, err
	}

	err = json.Unmarshal(data, &i)
	if core.IsErr(err, "cannot decode Identity string from json: %v") {
		return i, err
	}
	return i, nil
}

const secp256k1PublicKeySize = 33

func IdentityFromId(id string) (Identity, error) {
	id2 := strings.ReplaceAll(id, "_", "/")
	b, err := base64.StdEncoding.DecodeString(id2)
	if core.IsErr(err, "invalid id: %v") {
		return Identity{}, err
	}

	if len(b) != ed25519.PublicKeySize+secp256k1PublicKeySize {
		return Identity{}, core.ErrInvalidId
	}

	return Identity{
		SignatureKey: Key{
			Public: b[0:+ed25519.PublicKeySize],
		},
		EncryptionKey: Key{
			Public: b[ed25519.PublicKeySize:],
		},
	}, nil
}

func (i Identity) Base64() (string, error) {
	data, err := json.Marshal(i)
	if core.IsErr(err, "cannot marshal identity: %v") {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(data), nil
}

func (i Identity) Id() string {
	b := append(i.SignatureKey.Public, i.EncryptionKey.Public...)
	b64 := base64.StdEncoding.EncodeToString(b)
	return strings.ReplaceAll(b64, "/", "_")
}

func SameIdentity(a, b Identity) bool {
	return bytes.Equal(a.SignatureKey.Public, b.SignatureKey.Public) &&
		bytes.Equal(a.EncryptionKey.Public, b.EncryptionKey.Public)
}

func SetIdentity(i Identity) error {
	return sqlSetIdentity(i)
}

func GetIdentity(id string) (identity Identity, ok bool, err error) {
	identity, err = sqlGetIdentity(id)
	switch err {
	case nil:
		return identity, true, nil
	case sql.ErrNoRows:
		return identity, false, nil
	default:
		return identity, false, err
	}
}

func SetAlias(i Identity, alias string) error {
	return sqlSetAlias(i, alias)
}

func Trust(i Identity, trusted bool) error {
	return sqlSetTrust(i, trusted)
}

func Trusted() ([]Identity, error) {
	return sqlGetIdentities(true)
}

func Identities() ([]Identity, error) {
	return sqlGetIdentities(false)
}
