package sql

import (
	_ "embed"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestDb(t *testing.T) {
	// DbName = "safepool.test.db"
	// DeleteDB()

	// LoadSQLFromFile("sqlite.sql")
	// err := OpenDB()
	// assert.NoErrorf(t, err, "cannot open sqllite: %v", err)

	// s, _, _, ok := GetConfig("", "identity.public")
	// if !ok {
	// 	err = SetConfig("", "identity.public", "test", 0, nil)
	// 	assert.NoErrorf(t, err, "cannot set config: %v", err)
	// 	s, _, _, ok = GetConfig("", "identity.public")
	// }
	// assert.Equal(t, "test", s, "cannot get config: %v", err)

	// err = SetAccess(model.Transport{})
	// assert.NoErrorf(t, err, "cannot add domain: %v", err)

	// domains, err := GetDomains()
	// assert.NoErrorf(t, err, "cannot list domains: %v", err)
	// assert.Contains(t, domains, "public.safepool.zone", "cannot find expected domain")

	// now := time.Unix(core.Now().Unix(), 0)
	// err = SetFile(model.File{
	// 	Domain:  "public.safepool.zone",
	// 	Name:    "test.txt",
	// 	Author:  security.Identity{},
	// 	Hash:    []byte("hash"),
	// 	ModTime: now,
	// 	State:   model.LocalCreated,
	// })
	// assert.NoErrorf(t, err, "cannot set file: %v", err)

	// f, err := GetFileByName("public.safepool.zone", "test.txt")
	// assert.NoErrorf(t, err, "cannot get file: %v", err)
	// assert.Equal(t, now, f.ModTime)

	// err = SetEncKey("public.safepool.zone", 1, []byte("some key"))
	// assert.NoErrorf(t, err, "cannot set key: %v", err)

	// keys, err := GetEncKeys("public.safepool.zone")
	// assert.NoErrorf(t, err, "cannot get keys: %v", err)
	// assert.Len(t, keys, 1, "keys is not 1 size")
	// assert.Equal(t, []byte("some key"), keys[1], "unexpected key value")

	// err = CloseDB()
	// assert.NoErrorf(t, err, "cannot close sqllite: %v", err)
}
