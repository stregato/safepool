package pool

import (
	"bytes"
	"hash"
	"io"
	"path"
	"time"

	"github.com/code-to-go/safe/safepool/core"
	"github.com/code-to-go/safe/safepool/security"
	"github.com/code-to-go/safe/safepool/transport"
	"gopkg.in/yaml.v3"
)

func (p *Pool) writeFile(name string, r io.Reader) (*security.HashStream, error) {
	hr, err := security.NewHashStream(r, nil)
	if core.IsErr(err, "cannot create hash reader: %v") {
		return nil, err
	}

	er, err := security.EncryptingReader(p.masterKeyId, p.keyFunc, hr)
	if core.IsErr(err, "cannot create encrypting reader: %v") {
		return nil, err
	}

	err = p.e.Write(name, er)
	return hr, err
}

func (p *Pool) readFile(name string, rang *transport.Range, w io.Writer) (*security.HashStream, error) {
	hw, err := security.NewHashStream(nil, w)
	if core.IsErr(err, "cannot create hash stream: %v") {
		return nil, err
	}

	ew, err := security.DecryptingWriter(p.keyFunc, hw)
	if core.IsErr(err, "cannot create decrypting writer: %v") {
		return nil, err
	}
	err = p.e.Read(name, rang, ew)
	return hw, err
}

func (p *Pool) readAccessFile(e transport.Exchanger) (AccessFile, hash.Hash, error) {
	var a AccessFile
	var sh security.SignedHash
	signatureFile := path.Join(p.Name, ".access.sign")
	accessFile := path.Join(p.Name, ".access")

	err := transport.ReadJSON(e, signatureFile, &sh, nil)
	if core.IsErr(err, "cannot read signature file '%s': %v", signatureFile, err) {
		return AccessFile{}, nil, err
	}

	h := security.NewHash()
	err = transport.ReadJSON(e, accessFile, &a, h)
	if core.IsErr(err, "cannot read access file: %s", err) {
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

func (p *Pool) writeIdentity(name string, identity security.Identity) error {
	data, err := yaml.Marshal(p.Self.Public())
	if core.IsErr(err, "cannot marshal identity: %v") {
		return err
	}

	sign, err := security.Sign(p.Self, data)
	if core.IsErr(err, "cannot sign identity: %v") {
		return err
	}

	data = append(data, 0)
	data = append(data, sign...)
	err = p.e.Write(name, bytes.NewBuffer(data))
	core.IsErr(err, "cannot write '%s': %v", name)
	return err
}

func (p *Pool) readIdentity(name string) (security.Identity, error) {
	var identity security.Identity
	var buf bytes.Buffer

	err := p.e.Read(name, nil, &buf)
	if core.IsErr(err, "cannot read file %s: %v", name) {
		return identity, err
	}

	parts := bytes.Split(buf.Bytes(), []byte{0})
	if len(parts) != 2 {
		core.IsErr(ErrInvalidSignature, "identity file '%s' has no null separator: %v", name)
		return identity, ErrInvalidSignature
	}

	err = yaml.Unmarshal(parts[0], &identity)
	if core.IsErr(err, "invalid yaml format in '%s': %v", name) {
		return identity, err
	}

	if !security.Verify(identity.Id(), parts[0], parts[1]) {
		core.IsErr(ErrInvalidSignature, "invalide signature in '%s': %v", name)
		return identity, ErrInvalidSignature
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
