package chat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/code-to-go/safepool/core"
	pool "github.com/code-to-go/safepool/pool"
	"github.com/code-to-go/safepool/security"
	"github.com/code-to-go/safepool/services/common"
	"github.com/godruoyi/go-snowflake"
	"github.com/sirupsen/logrus"
)

type Message struct {
	Id          uint64    `json:"id,string"`
	Author      string    `json:"author"`
	Time        time.Time `json:"time"`
	ContentType string    `json:"contentType"`
	Text        string    `json:"text"`
	Binary      []byte    `json:"binary"`
	Preview     []byte    `json:"preview"`
	Signature   []byte    `json:"signature"`
}

func getHash(m *Message) []byte {
	h := security.NewHash()
	h.Write([]byte(m.Text))
	h.Write([]byte(m.ContentType))
	h.Write([]byte(m.Author))
	h.Write(m.Binary)
	return h.Sum(nil)
}

// Private is a list of the user ids that have exclusive access to the chat
type Private []string

type Chat struct {
	Pool *pool.Pool
	Name string
}

func Get(p *pool.Pool, name string) Chat {
	return Chat{
		Pool: p,
		Name: name,
	}
}

func getPrivateId(private Private) string {
	if len(private) == 0 {
		return ""
	}

	sort.Strings(private)

	var filtered Private
	last := ""
	for _, p := range private {
		if p != last {
			filtered = append(filtered, p)
			last = p
		}
	}
	return strings.Join(filtered, ":")
}

func (c *Chat) Privates() ([]Private, error) {
	return sqlListPrivates(c.Pool.Name, c.Name)
}

func (c *Chat) SendMessage(contentType string, text string, binary []byte, private Private) (uint64, error) {
	m := Message{
		Id:          snowflake.ID(),
		Author:      c.Pool.Self.Id(),
		Time:        core.Now(),
		ContentType: contentType,
		Text:        text,
		Binary:      binary,
	}
	h := getHash(&m)
	signature, err := security.Sign(c.Pool.Self, h)
	if core.IsErr(err, "cannot sign chat message: %v") {
		return 0, err
	}
	m.Signature = signature

	data, err := json.Marshal(m)
	if core.IsErr(err, "cannot sign chat message: %v") {
		return 0, err
	}

	meta, data, err := c.protect(data, private)
	if core.IsErr(err, "cannot protect chat message: %v") {
		return 0, nil
	}

	go func() {
		name := fmt.Sprintf("%s/%d.chat", c.Name, m.Id)
		_, err = c.Pool.Send(name, core.NewBytesReader(data), int64(len(data)), meta)
		core.IsErr(err, "cannot write chat message: %v")
	}()

	core.Info("added chat message with id %d", m.Id)
	return m.Id, nil
}

func (c *Chat) protect(data []byte, userIds []string) (meta []byte, data2 []byte, err error) {
	if len(userIds) == 0 {
		return nil, data, nil
	}
	if len(userIds) > 8 {
		return nil, nil, fmt.Errorf("private chats in %s can have at most 8 members", c.Pool.Name)
	}

	master := security.GenerateBytesKey(16)
	keys := map[string][]byte{}
	for _, userId := range userIds {
		identity, err := security.IdentityFromId(userId)
		if core.IsErr(err, "invalid user id '%s': %v", userId) {
			return nil, nil, err
		}
		k, err := security.EcEncrypt(identity, master)
		if core.IsErr(err, "cannot encrypt master for user '%s' in chat %s/%s: %v", userId, c.Pool.Name, c.Name) {
			return nil, nil, err
		}
		keys[userId] = k
	}

	meta, err = json.Marshal(keys)
	if core.IsErr(err, "cannot marshal privat keys in chat %s/%s: %v", c.Pool.Name, c.Name) {
		return nil, nil, err
	}

	data, err = security.EncryptBlock(master, master, data)
	if core.IsErr(err, "cannot encrypt message in chat %s/%s: %v", c.Pool.Name, c.Name) {
		return nil, nil, err
	}
	return meta, data, nil
}

func (c *Chat) Receive(after, before time.Time, limit int, private Private) ([]Message, error) {
	c.Pool.Sync()
	ctime := common.GetBreakpoint(c.Pool.Name, c.Name)
	fs, err := c.Pool.List(ctime)
	if core.IsErr(err, "cannot retrieve messages from pool: %v") {
		return nil, err
	}
	for _, f := range fs {
		c.accept(f)
		ctime = f.CTime
	}
	common.SetBreakpoint(c.Pool.Name, c.Name, ctime)
	privateId := getPrivateId(private)

	messages, err := sqlGetMessages(c.Pool.Name, c.Name, after, before, privateId, limit)
	if core.IsErr(err, "cannot read messages from db: %v") {
		return nil, err
	}

	sort.Slice(messages, func(i, j int) bool {
		return messages[i].Time.Before(messages[j].Time)
	})
	return messages, nil
}

// Reset removes all the local content
func (c *Chat) Reset() error {
	return sqlReset(c.Pool.Name)
}

func (c *Chat) accept(h pool.Head) {
	name := h.Name
	if !strings.HasPrefix(name, c.Name) || !strings.HasSuffix(name, ".chat") || h.Size > 10*1024*1024 {
		return
	}
	name = path.Base(name)
	id, err := strconv.ParseInt(name[0:len(name)-5], 10, 64)
	if err != nil {
		return
	}

	var master []byte
	var privateId string
	if h.Meta != nil {
		var keys map[string][]byte
		err = json.Unmarshal(h.Meta, &keys)
		if core.IsErr(err, "cannot unmarshal meta in %s %s: %v", c.Pool, h.Id) {
			return
		}
		k, ok := keys[c.Pool.Self.Id()]
		if !ok {
			return
		}

		master, err = security.EcDecrypt(c.Pool.Self, k)
		if core.IsErr(err, "cannot decode master key %s %s: %v", c.Pool, h.Id) {
			return
		}

		var private Private
		for userId := range keys {
			private = append(private, userId)
		}
		sort.Strings(private)
		privateId = strings.Join(private, ":")
	}

	buf := bytes.Buffer{}
	err = c.Pool.Receive(h.Id, nil, &buf)
	if core.IsErr(err, "cannot read %s from %s: %v", h.Name, h.Name) {
		return
	}

	data := buf.Bytes()
	if master != nil {
		data, err = security.DecryptBlock(master, master, data)
		if core.IsErr(err, "cannot decrypt private chat in %s %s: %v", c.Pool, h.Id) {
			return
		}
	}

	var m Message
	err = json.Unmarshal(data, &m)

	if core.IsErr(err, "invalid chat message %s: %v", h.Name) {
		return
	}

	hash := getHash(&m)
	if !security.Verify(m.Author, hash, m.Signature) {
		logrus.Error("message %s has invalid signature", h.Name)
		return
	}

	err = sqlSetMessage(c.Pool.Name, c.Name, uint64(id), m.Author, privateId, m)
	core.IsErr(err, "cannot write message %s to db:%v", h.Name)
}
