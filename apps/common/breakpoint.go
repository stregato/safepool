package common

func SetBreakpoint(pool, app string, ctime int64) error {
	return sqlSetBreakpoint(pool, app, ctime)
}

func GetBreakpoint(pool, app string) int64 {
	return sqlGetBreakpoint(pool, app)
}
