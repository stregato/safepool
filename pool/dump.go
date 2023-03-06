package pool

import (
	"fmt"

	"github.com/code-to-go/safepool/core"
)

func (p *Pool) Dump() map[string]any {
	m := map[string]any{}

	m["pool"] = p.Name
	m["primary"] = p.e.String()
	m["masterKeyId"] = p.masterKeyId

	var exchangers []string
	for _, e := range p.exchangers {
		exchangers = append(exchangers, e.String())
	}
	m["exchangers"] = exchangers
	m["accessHash"] = p.accessHash
	m["lastAccessSync"] = p.lastAccessSync
	m["lastAccessSyncElapsed"] = core.Since(p.lastAccessSync)
	m["lastHouseKeeping"] = p.lastHouseKeeping
	m["lastHouseKeepingElapsed"] = core.Since(p.lastHouseKeeping)

	keystore, _ := p.sqlGetKeystore()
	var keys []uint64
	for k := range keystore {
		keys = append(keys, k)
	}
	m["keys"] = keys

	tag := fmt.Sprintf("feeds@%s", p.e.String())
	lastSlot, modTime, _ := sqlGetCheckpoint(p.Name, tag)
	m["lastSlot"] = lastSlot
	m["checkpointModTime"] = modTime

	feeds, _ := p.List(0)
	m["feeds"] = feeds

	identities, _ := p.Users()
	var users []string
	for _, i := range identities {
		if i.Nick == "" {
			users = append(users, i.Id())
		} else {
			users = append(users, i.Nick)
		}
	}
	m["users"] = users

	return m
}
