package api

import (
	"github.com/code-to-go/safepool/sql"

	"github.com/sirupsen/logrus"
)

func sqlGetConfig(pool string, key string) (s string, i int, b []byte, ok bool) {
	var b64 string
	err := sql.QueryRow("GET_CONFIG", sql.Args{"pool": pool, "key": key}, &s, &i, &b64)
	switch err {
	case sql.ErrNoRows:
		ok = false
	case nil:
		ok = true
		if b64 != "" {
			b = sql.DecodeBase64(b64)
		}
	default:
		logrus.Errorf("cannot get config '%s': %v", key, err)
		ok = false
	}
	return s, i, b, ok
}

func sqlSetConfig(pool string, key string, s string, i int, b []byte) error {
	b64 := sql.EncodeBase64(b)
	_, err := sql.Exec("SET_CONFIG", sql.Args{"pool": pool, "key": key, "s": s, "i": i, "b": b64})
	if err != nil {
		logrus.Errorf("cannot exec 'SET_CONFIG': %v", err)
	}
	return err
}
