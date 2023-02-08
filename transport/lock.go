package transport

import (
	"time"

	"github.com/code-to-go/safe/safepool/core"

	"github.com/godruoyi/go-snowflake"
)

type lockFileContent struct {
	Id   uint64
	Span time.Duration
}

func waitLock(e Exchanger, name string) {
	var c lockFileContent
	var err error
	var id uint64
	var shot time.Time

	for shot.IsZero() && core.Since(shot) > c.Span {
		id = c.Id
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

func LockFile(e Exchanger, name string, span time.Duration) (uint64, error) {
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

func UnlockFile(e Exchanger, name string, id uint64) {
	var c lockFileContent
	_ = ReadJSON(e, name, &c, nil)
	if c.Id == id {
		e.Delete(name)
	}
}
