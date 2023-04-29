package pool

import (
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/code-to-go/safepool/core"
	"github.com/code-to-go/safepool/security"
	"github.com/code-to-go/safepool/storage"
	"github.com/godruoyi/go-snowflake"
)

type State int

const (
	Disabled State = iota
	Active
)

type Access struct {
	UserId string    `json:"userId"`
	State  State     `json:"state"`
	Since  time.Time `json:"since"`
}

type AccessKey struct {
	UserId string    `json:"userId"`
	Value  []byte    `json:"key"`
	Since  time.Time `json:"since"`
}

type AccessFile struct {
	Id          uint64      `json:"id"`
	Version     float32     `json:"version"`
	PoolId      uint64      `json:"poolId"`
	Keys        []AccessKey `json:"keys"`
	Nonce       []byte      `json:"nonce"`
	MasterKeyId uint64      `json:"masterKeyId"`
	Keystore    []byte      `json:"keystore"`
	Apps        []string    `json:"apps"`
}

const identityFolder = "identities"
const accessFolder = "access"
const touchFile = ".touch"

func (p *Pool) SetAccess(userId string, state State) error {
	_, ok, _ := security.GetIdentity(userId)
	if !ok {
		identity, err := security.IdentityFromId(userId)
		if core.IsErr(err, "id '%s' is invalid: %v") {
			return err
		}

		err = security.SetIdentity(identity)
		if core.IsErr(err, "cannot save identity '%s' to db: %v", identity) {
			return err
		}
	}

	err := p.sqlSetAccess(Access{
		UserId: userId,
		State:  state,
		Since:  core.Now(),
	})
	if core.IsErr(err, "cannot link identity '%s' to pool '%s': %v", userId, p.Name) {
		return err
	}

	return err
}

func (p *Pool) ExportSelf(force bool) error {
	return p.exportSelf(p.e, force)
}

func (p *Pool) exportSelf(e storage.Storage, force bool) error {
	name := path.Join(p.Name, identityFolder, p.Self.Id())
	if !force {
		_, err := e.Stat(name)
		if err == nil {
			return nil
		}
	}

	err := p.writeIdentity(e, name, p.Self)
	if core.IsErr(err, "cannot write identity to '%s': %v", name) {
		return err
	}

	p.touchGuard(e, identityFolder, touchFile)
	return nil
}

func (p *Pool) syncIdentities(e storage.Storage) error {
	if p.checkGuard(e, identityFolder, touchFile) {
		return nil
	}

	ls, err := e.ReadDir(path.Join(p.Name, identityFolder), 0)
	if core.IsErr(err, "cannot list files from %s: %v", e) {
		return err
	}

	selfId := p.Self.Id()
	for _, l := range ls {
		n := l.Name()
		if !strings.HasPrefix(n, ".") && n != selfId {
			name := path.Join(p.Name, identityFolder, n)
			i, err := p.readIdentity(e, name)
			if !core.IsErr(err, "cannot read identity from '%s': %v", name) {
				security.SetIdentity(i)
			}
		}
	}
	return nil
}

func (p *Pool) syncSecondaryExchanges(ignoreGuard bool) {
	p.mutex.Lock()
	for _, e := range p.exchangers {
		if e != p.e {
			err := p.syncAccessFor(e, ignoreGuard)
			core.IsErr(err, "cannot synchronize secondary exchange '%s': %v", e.String())
		}
	}
	p.mutex.Unlock()
}

func (p *Pool) SyncAccess(ignoreGuard bool) error {
	err := p.syncAccessFor(p.e, ignoreGuard)
	if core.IsErr(err, "cannot sync privary exchange '%s': %v", p.e.String()) {
		return err
	}
	go p.syncSecondaryExchanges(ignoreGuard)

	return err
}

func (p *Pool) syncAccessFor(e storage.Storage, ignoreGuard bool) error {

	core.Info("sync access for %s", e.String())
	err := p.syncIdentities(e)
	if core.IsErr(err, "cannot sync own identity: %v") {
		return err
	}

	if !ignoreGuard && p.checkGuard(e, accessFolder, touchFile) {
		core.Info("access checkpoint is recent, skip sync")
		return nil
	}

	updates, sources, requireExport, err := p.syncAccessFiles(e)
	if err != nil {
		return err
	}
	core.IsErr(err, "cannot save checkpoint to db: %v")

	p.sqlDelKey(p.BaseId())
	switch len(sources) {
	case 0:
	case 1:
		keyId := sources[0].MasterKeyId
		keyValue := p.keyFunc(keyId)
		p.sqlSetMasterKey(keyId)
		if core.IsErr(err, "cannot set master key for id '%d': %v", keyId) {
			return err
		}
		p.masterKeyId = keyId
		p.masterKey = keyValue
	default:
		err = p.updateMasterKey()
		if core.IsErr(err, "cannot update master key: %v") {
			return err
		}
		requireExport = true
	}

	for _, update := range updates {
		err = p.sqlSetAccess(update)
		if core.IsErr(err, "cannot update access information for user '%s': %v", update.UserId) {
			return err
		}
	}

	if requireExport {
		err = p.exportAccessFile(e)
		if core.IsErr(err, "cannot export access file: %v", e) {
			return err
		}
	}

	return nil
}

