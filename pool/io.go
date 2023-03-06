package pool

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/code-to-go/safepool/core"
	"github.com/code-to-go/safepool/security"
	"github.com/code-to-go/safepool/transport"
	"github.com/godruoyi/go-snowflake"
)

func (p *Pool) Send(name string, r io.ReadSeekCloser, size int64, meta []byte) (Feed, error) {
	id := snowflake.ID()
	slot := core.Now().Format(FeedDateFormat)
	n := path.Join(p.Name, FeedsFolder, slot, fmt.Sprintf("%d.body", id))
	h, err := p.writeFile(p.e, n, r, size)
	if core.IsErr(err, "cannot post file %s to %s: %v", name, p.e) {
		return Feed{}, err
	}

	hash := h.Sum(nil)
	signature, err := security.Sign(p.Self, hash)
	if core.IsErr(err, "cannot sign file %s.body in %s: %v", name, p.e) {
		return Feed{}, err
	}
	f := Feed{
		Id:        id,
		Name:      name,
		Size:      size,
		Hash:      hash,
		ModTime:   core.Now(),
		AuthorId:  p.Self.Id(),
		Signature: signature,
		Meta:      meta,
		Slot:      slot,
		CTime:     core.Now().Unix(),
	}
	data, err := json.Marshal(f)
	if core.IsErr(err, "cannot marshal header to json: %v") {
		return Feed{}, err
	}

	hr := core.NewBytesReader(data)
	hn := path.Join(p.Name, FeedsFolder, slot, fmt.Sprintf("%d.head", id))
	_, err = p.writeFile(p.e, hn, hr, int64(len(data)))
	if core.IsErr(err, "cannot write header %s.head in %s: %v", name, p.e) {
		p.e.Delete(n)
		return Feed{}, err
	}

	_, err = p.e.SetCheckpoint(path.Join(p.Name, FeedsFolder, ".touch"))
	if core.IsErr(err, "cannot set checkpoint in %s: %v", p.e) {
		return Feed{}, err
	}

	core.Info("file '%s' sent to exchange '%s': id '%d', size '%d', hash '%s'", name, p.e, id,
		size, base64.StdEncoding.EncodeToString(hash))

	go func() {
		p.mutex.Lock()
		defer p.mutex.Unlock()

		for _, e := range p.exchangers {
			if e == p.e {
				continue
			}

			r.Seek(0, 0)
			hr.Seek(0, 0)

			_, err = p.writeFile(e, n, r, size)
			if core.IsErr(err, "cannot send %s to secondary exchange %e: %v", n, e) {
				continue
			}
			_, err = p.writeFile(e, hn, hr, int64(len(data)))
			if core.IsErr(err, "cannot send %s to secondary exchange %e: %v", hn, e) {
				continue
			}
			_, err := e.SetCheckpoint(path.Join(p.Name, FeedsFolder, ".touch"))
			core.IsErr(err, "cannot set checkpoint in %s: %v", p.e)
			core.Info("file '%s' sent to exchange '%s': id '%d', size '%d', hash '%s'", name, e, id,
				size, base64.StdEncoding.EncodeToString(hash))

		}

		r.Close()
	}()

	return f, nil
}

func (p *Pool) Receive(id uint64, rang *transport.Range, w io.Writer) error {
	f, err := sqlGetFeed(p.Name, id)
	if core.IsErr(err, "cannot retrieve %d from pool %v: %v", id, p) {
		return err
	}

	bodyName := path.Join(p.Name, FeedsFolder, f.Slot, fmt.Sprintf("%d.body", id))
	cached, err := p.getFromCache(bodyName, rang, w)
	if cached {
		return err
	}
	cw, err := p.cacheWriter(bodyName, w)
	if err == nil {
		defer cw.Close()
		w = cw
	}

	hr, err := p.readFile(p.e, bodyName, rang, w)
	if core.IsErr(err, "cannot read body '%s': %v", bodyName) {
		return err
	}
	hash := hr.Sum(nil)
	if !bytes.Equal(hash, f.Hash) {
		core.IsErr(security.ErrInvalidSignature, "mismatch between declared hash '%s' and actual hash '%s' in '%s'", f.Hash, hash, bodyName)
		return security.ErrInvalidSignature
	}

	core.Info("received file with id %d from pool '%s', hash '%s'", id, p.Name, base64.StdEncoding.EncodeToString(hash))
	return nil
}

func (p *Pool) writeFile(e transport.Exchanger, name string, r io.ReadSeekCloser, size int64) (hash.Hash, error) {
	hr, err := security.NewHashReader(r)
	if core.IsErr(err, "cannot create hash reader: %v") {
		return nil, err
	}

	er, err := security.EncryptingReader(p.masterKeyId, p.keyFunc, hr)
	if core.IsErr(err, "cannot create encrypting reader: %v") {
		return nil, err
	}

	err = e.Write(name, er, size+security.AESHeaderSize, nil)
	return hr.Hash, err
}

func (p *Pool) readFile(e transport.Exchanger, name string, rang *transport.Range, w io.Writer) (hash.Hash, error) {
	hw, err := security.NewHashWriter(w)
	if core.IsErr(err, "cannot create hash stream: %v") {
		return nil, err
	}

	ew, err := security.DecryptingWriter(p.keyFunc, hw)
	if core.IsErr(err, "cannot create decrypting writer: %v") {
		return nil, err
	}
	err = e.Read(name, rang, ew, nil)
	return hw.Hash, err
}

