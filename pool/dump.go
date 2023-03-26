package pool

import (
	"fmt"

	"github.com/code-to-go/safepool/core"
	"github.com/code-to-go/safepool/sql"
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
	m["lastAccessSync"] = p.lastAccessSync
	m["lastAccessSyncElapsed"] = core.Since(p.lastAccessSync)
	m["lastHouseKeeping"] = p.lastReplica
	m["lastHouseKeepingElapsed"] = core.Since(p.lastReplica)

	keystore, _ := p.sqlGetKeystore()
	var keys []uint64
	for k := range keystore {
		keys = append(keys, k)
	}
	m["keys"] = keys

	configNode := fmt.Sprintf("pool/%s", p.Name)
	checkpointKey := fmt.Sprintf("checkpoints/%s", p.e.String())
	slotKey := fmt.Sprintf("slots/%s", p.e.String())
	lastSlot, _, _, _ := sql.GetConfig(configNode, slotKey)
	m["lastSlot"] = lastSlot
	_, lastCheckpoint, _, _ := sql.GetConfig(configNode, checkpointKey)
	m["checkpointModTime"] = lastCheckpoint

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
