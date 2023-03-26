package pool

import (
	"bytes"
	"fmt"
	"path"

	"github.com/code-to-go/safepool/core"
	"github.com/code-to-go/safepool/sql"
)

func (p *Pool) getGuardParams(ph ...string) (configNode, file, tag string) {
	configNode = fmt.Sprintf("pool/%s", p.Name)
	phj := path.Join(ph...)
	file = path.Join(p.Name, phj)

	tag = fmt.Sprintf("%s/@%s", phj, p.e.String())
	return configNode, file, tag
}

func (p *Pool) checkGuard(ph ...string) bool {
	configNode, file, tag := p.getGuardParams(ph...)
	_, lastCheckpoint, _, _ := sql.GetConfig(configNode, tag)

	var checkpoint int64
	if stat, err := p.e.Stat(file); err == nil {
		checkpoint = stat.ModTime().UnixMilli()
	}

	ok := checkpoint != 0 && lastCheckpoint != 0 && checkpoint <= lastCheckpoint
	if !ok {
		sql.SetConfig(configNode, tag, "", checkpoint, nil)
	}
	core.Debug("guard %s, %d < %d = %b", file, checkpoint, lastCheckpoint, ok)
	return ok
}

func (p *Pool) touchGuard(ph ...string) {
	_, file, _ := p.getGuardParams(ph...)
	err := p.e.Write(file, bytes.NewReader(nil), 0, nil)
	core.IsErr(err, "cannot touch guard: %v")
}
