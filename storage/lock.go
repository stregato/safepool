package storage

import (
	"os"
	"time"

	"github.com/code-to-go/safepool/core"

	"github.com/godruoyi/go-snowflake"
)

type lockFileContent struct {
	Id   uint64
	Span time.Duration
}

func waitLock(e Storage, name string) {
	var c lockFileContent
	var err error
	var id uint64
	var shot time.Time

	for shot.IsZero() && core.Since(shot) > c.Span {
		id = c.Id
		_, err = e.Stat(name)
		if os.IsNotExist(err) {
			return
		}
		err = ReadJSON(e, name, &c, nil)
		if err != nil {
			return
		}
		if c.Id != id {
			shot = core.Now()
		}
		time.Sleep(time.Second)
	}
}

func LockFile(e Storage, name string, span time.Duration) (uint64, error) {
	var c lockFileContent
	id := snowflake.ID()
	for c.Id != id {
		waitLock(e, name)

		err := WriteJSON(e, name, lockFileContent{
			Id:   id,
			Span: span,
		}, nil)
		if core.IsErr(err, "cannot write lock file %s: %v", name) {
			return 0, err
		}
		time.Sleep(time.Second)
		err = ReadJSON(e, name, &c, nil)
		if core.IsErr(err, "cannot read lock file %s: %v", name) {
			return 0, err
		}
	}
	return id, nil
}

func UnlockFile(e Storage, name string, id uint64) {
	var c lockFileContent
	_, err := e.Stat(name)
	if os.IsNotExist(err) {
		return
	}
	_ = ReadJSON(e, name, &c, nil)
	if c.Id == id {
		e.Delete(name)
	}
}
