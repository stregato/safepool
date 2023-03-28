package pool

import (
	"bytes"
	"fmt"
	"path"

	"github.com/code-to-go/safepool/core"
	"github.com/code-to-go/safepool/sql"
	"github.com/code-to-go/safepool/storage"
)

func (p *Pool) getGuardParams(e storage.Storage, ph ...string) (configNode, file, tag string) {
	configNode = fmt.Sprintf("pool/%s", p.Name)
	phj := path.Join(ph...)
	file = path.Join(p.Name, phj)

	tag = fmt.Sprintf("%s/@%s", phj, e.String())
	return configNode, file, tag
}

func (p *Pool) checkGuard(e storage.Storage, ph ...string) bool {
	configNode, file, tag := p.getGuardParams(e, ph...)
	_, lastCheckpoint, _, _ := sql.GetConfig(configNode, tag)

	var checkpoint int64
	if stat, err := e.Stat(file); err == nil {
		checkpoint = stat.ModTime().UnixMilli()
	}

	ok := checkpoint != 0 && lastCheckpoint != 0 && checkpoint <= lastCheckpoint
	if !ok {
		sql.SetConfig(configNode, tag, "", checkpoint, nil)
	}
	core.Debug("guard %s, %d < %d = %b", file, checkpoint, lastCheckpoint, ok)
	return ok
}

func (p *Pool) touchGuard(e storage.Storage, ph ...string) {
	_, file, _ := p.getGuardParams(e, ph...)
	err := e.Write(file, bytes.NewReader(nil), 0, nil)
	core.IsErr(err, "cannot touch guard: %v")
}
