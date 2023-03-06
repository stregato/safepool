package pool

import (
	"github.com/code-to-go/safepool/core"
	"github.com/code-to-go/safepool/security"
)

// Init initialized a domain on the specified exchangers
func Open(self security.Identity, name string) (*Pool, error) {
	config, err := sqlGetPool(name)
	if core.IsErr(err, "unknown pool %s: %v", name) {
		return nil, err
	}
	p := &Pool{
		Name:             name,
		Self:             self,
		config:           config,
		lastAccessSync:   core.Now(),
		lastHouseKeeping: core.Now(),
	}
	err = p.connectSafe(config)
	if err != nil {
		return nil, err
	}

	err = p.syncAccess()
	return p, err
}