func (p *Pool) syncAccessFiles(e storage.Storage) (updates map[string]Access, sources []*AccessFile, requireExport bool, err error) {
	_, accesses, err := p.sqlGetAccesses(false)
	if core.IsErr(err, "cannot read access from db: %v", err) {
		return nil, nil, false, err
	}

	am := map[string]Access{}
	for _, access := range accesses {
		am[access.UserId] = access
	}

	accessFiles, err := e.ReadDir(path.Join(p.Name, accessFolder), 0)
	if !os.IsNotExist(err) && core.IsErr(err, "cannot read access folder in %s:%v", e.String()) {
		return nil, nil, false, err
	}
	sort.Slice(accessFiles, func(i, j int) bool { return accessFiles[i].Name() > accessFiles[j].Name() })

	if len(accessFiles) > 0 {
		p.lastReadAccessFile = accessFiles[0].Name()
	} else {
		requireExport = true
	}
	updates = map[string]Access{}
	for idx, accessFile := range accessFiles {
		name := accessFile.Name()
		if name[0] == '.' {
			continue
		}

		_, af, err := p.readAccessFile(e, name)
		if core.IsErr(err, "cannot read access file %s: %v", name) {
			return nil, nil, false, err
		}

		_, masterkeyValue, err := p.extractMasterKey(af)
		if core.IsErr(err, "cannot extract master key from %s: %v", accessFile.Name()) {
			return nil, nil, false, err
		}
		if masterkeyValue == nil {
			if idx == 0 {
				return nil, nil, false, ErrNotAuthorized
			}
			continue
		}
		_, err = p.decodeKeystore(masterkeyValue, af.Keystore, af.Nonce)
		if core.IsErr(err, "cannot import keystore: %v") {
			return nil, nil, false, err
		}

		updateIns, updateOuts := p.mergeWithFile(&af, am, updates)
		if updateIns > 0 {
			sources = append(sources, &af)
		}
		if updateOuts > 0 {
			requireExport = true
		}
	}

	return updates, sources, requireExport, nil
}

func (p *Pool) exportAccessFile(e storage.Storage) error {
	if !core.TimeIsSync() {
		return ErrNoSyncClock
	}

	if p.masterKeyId == 0 {
		return ErrNotAuthorized
	}

	identities, accesses, err := p.sqlGetAccesses(false)
	if core.IsErr(err, "cannot read identities from db for '%s': %v", p.Name) {
		return err
	}

	var keys []AccessKey
	for idx, access := range accesses {
		var key []byte
		identity := identities[idx]
		if access.State == Active {
			k, err := security.EcEncrypt(identity, p.masterKey)
			if !core.IsErr(err, "cannot encrypt master key for '%s' in '%s': %v", identity.Nick, p.Name) {
				key = k
			}
		}
		keys = append(keys, AccessKey{
			UserId: access.UserId,
			Since:  access.Since,
			Value:  key,
		})
	}

	keystore, nonce, err := p.encodeKeystore()
	if core.IsErr(err, "cannot encode keystore for export of pool '%s': %v", p.Name) {
		return err
	}

	a := AccessFile{
		Id:          snowflake.ID(),
		Version:     1.0,
		PoolId:      p.Id,
		Keys:        keys,
		Nonce:       nonce,
		MasterKeyId: p.masterKeyId,
		Keystore:    keystore,
		Apps:        p.Apps,
	}
	name := fmt.Sprintf("%d", a.Id)
	err = p.writeAccessFile(e, a, name)
	if core.IsErr(err, "cannot write access file '%s': %v", name) {
		return err
	}
	p.touchGuard(e, accessFolder, ".touch")

	accessFolder := path.Join(p.Name, accessFolder)
	accessFiles, err := e.ReadDir(accessFolder, 0)
	if !os.IsNotExist(err) && core.IsErr(err, "cannot read access folder in %s:%v", e.String()) {
		return err
	}
	for _, accessFile := range accessFiles {
		name := accessFile.Name()
		if !strings.HasPrefix(name, ".") && name <= p.lastReadAccessFile {
			e.Delete(path.Join(accessFolder, name))
		}
	}

	return nil
}

func (p *Pool) extractMasterKey(a AccessFile) (masterKeyId uint64, masterKey []byte, err error) {
	selfId := p.Self.Id()
	for _, key := range a.Keys {
		if key.UserId == selfId {
			masterKey, err := security.EcDecrypt(p.Self, key.Value)
			if core.IsErr(err, "cannot derive master key for pool '%s'", p.Name) {
				return 0, nil, err
			}
			err = p.sqlSetKey(a.MasterKeyId, masterKey)
			if core.IsErr(err, "cannot save master key: %v") {
				return 0, nil, err
			}
			return a.MasterKeyId, masterKey, nil
		}
	}
	return 0, nil, nil
}

func (p *Pool) mergeWithFile(af *AccessFile, am map[string]Access, updates map[string]Access) (updateIns, updateOuts int) {
	for _, key := range af.Keys {
		a, isInDb := am[key.UserId]
		if isInDb && key.Since == a.Since {
			continue
		}
		if isInDb && key.Since.Before(a.Since) {
			continue
		}
		a = Access{
			UserId: key.UserId,
			State:  core.If(key.Value == nil, Disabled, Active),
			Since:  key.Since,
		}
		am[key.UserId] = a
		updates[key.UserId] = a
		updateIns++
	}
	return updateIns, len(am) - len(af.Keys)
}

func (p *Pool) updateMasterKey() error {
	keyId := snowflake.ID()
	key := security.GenerateBytesKey(32)
	err := p.sqlSetKey(keyId, key)
	if core.IsErr(err, "cannot store master encryption key to db: %v") {
		return err
	}
	err = p.sqlSetMasterKey(keyId)
	if core.IsErr(err, "cannot set master key for id '%d': %v", keyId) {
		return err
	}
	p.masterKeyId = keyId
	p.masterKey = key
	return nil
}
