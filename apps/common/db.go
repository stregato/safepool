package common

import (
	"github.com/code-to-go/safepool/core"
	"github.com/code-to-go/safepool/sql"
)

func sqlSetBreakpoint(pool string, app string, ctime int64) error {
	_, err := sql.Exec("SET_BREAKPOINT", sql.Args{"pool": pool, "app": app,
		"ctime": ctime})
	if core.IsErr(err, "cannot set breakpoint '%s/%s' on db: %v", pool, app) {
		return err
	}
	return err
}

func sqlGetBreakpoint(pool string, app string) int64 {
	var ctime int64
	err := sql.QueryRow("GET_BREAKPOINT", sql.Args{"pool": pool, "app": app}, &ctime)
	if err == nil {
		return ctime
	} else {
		return -1
	}
}
