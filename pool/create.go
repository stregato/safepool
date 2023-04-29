package pool

import (
	"os"
	"path"
	"sync"

	"github.com/code-to-go/safepool/core"
	"github.com/code-to-go/safepool/security"
	"github.com/code-to-go/safepool/storage"
	"github.com/godruoyi/go-snowflake"
)

var Connections = map[string]storage.Storage{}
var ConnectionsMutex = &sync.Mutex{}

// Create creates a new pool on the defined exchanges
func Create(self security.Identity, name string, apps []string) (*Pool, error) {
	config, err := sqlGetPool(name)
	if core.IsErr(err, "unknown pool %s: %v", name) {
		return nil, err
	}

	p := &Pool{
		Name:           name,
		Id:             snowflake.ID(),
		Self:           self,
		LifeSpanHours:  core.If(config.LifeSpanHours > 0, config.LifeSpanHours, 24*30),
		lastAccessSync: core.Now(),
		lastReplica:    core.Now(),
	}
	security.Trust(p.Self, true)

	err = p.connectSafe(config)
	if err != nil {
		return nil, err
	}
	p.ExportSelf(true)

	err = p.checkExisting()
	if err != nil {
		return nil, err
	}

	err = p.updateMasterKey()
	if core.IsErr(err, "cannot generate master encryption key: %v") {
		return nil, err
	}

	access := Access{
		UserId: self.Id(),
		State:  Active,
		Since:  core.Now(),
	}
	err = p.sqlSetAccess(access)
	if core.IsErr(err, "cannot link identity to pool '%s': %v", p.Name) {
		return nil, err
	}

	err = p.SyncAccess(true)
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
