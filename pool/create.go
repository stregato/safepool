package pool

import (
	"os"
	"path"
	"sync"

	"github.com/code-to-go/safepool/core"
	"github.com/code-to-go/safepool/security"
	"github.com/code-to-go/safepool/transport"
	"github.com/godruoyi/go-snowflake"
)

var Connections = map[string]transport.Exchanger{}
var ConnectionsMutex = &sync.Mutex{}

// Create creates a new pool on the defined exchanges
func Create(self security.Identity, name string, apps []string) (*Pool, error) {
	config, err := sqlGetPool(name)
	if core.IsErr(err, "unknown pool %s: %v", name) {
		return nil, err
	}

	p := &Pool{
		Name:             name,
		Id:               snowflake.ID(),
		Self:             self,
		lastAccessSync:   core.Now(),
		lastHouseKeeping: core.Now(),
		config:           config,
	}
	err = p.connectSafe(config)
	if err != nil {
		return nil, err
	}

	err = p.checkExisting()
	if err != nil {
		return nil, err
	}

	p.masterKeyId = snowflake.ID()
	p.masterKey = security.GenerateBytesKey(32)
	err = p.sqlSetKey(p.masterKeyId, p.masterKey)
	if core.IsErr(err, "çannot store master encryption key to db: %v") {
		return nil, err
	}

	access := Access{
		Id:      self.Id(),
		State:   Active,
		ModTime: core.Now(),
	}
	err = p.sqlSetAccess(access)
	if core.IsErr(err, "cannot link identity to pool '%s': %v", p.Name) {
		return nil, err
	}

	err = p.syncAccess()
	if core.IsErr(err, "cannot sync access: %v") {
		return nil, err
	}
	return p, err
}

const pingName = ".reserved.ping.%d.test"

func (p *Pool) checkExisting() error {
	for _, e := range p.exchangers {
		_, err := e.Stat(path.Join(p.Name, ".access"))
		if os.IsNotExist(err) {
			continue
		}

		if ForceCreation {
			err = e.Delete(p.Name)
			if core.IsErr(err, "cannot delete %s: %v", p.Name) {
				return err
			}
		} else {
			core.IsErr(err, "pool already exist in %s or other issue: %v", e.String())
			return err
		}
	}
	return nil
}
