package pool

import (
	"bytes"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/adrg/xdg"

	"github.com/code-to-go/safepool/core"
	"github.com/code-to-go/safepool/security"
	"github.com/code-to-go/safepool/sql"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
)

var config = Config{
	Name:   "test.safepool.net/public",
	Public: []string{"sftp://sftp_user:11H^m63W5vAL@localhost/sftp_user"},
}

func TestSafeCreation(t *testing.T) {
	dpPath := filepath.Join(xdg.ConfigHome, "safepool.test.db")
	sql.DeleteDB()
	sql.LoadSQLFromFile("../api/sqlite.sql")
	err := sql.OpenDB(dpPath)
	assert.NoErrorf(t, err, "cannot open db")

	self, err := security.NewIdentity("test")
	assert.NoErrorf(t, err, "cannot create identity")

	err = Define(config)
	assert.NoErrorf(t, err, "Cannot define pool: %v", err)

	ForceCreation = true
	ReplicaPeriod = 0
	s, err := Create(self, "test.safepool.net/public", nil)
	assert.NoErrorf(t, err, "Cannot create pool: %v", err)
	s.Close()

	s, err = Open(self, "test.safepool.net/public")
	assert.NoErrorf(t, err, "Cannot open pool: %v", err)
	defer s.Close()

	s1 := "just a simple test"
	h, err := s.Send("test.txt", bytes.NewBufferString(s1), nil)
	assert.NoErrorf(t, err, "Cannot create post: %v", err)

	b2 := bytes.Buffer{}
	err = s.Receive(h.Id, nil, &b2)
	assert.NoErrorf(t, err, "Cannot get %d: %v", h.Id, err)

	s2 := b2.String()
	assert.EqualValues(t, s1, s2)

	hs, _ := s.List(0)
	for _, h := range hs {
		fmt.Printf("\t%s\t%d\t%d", h.Name, h.Size, h.Id)
	}
	s.Delete()
}

func BenchmarkSafe(b *testing.B) {
	sql.DbPath = filepath.Join(xdg.ConfigHome, "safepool.test.db")
	sql.DeleteDB()
	sql.LoadSQLFromFile("../sql/sqlite.sql")
	err := sql.OpenDB(sql.DbPath)
	assert.NoErrorf(b, err, "cannot open db")

	self, err := security.NewIdentity("test")
	assert.NoErrorf(b, err, "cannot create identity")

	err = Define(config)
	assert.NoErrorf(b, err, "Cannot define pool: %v", err)

	ForceCreation = true
	ReplicaPeriod = 0
	s, err := Create(self, "test.safepool.net/public", nil)
	assert.NoErrorf(b, err, "Cannot create pool: %v", err)
	s.Close()

	s, err = Open(self, "test.safepool.net/public")
	assert.NoErrorf(b, err, "Cannot open pool: %v", err)
	defer s.Close()

	s1 := "just a simple test"
	h, err := s.Send("test.txt", bytes.NewBufferString(s1), nil)
	assert.NoErrorf(b, err, "Cannot create post: %v", err)

	b2 := bytes.Buffer{}
	err = s.Receive(h.Id, nil, &b2)
	assert.NoErrorf(b, err, "Cannot get %d: %v", h.Id, err)

	s2 := b2.String()
	assert.EqualValues(b, s1, s2)

	hs, _ := s.List(0)
	for _, h := range hs {
		fmt.Printf("\t%s\t%d\t%d", h.Name, h.Size, h.Id)
	}
	s.Delete()
}

func TestSafeReplica(t *testing.T) {
	sql.DbPath = filepath.Join(xdg.ConfigHome, "safepool.test.db")
	sql.DeleteDB()
	sql.LoadSQLFromFile("../sql/sqlite.sql")
	err := sql.OpenDB(sql.DbPath)
	assert.NoErrorf(t, err, "cannot open db")

	self, err := security.NewIdentity("test")
	assert.NoErrorf(t, err, "cannot create identity")

	err = Define(config)
	assert.NoErrorf(t, err, "Cannot define pool: %v", err)

	ForceCreation = true
	ReplicaPeriod = time.Second * 5

	now := core.Now()
	s, err := Create(self, "test.safepool.net/public", nil)
	creationTime := core.Since(now)
	assert.NoErrorf(t, err, "Cannot create pool: %v", err)
	defer s.Close()
	defer s.Delete()

	s1 := "just a simple test"
	now = core.Now()
	_, err = s.Send("test.txt", bytes.NewBufferString(s1), nil)
	postTime := core.Since(now)
	assert.NoErrorf(t, err, "Cannot create post: %v", err)

	time.Sleep(5 * time.Minute)

	fmt.Printf("creation: %s, post: %s\n", creationTime, postTime)
}
