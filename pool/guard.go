package pool

import (
	"bytes"
	"fmt"
	"os"
	"path"

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

	return checkpoint == 0 || lastCheckpoint == 0 || checkpoint <= lastCheckpoint
}

func (p *Pool) updateGuard(hasChanges bool, ph ...string) {
	configNode, file, tag := p.getGuardParams(ph...)
	if hasChanges {
		p.e.Write(file, bytes.NewReader(nil), 0, nil)
	}
	f, err := p.e.Stat(file)
	if os.IsNotExist(err) {
		p.e.Write(file, bytes.NewReader(nil), 0, nil)
		f, err = p.e.Stat(file)
	}
	if err == nil {
		sql.SetConfig(configNode, tag, "", f.ModTime().UnixMilli(), nil)
	}
}

// func (p *Pool) resetGuard(ph ...string) {
// 	phj := path.Join(ph...)
// 	configNode := fmt.Sprintf("pool/%s", p.Name)
// 	tag := fmt.Sprintf("%s/@%s", phj, p.e.String())
// 	sql.SetConfig(configNode, tag, "", 0, nil)
// }
