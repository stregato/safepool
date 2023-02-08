package pool

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/code-to-go/safe/safepool/core"
	"github.com/code-to-go/safe/safepool/security"
)

type Token struct {
	Config Config
	Host   security.Identity
}

func EncodeToken(t Token, guestId string) (string, error) {
	tk, err := json.Marshal(Token{
		Config: Config{
			Name:   t.Config.Name,
			Public: t.Config.Public,
		},
		Host: t.Host.Public(),
	})
	if core.IsErr(err, "cannot marshal config to token: %v") {
		return "", err
	}

	sig, err := security.Sign(t.Host, tk)
	if core.IsErr(err, "cannot sign with host key: %v") {
		return "", err
	}

	flag := 0
	if guestId != "" {
		identity, ok, _ := security.GetIdentity(guestId)
		if !ok {
			return "", ErrNotAuthorized
		}

		tk, err = security.EcEncrypt(identity, tk)
		if core.IsErr(err, "cannot encrypt with guest identity: %v") {
			return "", err
		}
		flag = 1
	}

	return fmt.Sprintf("%d:%s:%s",
		flag,
		base64.StdEncoding.EncodeToString(tk),
		base64.StdEncoding.EncodeToString(sig)), nil

}

func DecodeToken(guest security.Identity, token string) (Token, error) {
	var t Token
	parts := strings.Split(token, ":")
	if len(parts) != 3 {
		return t, ErrInvalidToken
	}

	var err error
	flag, tk64, sig64 := parts[0], parts[1], parts[2]
	tk, _ := base64.StdEncoding.DecodeString(tk64)
	sig, _ := base64.StdEncoding.DecodeString(sig64)
	if flag == "1" {
		tk, err = security.EcDecrypt(guest, tk)
		if core.IsErr(err, "cannot decode with guest '%s': %v", guest) {
			return t, ErrInvalidToken
		}
	}

	err = json.Unmarshal(tk, &t)
	if core.IsErr(err, "cannot unmarshal token: %s") {
		return t, ErrInvalidToken
	}

	if !security.Verify(t.Host.Id(), tk, sig) {
		core.IsErr(ErrInvalidSignature, "token has invalid signature: %v")
		return t, ErrInvalidSignature
	}

	return t, nil
}