func (p *Pool) readAccessFile(e transport.Exchanger, name string) (AccessFile, hash.Hash, error) {
	var a AccessFile
	var sh security.SignedHash
	signatureFile := path.Join(p.Name, fmt.Sprintf("%s.sign", name))
	accessFile := path.Join(p.Name, name)

	err := transport.ReadJSON(e, signatureFile, &sh, nil)
	if os.IsNotExist(err) || core.IsErr(err, "cannot read signature file '%s': %v", signatureFile, err) {
		return AccessFile{}, nil, err
	}

	h := security.NewHash()
	err = transport.ReadJSON(e, accessFile, &a, h)
	if os.IsNotExist(err) || core.IsErr(err, "cannot read access file: %s", err) {
		return AccessFile{}, nil, err
	}

	trusted, err := security.Trusted()
	if core.IsErr(err, "cannot get trusted identities: %v") {
		return AccessFile{}, nil, nil
	}

	if security.VerifySignedHash(sh, []security.Identity{p.Self}, h.Sum(nil)) {
		p.Trusted = true
		return a, h, nil
	}

	if security.VerifySignedHash(sh, trusted, h.Sum(nil)) {
		_ = security.AppendToSignedHash(sh, p.Self)
		if !core.IsErr(err, "cannot lock access on %s: %v", p.Name, err) {
			if security.AppendToSignedHash(sh, p.Self) == nil {
				err = transport.WriteJSON(e, signatureFile, sh, nil)
				core.IsErr(err, "cannot write signature file on %s: %v", p.Name, err)
			}
		}
		p.Trusted = true
	}

	return a, h, nil
}

func (p *Pool) writeAccessFile(e transport.Exchanger, a AccessFile) (hash.Hash, error) {
	lockFile := path.Join(p.Name, ".access.lock")
	signatureFile := path.Join(p.Name, ".access.sign")
	accessFile := path.Join(p.Name, ".access")

	lockId, err := transport.LockFile(e, lockFile, time.Minute)
	if core.IsErr(err, "cannot lock access on %s: %v", p.Name, err) {
		return nil, err
	}
	defer transport.UnlockFile(e, lockFile, lockId)

	h := security.NewHash()
	err = transport.WriteJSON(e, accessFile, a, h)
	if core.IsErr(err, "cannot write access file on %s: %v", p.Name, err) {
		return nil, err
	}

	sh, err := security.NewSignedHash(h.Sum(nil), p.Self)
	if core.IsErr(err, "cannot generate signature hash on %s: %v", p.Name, err) {
		return nil, err
	}
	err = transport.WriteJSON(e, signatureFile, sh, nil)
	if core.IsErr(err, "cannot write signature file on %s: %v", p.Name, err) {
		return nil, err
	}

	return h, nil
}

func (p *Pool) writeIdentity(e transport.Exchanger, name string, identity security.Identity) error {
	data, err := json.Marshal(p.Self.Public())
	if core.IsErr(err, "cannot marshal identity: %v") {
		return err
	}

	sign, err := security.Sign(p.Self, data)
	if core.IsErr(err, "cannot sign identity: %v") {
		return err
	}

	s := fmt.Sprintf("%s:%s", base64.StdEncoding.EncodeToString(data), base64.StdEncoding.EncodeToString(sign))
	r := core.NewStringReader(s)
	err = e.Write(name, r, int64(len(s)), nil)
	core.IsErr(err, "cannot write '%s': %v", name)
	return err
}

func (p *Pool) readIdentity(e transport.Exchanger, name string) (security.Identity, error) {
	var identity security.Identity
	var buf bytes.Buffer

	err := e.Read(name, nil, &buf, nil)
	if core.IsErr(err, "cannot read file %s: %v", name) {
		return identity, err
	}

	parts := strings.Split(buf.String(), ":")
	if len(parts) != 2 {
		core.IsErr(security.ErrInvalidSignature, "identity file '%s' has no : separator: %v", name)
		return identity, security.ErrInvalidSignature
	}

	data, _ := base64.StdEncoding.DecodeString(parts[0])
	err = json.Unmarshal(data, &identity)
	if core.IsErr(err, "invalid yaml format in '%s': %v", name) {
		return identity, err
	}

	sig, _ := base64.StdEncoding.DecodeString(parts[1])
	if !security.Verify(identity.Id(), data, sig) {
		core.IsErr(security.ErrInvalidSignature, "invalide signature in '%s': %v", name)
		return identity, security.ErrInvalidSignature
	}
	return identity, nil
}

func (p *Pool) lockAccessFile(e transport.Exchanger) (uint64, error) {
	lockFile := path.Join(p.Name, ".access.lock")
	lockId, err := transport.LockFile(e, lockFile, time.Minute)
	core.IsErr(err, "cannot lock access on %s: %v", p.Name, err)
	return lockId, err
}

func (p *Pool) unlockAccessFile(e transport.Exchanger, lockId uint64) {
	lockFile := path.Join(p.Name, ".access.lock")
	transport.UnlockFile(e, lockFile, lockId)
}
