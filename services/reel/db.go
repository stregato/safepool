package reel

import (
	"time"

	"github.com/code-to-go/safepool/core"
	"github.com/code-to-go/safepool/sql"
)

func sqlSetReel(pool, reel, thread string, id uint64, author, contentType string, ctime time.Time, thumbnail []byte) error {
	_, err := sql.Exec("SET_REEL", sql.Args{"pool": pool, "reel": reel, "thread": thread, "id": id, "author": author,
		"contentType": contentType, "ctime": sql.EncodeTime(ctime), "thumbnail": thumbnail})
	core.IsErr(err, "cannot set message %d on db: %v", id)
	return err
}

// id,name,contentType,ctime,thumnail
func sqlListReel(pool, reel, thread string, from, to time.Time, limit int) ([]Head, error) {
	var heads []Head
	rows, err := sql.Query("GET_REEL", sql.Args{"pool": pool, "reel": reel, "thread": thread,
		"from": sql.EncodeTime(from), "to": sql.EncodeTime(to), "limit": limit})
	if err == nil {
		for rows.Next() {
			var ctime int64
			var h Head
			err = rows.Scan(&h.Id, &h.Name, &h.ContentType, &ctime, &h.Thumbnail)
			if !core.IsErr(err, "cannot read message from db: %v", err) {
				h.Time = sql.DecodeTime(ctime)
				heads = append(heads, h)
			}
		}
	}

	return heads, err
}

func sqlReset(pool, reel string) error {
	_, err := sql.Exec("DELETE_REEL", sql.Args{"pool": pool, "reel": reel})
	return err
}

func sqlListThreads(pool string, reel string) ([]string, error) {
	var threads []string
	rows, err := sql.Query("GET_REEL_THREADS", sql.Args{"pool": pool, "reel": reel})
	if core.IsErr(err, "cannot read threads of %s/%s from db: %v", pool, reel) {
		return nil, err
	}
	for rows.Next() {
		var thread string

		err = rows.Scan(&thread)
		if core.IsErr(err, "cannot read private of %s/%s from db: %v", pool, reel) {
			return nil, err
		}
		threads = append(threads, thread)
	}
	return threads, err
}
