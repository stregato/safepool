package pool

import (
	"github.com/code-to-go/safepool/core"
)

func Define(c Config) error {
	return sqlSetPool(c.Name, c)
}

func GetConfig(name string) (Config, error) {
	c, err := sqlGetPool(name)
	if core.IsErr(err, "cannot load config for pool '%s'", name) {
		return Config{}, err
	}
	return c, nil
}
