package pool

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"path"
	"strings"

	"github.com/code-to-go/safepool/core"
	"github.com/code-to-go/safepool/security"
	"github.com/code-to-go/safepool/storage"
	"github.com/godruoyi/go-snowflake"
)

func (p *Pool) Send(name string, r io.ReadSeekCloser, size int64, meta []byte) (Head, error) {
	id := snowflake.ID()
	slot := core.Now().Format(FeedDateFormat)
	n := path.Join(p.Name, FeedsFolder, slot, fmt.Sprintf("%d.body", id))
	h, err := p.writeFile(p.e, n, r, size)
	if core.IsErr(err, "cannot post file %s to %s: %v", name, p.e) {
		return Head{}, err
	}

	hash := h.Sum(nil)
	signature, err := security.Sign(p.Self, hash)
	if core.IsErr(err, "cannot sign file %s.body in %s: %v", name, p.e) {
		return Head{}, err
	}
	f := Head{
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
		return Head{}, err
	}

	hr := core.NewBytesReader(data)
	hn := path.Join(p.Name, FeedsFolder, slot, fmt.Sprintf("%d.head", id))
	_, err = p.writeFile(p.e, hn, hr, int64(len(data)))
	if core.IsErr(err, "cannot write header %s.head in %s: %v", name, p.e) {
		p.e.Delete(n)
		return Head{}, err
	}

	tn := path.Join(p.Name, FeedsFolder, ".touch")
	err = storage.WriteFile(p.e, tn, nil)
	if core.IsErr(err, "cannot set touch file %s in %s: %v", n, p.e) {
		return Head{}, err
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
			tn := path.Join(p.Name, FeedsFolder, ".touch")
			err = storage.WriteFile(e, tn, nil)
			core.IsErr(err, "cannot set checkpoint in %s: %v", p.e)
			core.Info("file '%s' sent to exchange '%s': id '%d', size '%d', hash '%s'", name, e, id,
				size, base64.StdEncoding.EncodeToString(hash))

		}

		r.Close()
	}()

	return f, nil
}

func (p *Pool) Receive(id uint64, rang *storage.Range, w io.Writer) error {
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

func (p *Pool) writeFile(e storage.Storage, name string, r io.ReadSeekCloser, size int64) (hash.Hash, error) {
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

func (p *Pool) readFile(e storage.Storage, name string, rang *storage.Range, w io.Writer) (hash.Hash, error) {
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

func (p *Pool) readAccessFile(e storage.Storage, name string) (id string, accessFile AccessFile, err error) {
	name = path.Join(p.Name, accessFolder, name)
	data, err := storage.ReadFile(e, name)
	if core.IsErr(err, "cannot read access file: %s", err) {
		return "", AccessFile{}, err
	}

	id, err = security.Unmarshal(data, &accessFile, security.SignatureField)
	if core.IsErr(err, "invalid access file %s: %v", name) {
		return "", AccessFile{}, err
	}

	return id, accessFile, nil
}

func (p *Pool) writeAccessFile(e storage.Storage, a AccessFile, name string) error {
	filePath := path.Join(p.Name, accessFolder, name)

	data, err := security.Marshal(p.Self, a, security.SignatureField)
	if core.IsErr(err, "cannot marshal access file on %s: %v", p.Name, err) {
		return err
	}
	err = storage.WriteFile(e, filePath, data)
	if core.IsErr(err, "cannot write access file on %s: %v", p.Name, err) {
		return err
	}

	return nil
}

func (p *Pool) writeIdentity(e storage.Storage, name string, identity security.Identity) error {
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

func (p *Pool) readIdentity(e storage.Storage, name string) (security.Identity, error) {
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
