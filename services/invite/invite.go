package invite

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"strings"

	"github.com/code-to-go/safepool/apps/common"
	"github.com/code-to-go/safepool/core"
	"github.com/code-to-go/safepool/pool"
	"github.com/code-to-go/safepool/security"
	"github.com/godruoyi/go-snowflake"
)

var ErrInvalidToken = errors.New("provided token is invalid: missing name or configs")

type Invite struct {
	Subject      string            `json:"subject"`
	Sender       security.Identity `json:"sender"`
	RecipientIds []string          `json:"recipientIds"`
	Name         string            `json:"name"`
	Exchanges    []string          `json:"exchanges"`
}

func (i Invite) Join() error {
	if i.Exchanges == nil {
		return core.ErrNotAuthorized
	}

	if i.Name == "" {
		core.IsErr(ErrInvalidToken, "invalid empty name in invite: %v")
		return ErrInvalidToken
	} else {
		core.Info("valid token for pool '%s'", i.Name)
	}

	err := security.SetIdentity(i.Sender)
	if core.IsErr(err, "cannot save identity '%s': %v", i.Sender.Nick) {
		return err
	}

	err = security.Trust(i.Sender, true)
	if core.IsErr(err, "cannot trust identity '%s': %v", i.Sender.Nick) {
		return err
	}

	return pool.Define(pool.Config{Name: i.Name, Public: i.Exchanges})
}

func Add(p *pool.Pool, i Invite) error {
	bs, err := json.Marshal(i)
	if core.IsErr(err, "cannot marshal invite: %v") {
		return err
	}
	name := fmt.Sprintf("invite/%d", snowflake.ID())
	_, err = p.Send(name, core.NewBytesReader(bs), int64(len(bs)), nil)
	core.IsErr(err, "cannot send invite to pool '%s': %v", p.Name)
	return err
}

func Receive(p *pool.Pool, after int64, onlyMine bool) ([]Invite, error) {
	p.Sync()
	ctime := common.GetBreakpoint(p.Name, "invite")
	fs, _ := p.List(ctime)
	for _, f := range fs {
		accept(p, f)
		ctime = f.CTime
	}
	common.SetBreakpoint(p.Name, "invite", ctime)
	return sqlGetInvites(p.Name, after, onlyMine)
}

func accept(p *pool.Pool, f pool.Head) {
	if !strings.HasPrefix(f.Name, "invite/") {
		return
	}

	var buf bytes.Buffer
	err := p.Receive(f.Id, nil, &buf)
	if core.IsErr(err, "cannot retrieve invite from '%s/%d': %v", p.Name, f.Id) {
		return
	}

	var i Invite
	err = json.Unmarshal(buf.Bytes(), &i)
	if core.IsErr(err, "cannot unmarshal invite: %v") {
		return
	}

	err = sqlSetInvite(p.Name, f.CTime, i)
	core.IsErr(err, "cannot save document to db: %v")
}

type Token struct {
	Version    float32  `json:"v"`
	Subject    string   `json:"s"`
	SenderNick string   `json:"d"`
	Name       string   `json:"n"`
	Crc        uint32   `json:"c"`
	Keys       [][]byte `json:"k"`
	Storages   []byte   `json:"t"`
}

const tokenVersion = 1.0

func Decode(self security.Identity, token string) (Invite, error) {
	token = strings.ReplaceAll(token, "_", "/")
	data, _ := base64.StdEncoding.DecodeString(token)
	r, err := gzip.NewReader(bytes.NewReader(data))
	if core.IsErr(err, "invalid token, cannot gunzip: %v") {
		return Invite{}, err
	}

	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	if core.IsErr(err, "invalid token, cannot gunzip: %v") {
		return Invite{}, err
	}
	r.Close()

	var t Token
	senderId, err := security.Unmarshal(buf.Bytes(), &t, "g")
	if core.IsErr(err, "invalid token, cannot unmarshal: %v") {
		return Invite{}, err
	}

	sender, err := security.IdentityFromId(senderId)
	if core.IsErr(err, "invalid sender id: %v") {
		return Invite{}, err
	}
	sender.Nick = t.SenderNick
	i := Invite{
		Subject: t.Subject,
		Sender:  sender,
		Name:    t.Name,
	}

	if len(t.Keys) > 0 {
		noonce := []byte(senderId)[0:aes.BlockSize]
		for _, key := range t.Keys {
			key, err = security.EcDecrypt(self, key)
			if err != nil {
				continue
			}

			decrypted, err := security.DecryptBlock(key, noonce, t.Storages)
			if err != nil {
				continue
			}
			if crc32.Checksum(decrypted, crc32.IEEETable) != uint32(t.Crc) {
				continue
			}
			t.Storages = decrypted
			goto proceed
		}
		return i, nil
	}
proceed:
	err = json.Unmarshal(t.Storages, &i.Exchanges)
	if core.IsErr(err, "cannot unmarshal storages in token: %v") {
		return Invite{}, nil
	}
	return i, nil
}

func Encode(i Invite) (string, error) {
	var keys [][]byte

	masterKey := security.GenerateBytesKey(32)
	for _, id := range i.RecipientIds {
		identity, err := security.IdentityFromId(id)
		if err == nil {
			key, err := security.EcEncrypt(identity, masterKey)
			if err == nil {
				keys = append(keys, key)
			}
		}
	}
	storages, err := json.Marshal(i.Exchanges)
	if core.IsErr(err, "cannot marshal storages in invite: %v") {
		return "", err
	}
	crc := crc32.Checksum(storages, crc32.IEEETable)

	if len(keys) > 0 {
		noonce := []byte(i.Sender.Id())[0:aes.BlockSize]
		storages, err = security.EncryptBlock(masterKey, noonce, storages)
		if core.IsErr(err, "cannot encrypt token: %v") {
			return "", err
		}
	}
	t := Token{
		Version:    tokenVersion,
		Subject:    i.Subject,
		SenderNick: i.Sender.Nick,
		Name:       i.Name,
		Crc:        crc,
		Keys:       keys,
		Storages:   storages,
	}

	data, err := security.Marshal(i.Sender, &t, "g")

	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	io.Copy(w, bytes.NewReader(data))
	w.Close()

	return strings.ReplaceAll(base64.StdEncoding.EncodeToString(buf.Bytes()), "/", "_"), err
}
