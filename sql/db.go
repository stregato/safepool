package sql

import (
	"github.com/code-to-go/safepool/core"
)

func GetConfig(node string, key string) (s string, i int64, b []byte, ok bool) {
	var b64 string
	err := QueryRow("GET_CONFIG", Args{"node": node, "key": key}, &s, &i, &b64)
	switch err {
	case ErrNoRows:
		ok = false
	case nil:
		ok = true
		if b64 != "" {
			b = DecodeBase64(b64)
		}
	default:
		core.IsErr(err, "cannot get config for %s/%s: %v", node, key)
		ok = false
	}
	return s, i, b, ok
}

func SetConfig(node string, key string, s string, i int64, b []byte) error {
	b64 := EncodeBase64(b)
	_, err := Exec("SET_CONFIG", Args{"node": node, "key": key, "s": s, "i": i, "b": b64})
	core.IsErr(err, "cannot set config %s/%s with values %s, %d, %v: %v", node, key, s, i, b)
	return err
}

func DelConfigs(node string) error {
	_, err := Exec("DEL_CONFIG", Args{"node": node})
	core.IsErr(err, "cannot del configs %s", node)
	return err
}
