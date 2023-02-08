package pool

import (
	"bytes"
	"hash"
	"math/rand"
	"path"
	"time"

	"github.com/code-to-go/safe/safepool/core"
	"github.com/code-to-go/safe/safepool/security"
	"github.com/code-to-go/safe/safepool/transport"
	"github.com/godruoyi/go-snowflake"
)

type State int

const (
	Disabled State = iota
	Active
)

type Access struct {
	Id      string
	State   State
	ModTime time.Time
}

type AccessKey struct {
	Access Access
	Key    []byte
}

type AccessFile struct {
	Version     float32
	AccessKeys  []AccessKey
	Nonce       []byte
	MasterKeyId uint64
	Keystore    []byte
	Apps        []string
}

const IdentityFolder = "identities"

func (p *Pool) syncIdentities() error {
	security.SetIdentity(p.Self)
	security.Trust(p.Self, true)

	name := path.Join(p.Name, IdentityFolder, p.Self.Id())
	_, err := p.e.Stat(name)
	if err != nil {
		err = p.writeIdentity(name, p.Self)
		if core.IsErr(err, "cannot write identity to '%s': %v", name) {
			return err
		}
	}

	err = p.importIdentities()
	if core.IsErr(err, "cannot import identities: %v") {
		return err
	}

	return nil
}

func (p *Pool) importIdentities() error {
	ls, err := p.e.ReadDir(path.Join(p.Name, IdentityFolder), 0)
	if core.IsErr(err, "cannot list files from %s: %v", p.e) {
		return err
	}

	identities, err := security.Identities()
	if core.IsErr(err, "cannot process identities: %v") {
		return err
	}
	m := map[string]security.Identity{}
	for _, i := range identities {
		m[i.Id()] = i
	}

	selfId := p.Self.Id()
	for _, l := range ls {
		n := l.Name()
		if n == selfId {
			continue
		}

		identity, ok := m[n]
		if !ok || identity.Nick == "❓ Incognito..." || rand.Intn(100) > 95 {
			name := path.Join(p.Name, IdentityFolder, n)
			i, err := p.readIdentity(name)
			if !core.IsErr(err, "cannot read identity from '%s': %v", name) {
				security.SetIdentity(i)
			}
		}
	}
	return nil
}

func (p *Pool) sync(e transport.Exchanger) (hash.Hash, error) {
	l, err := p.lockAccessFile(e)
	if core.IsErr(err, "cannot lock access on %s: %v", p.e) {
		return nil, err
	}
	defer p.unlockAccessFile(e, l)

	a, h, err := p.readAccessFile(e)
	if core.IsErr(err, "cannot read access file:%v") {
		return nil, err
	}
	p.Apps = a.Apps

	if bytes.Equal(h.Sum(nil), p.accessHash) {
		return h, nil
	}

	requireExport, err := p.syncAccesses(a)
	if core.IsErr(err, "cannot sync accesss: %v") {
		return nil, err
	}

	_, err = p.decodeKeystore(a.Keystore, a.Nonce)
	if core.IsErr(err, "cannot import keystore: %v") {
		return nil, err
	}

	p.accessHash = h.Sum(nil)
	if requireExport {
		err = p.exportAccessFile()
		return h, err
	}
	return h, nil
}

func (p *Pool) exportAccessFile() error {
	identities, accesses, err := p.sqlGetAccesses(false)
	if core.IsErr(err, "cannot read identities from db for '%s': %v", p.Name) {
		return err
	}

	var accessKeys []AccessKey
	for idx, access := range accesses {
		var key []byte
		identity := identities[idx]
		if access.State == Active {
			k, err := security.EcEncrypt(identity, p.masterKey)
			if !core.IsErr(err, "cannot encrypt master key for '%s' in '%s': %v", identity.Nick, p.Name) {
				key = k
			}
		}
		accessKeys = append(accessKeys, AccessKey{
			Access: access,
			Key:    key,
		})
	}

	keystore, nonce, err := p.encodeKeystore()
	if core.IsErr(err, "cannot encode keystore for export of pool '%s': %v", p.Name) {
		return err
	}

	a := AccessFile{
		Version:     1.0,
		AccessKeys:  accessKeys,
		Nonce:       nonce,
		MasterKeyId: p.masterKeyId,
		Keystore:    keystore,
		Apps:        p.Apps,
	}
	_, err = p.writeAccessFile(p.e, a)
	if core.IsErr(err, "cannot write access file: %v") {
		return err
	}
	return nil
}

func (p *Pool) syncAccesses(a AccessFile) (requireExport bool, err error) {
	var needNewMasterKey bool
	identities, accesses, err := p.sqlGetAccesses(false)
	if core.IsErr(err, "cannot read identities during grant import: %v", err) {
		return false, err
	}
	amap := map[string]Access{}
	for _, access := range accesses {
		amap[access.Id] = access
	}
	imap := map[string]security.Identity{}
	for _, identity := range identities {
		imap[identity.Id()] = identity
	}

	selfId := p.Self.Id()
	for _, accessKey := range a.AccessKeys {
		if accessKey.Access.Id == selfId {
			masterKey, err := security.EcDecrypt(p.Self, accessKey.Key)
			if core.IsErr(err, "cannot derive master key for pool '%s'", p.Name) {
				return false, err
			}
			p.masterKey = masterKey
			p.masterKeyId = a.MasterKeyId
			err = p.sqlSetKey(a.MasterKeyId, masterKey)
			if core.IsErr(err, "cannot save master key: %v") {
				return false, err
			}
		}

		if accessKey.Key == nil {
			requireExport = true
		}

		access, isInDb := amap[accessKey.Access.Id]
		if !isInDb {
			switch {
			case accessKey.Access.ModTime.After(access.ModTime):
				err = p.sqlSetAccess(accessKey.Access)
				core.IsErr(err, "cannot set access for identity '%s' on pool '%s': %v", access.Id, p.Name)
			case accessKey.Access.ModTime.Before(access.ModTime):
				requireExport = true
				needNewMasterKey = needNewMasterKey || accessKey.Access.State != access.State && access.State == Disabled
			}
		}
		delete(amap, access.Id)
	}

	requireExport = requireExport || len(amap) > 0
	if p.masterKeyId == 0 {
		return false, ErrNotAuthorized
	}

	if needNewMasterKey {
		err = p.updateMasterKey()
		if core.IsErr(err, "cannot update master encryption key for pool '%s': %v", p.Name) {
			return false, err
		}
	}

	return requireExport, nil
}

func (p *Pool) updateMasterKey() error {
	p.masterKeyId = snowflake.ID()
	p.masterKey = security.GenerateBytesKey(32)
	err := p.sqlSetKey(p.masterKeyId, p.masterKey)
	if core.IsErr(err, "çannot store master encryption key to db: %v") {
		return err
	}

	return nil
}
