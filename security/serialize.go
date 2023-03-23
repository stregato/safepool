package security

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/code-to-go/safepool/core"
)

const SignatureField = "dgst_ed25519_blake2b"

func Marshal(identity Identity, v any, signatureField string) ([]byte, error) {
	data, err := json.Marshal(v)
	if core.IsErr(err, "cannot marshal to json: %v") {
		return data, err
	}

	s := strings.Trim(string(data), " ")
	if len(s) == 0 {
		return nil, &json.MarshalerError{}
	}

	hs := QuickHash([]byte(s))
	signature, err := Sign(identity, hs)
	if core.IsErr(err, "cannot sign json payload: %v") {
		return nil, err
	}

	last := rune(s[len(s)-1])
	switch last {
	case '}':
		s = fmt.Sprintf(`%s,"%s":"%s:%s"%c`, s[0:len(s)-1], signatureField, identity.Id(),
			base64.StdEncoding.EncodeToString(signature), last)
	case ']':
		s = fmt.Sprintf(`%s,"%s:%s"%c`, s[0:len(s)-1], identity.Id(),
			base64.StdEncoding.EncodeToString(signature), last)
	}
	return []byte(s), nil
}

var listRegex = regexp.MustCompile(`,\s*"([\w+=\/]+):([\w+=\/]+)"]$`)

func Unmarshal(data []byte, v any, signatureField string) (id string, err error) {
	var sig []byte
	var loc []int
	data = bytes.TrimRight(data, " ")
	last := data[len(data)-1]
	switch last {
	case '}':
		dictRegex := regexp.MustCompile(fmt.Sprintf(`,\s*"%s"\s*:\s*"([\w+=\/]+):([\w+=\/]+)"`, signatureField))
		loc = dictRegex.FindSubmatchIndex(data)
	case ']':
		loc = listRegex.FindSubmatchIndex(data)
	}
	if len(loc) != 6 {
		return "", fmt.Errorf("no signature field dgst_ed25519_blake2b in data")
	}

	id = string(data[loc[2]:loc[3]])
	signature64 := string(data[loc[4]:loc[5]])
	sig, err = base64.StdEncoding.DecodeString(signature64)
	if core.IsErr(err, "cannot decode signature: %v") {
		return "", err
	}
	data = append(data[0:loc[0]], data[loc[1]:]...)

	err = json.Unmarshal(data, v)
	if core.IsErr(err, "invalid json: %v") {
		return "", err
	}

	hs := QuickHash(data)
	if !Verify(id, hs, sig) {
		core.IsErr(ErrInvalidSignature, "invalid signature %s: %v", id)
		return "", ErrInvalidSignature
	}

	return id, err
}
