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

	"github.com/code-to-go/safe/safepool/core"
	pool "github.com/code-to-go/safe/safepool/pool"
	"github.com/code-to-go/safe/safepool/security"
	"github.com/godruoyi/go-snowflake"
	"github.com/sirupsen/logrus"
)

type Message struct {
	Id          uint64    `json:"id,string"`
	Author      string    `json:"author"`
	Time        time.Time `json:"time"`
	ContentType string    `json:"contentType"`
	Text        string    `json:"text"`
	Binary      []byte    `json:"bytes"`
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

type Chat struct {
	Pool    *pool.Pool
	Channel string
}

func Get(p *pool.Pool, channel string) Chat {
	return Chat{
		Pool:    p,
		Channel: channel,
	}
}

func (c *Chat) TimeOffset(s *pool.Pool) int {
	return sqlGetOffset(s.Name)
}

func (c *Chat) Accept(s *pool.Pool, feed pool.Feed) bool {
	name := feed.Name
	if !strings.HasPrefix(name, "/chat/") || !strings.HasSuffix(name, ".chat") || feed.Size > 10*1024*1024 {
		return false
	}
	name = path.Base(name)
	id, err := strconv.ParseInt(name[0:len(name)-5], 10, 64)
	if err != nil {
		return false
	}

	buf := bytes.Buffer{}
	err = s.Receive(feed.Id, nil, &buf)
	if core.IsErr(err, "cannot read %s from %s: %v", feed.Name, s.Name) {
		return true
	}

	var m Message
	err = json.Unmarshal(buf.Bytes(), &m)
	if core.IsErr(err, "invalid chat message %s: %v", feed.Name) {
		return true
	}

	h := getHash(&m)
	if !security.Verify(m.Author, h, m.Signature) {
		logrus.Error("message %s has invalid signature", feed.Name)
		return true
	}

	err = sqlSetMessage(s.Name, uint64(id), m.Author, m, feed.Offset)
	core.IsErr(err, "cannot write message %s to db:%v", feed.Name)
	return true
}

func (c *Chat) SendMessage(contentType string, text string, binary []byte) (uint64, error) {
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

	go func() {
		name := fmt.Sprintf("%s/%d.chat", c.Channel, m.Id)
		_, err = c.Pool.Send(name, bytes.NewBuffer(data), nil)
		core.IsErr(err, "cannot write chat message: %v")
	}()

	core.Info("added chat message with id %d", m.Id)
	return m.Id, nil
}

func (c *Chat) accept(h pool.Feed) {
	name := h.Name
	if !strings.HasPrefix(name, c.Channel) || !strings.HasSuffix(name, ".chat") || h.Size > 10*1024*1024 {
		return
	}
	name = path.Base(name)
	id, err := strconv.ParseInt(name[0:len(name)-5], 10, 64)
	if err != nil {
		return
	}

	buf := bytes.Buffer{}
	err = c.Pool.Receive(h.Id, nil, &buf)
	if core.IsErr(err, "cannot read %s from %s: %v", h.Name, h.Name) {
		return
	}

	var m Message
	err = json.Unmarshal(buf.Bytes(), &m)
	if core.IsErr(err, "invalid chat message %s: %v", h.Name) {
		return
	}

	hash := getHash(&m)
	if !security.Verify(m.Author, hash, m.Signature) {
		logrus.Error("message %s has invalid signature", h.Name)
		return
	}

	err = sqlSetMessage(c.Pool.Name, uint64(id), m.Author, m, h.Offset)
	core.IsErr(err, "cannot write message %s to db:%v", h.Name)
}

func (c *Chat) GetMessages(afterId, beforeId uint64, limit int) ([]Message, error) {
	c.Pool.Sync()
	hs, err := c.Pool.List(sqlGetOffset(c.Pool.Name))
	if core.IsErr(err, "cannot retrieve messages from pool: %v") {
		return nil, err
	}
	for _, h := range hs {
		c.accept(h)
	}

	messages, err := sqlGetMessages(c.Pool.Name, afterId, beforeId, limit)
	if core.IsErr(err, "cannot read messages from db: %v") {
		return nil, err
	}

	sort.Slice(messages, func(i, j int) bool {
		return messages[i].Id < messages[j].Id
	})
	return messages, nil
}
