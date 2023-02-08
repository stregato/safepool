package pool

import (
	"encoding/json"

	"github.com/code-to-go/safe/safepool/core"
	"github.com/code-to-go/safe/safepool/security"
	"github.com/code-to-go/safe/safepool/sql"
)

func sqlGetFeeds(pool string, offset int) ([]Feed, error) {
	rows, err := sql.Query("GET_FEEDS", sql.Args{"pool": pool, "offset": offset})
	if core.IsErr(err, "cannot get pools feeds from db: %v") {
		return nil, err
	}
	defer rows.Close()

	var feeds []Feed
	for rows.Next() {
		var f Feed
		var modTime int64
		var hash string
		var meta string
		err = rows.Scan(&f.Id, &f.Name, &modTime, &f.Size, &f.AuthorId, &hash, &f.Offset, &meta, &f.Slot)
		if !core.IsErr(err, "cannot read pool feeds from db: %v") {
			f.Hash = sql.DecodeBase64(hash)
			f.ModTime = sql.DecodeTime(modTime)
			f.Meta = sql.DecodeBase64(meta)
			feeds = append(feeds, f)
		}
	}
	return feeds, nil
}

func sqlGetFeed(pool string, id uint64) (Feed, error) {
	var f Feed
	var modTime int64
	var hash string
	var meta string
	err := sql.QueryRow("GET_FEED", sql.Args{"pool": pool, "id": id},
		&f.Id, &f.Name, &modTime, &f.Size, &f.AuthorId, &hash, &f.Offset, &meta, &f.Slot)
	if core.IsErr(err, "cannot get feed with id '%d' in pool '%s': %v", id, pool) {
		return Feed{}, err
	}

	f.Hash = sql.DecodeBase64(hash)
	f.ModTime = sql.DecodeTime(modTime)
	f.Meta = sql.DecodeBase64(meta)
	return f, nil
}

func sqlDelFeedBefore(pool string, id int64) error {
	_, err := sql.Exec("DEL_FEED_BEFORE", sql.Args{"pool": pool, "beforeId": id})
	return err
}

func sqlAddFeed(pool string, f Feed) error {
	_, err := sql.Exec("SET_FEED", sql.Args{
		"pool":     pool,
		"id":       f.Id,
		"name":     f.Name,
		"size":     f.Size,
		"authorId": f.AuthorId,
		"modTime":  sql.EncodeTime(f.ModTime),
		"hash":     sql.EncodeBase64(f.Hash[:]),
		"meta":     sql.EncodeBase64(f.Meta),
		"slot":     f.Slot,
	})
	return err
}

func (p *Pool) sqlGetKey(keyId uint64) []byte {
	rows, err := sql.Query("GET_KEY", sql.Args{"pool": p.Name, "keyId": keyId})
	if err != nil {
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		var key string
		var err = rows.Scan(&key)
		if !core.IsErr(err, "cannot read key from db: %v") {
			return sql.DecodeBase64(key)
		}
	}
	return nil
}

func (p *Pool) sqlSetKey(keyId uint64, value []byte) error {
	_, err := sql.Exec("SET_KEY", sql.Args{"pool": p.Name, "keyId": keyId, "keyValue": sql.EncodeBase64(value)})
	return err
}

func (p *Pool) sqlGetKeystore() (Keystore, error) {
	rows, err := sql.Query("GET_KEYS", sql.Args{"pool": p.Name})
	if core.IsErr(err, "cannot read keystore for pool %s: %v", p.Name) {
		return nil, err
	}
	defer rows.Close()

	ks := Keystore{}
	for rows.Next() {
		var keyId uint64
		var keyValue string
		var err = rows.Scan(&keyId, &keyValue)
		if !core.IsErr(err, "cannot read key from db: %v") {
			ks[keyId] = sql.DecodeBase64(keyValue)
		}
	}
	return ks, nil
}

func (p *Pool) sqlGetAccesses(onlyTrusted bool) (identities []security.Identity, accesses []Access, err error) {
	var q string
	if onlyTrusted {
		q = "GET_TRUSTED_ACCESSES"
	} else {
		q = "GET_ACCESSES"
	}

	rows, err := sql.Query(q, sql.Args{"pool": p.Name})
	if core.IsErr(err, "cannot get trusted identities from db: %v") {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var i security.Identity
		var id string
		var i64 string
		var state State
		var modTime int64
		var ts int64
		err = rows.Scan(&id, &i64, &state, &modTime, &ts)
		if core.IsErr(err, "cannot read identity from db: %v") {
			continue
		}

		i, err = security.IdentityFromBase64(i64)
		if core.IsErr(err, "invalid identity '%s': %v", i64) {
			continue
		}
		identities = append(identities, i)

		accesses = append(accesses, Access{
			Id:      id,
			ModTime: sql.DecodeTime(modTime),
			State:   state,
		})
	}
	return identities, accesses, nil
}

func (p *Pool) sqlSetAccess(a Access) error {
	_, err := sql.Exec("SET_ACCESS", sql.Args{
		"id":      a.Id,
		"pool":    p.Name,
		"modTime": sql.EncodeTime(a.ModTime),
		"state":   a.State,
		"ts":      sql.EncodeTime(core.Now()),
	})
	return err
}

func sqlSetPool(name string, c Config) error {
	data, err := json.Marshal(&c)
	if core.IsErr(err, "cannot marshal transport configuration of %s: %v", name) {
		return err
	}

	_, err = sql.Exec("SET_POOL", sql.Args{"name": name, "configs": sql.EncodeBase64(data)})
	core.IsErr(err, "cannot save transport configuration of %s: %v", name)
	return err
}

func sqlGetPool(name string) (Config, error) {
	var blob string
	var c Config
	err := sql.QueryRow("GET_POOL", sql.Args{"name": name}, &blob)
	if core.IsErr(err, "cannot get pool %s config: %v", name) {
		return Config{}, err
	}

	data := sql.DecodeBase64(blob)
	err = json.Unmarshal(data, &c)
	core.IsErr(err, "cannot unmarshal configs of %s: %v", name)
	return c, err
}

func sqlSetSlot(pool, exchange, slot string) error {
	_, err := sql.Exec("SET_SLOT", sql.Args{"pool": pool, "exchange": exchange, "slot": slot})
	core.IsErr(err, "cannot save slot %s: %v", slot)
	return err
}

func sqlGetSlot(pool, exchange string) (string, error) {
	var slot string
	err := sql.QueryRow("GET_SLOT", sql.Args{"pool": pool, "exchange": exchange}, &slot)
	if core.IsErr(err, "cannot get slot: %v") {
		return "", err
	}
	return slot, nil
}

func sqlListPool() ([]string, error) {
	var names []string
	rows, err := sql.Query("LIST_POOL", nil)
	if core.IsErr(err, "cannot list pools: %v") {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var n string
		err = rows.Scan(&n)
		if err == nil {
			names = append(names, n)
		}
	}
	return names, err
}
