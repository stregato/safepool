package pool

import (
	"database/sql"

	"github.com/code-to-go/safepool/core"
	"github.com/code-to-go/safepool/security"
)

const defaultLifeSpanHours = 30 * 24

var defaultApps = []string{"chat", "invite", "library", "private"}

// Init initialized a domain on the specified exchangers
func Open(self security.Identity, name string) (*Pool, error) {
	config, err := sqlGetPool(name)
	if core.IsErr(err, "unknown pool %s: %v", name) {
		return nil, err
	}

	p := &Pool{
		Name:          name,
		Self:          self,
		Apps:          config.Apps,
		LifeSpanHours: config.LifeSpanHours,

		lastAccessSync: core.Now(),
		lastReplica:    core.Now(),
	}
	if p.LifeSpanHours < 1 {
		p.LifeSpanHours = defaultLifeSpanHours
	}
	if len(p.Apps) == 0 {
		p.Apps = defaultApps
	}

	masterKeyId, masterKey, err := p.sqlGetMasterKey()
	if err == nil {
		p.masterKeyId = masterKeyId
		p.masterKey = masterKey
	}
	if err != sql.ErrNoRows && core.IsErr(err, "missing master key in pool %s: %v", name) {
		return nil, err
	}

	err = p.connectSafe(config)
	if err != nil {
		return nil, err
	}
	p.ExportSelf(false)

	err = p.SyncAccess(false)
	if core.IsErr(err, "cannot sync access: %v") {
		return nil, err
	}

	if p.masterKeyId == 0 || p.masterKey == nil {
		return nil, ErrNotAuthorized
	}

	p.startReplica()
	return p, nil
}
