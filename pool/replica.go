package pool

import (
	"path"

	"github.com/code-to-go/safepool/core"
	"github.com/code-to-go/safepool/storage"
)

func (p *Pool) replica() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	for _, e := range p.exchangers {
		if e != p.e {
			err := p.syncAccessFor(e)
			if core.IsErr(err, "cannot sync access between %s and %s: %v", p.e, e) {
				continue
			}

			for _, f := range []string{identityFolder, FeedsFolder} {
				err = p.syncContent(e, f)
				core.IsErr(err, "cannot access %s during replica: %v", e)
			}
		}
	}
}

func (p *Pool) syncContent(e storage.Storage, folder string) error {
	ls, _ := p.e.ReadDir(path.Join(p.Name, folder), 0)
	m := map[string]bool{}
	for _, l := range ls {
		n := l.Name()
		if n[0] != '.' {
			m[l.Name()] = true
		}
	}

	ls, _ = e.ReadDir(path.Join(p.Name, folder), 0)
	for _, l := range ls {
		n := l.Name()
		if n[0] != '.' && !m[n] {
			n = path.Join(p.Name, n)
			err := storage.CopyFile(p.e, n, e, n)
			core.IsErr(err, "cannot clone '%s': %v", n)
			core.Info("copied '%s' from '%s' to '%s'", e, p.e)
		}
		delete(m, n)
	}

	for n := range m {
		n = path.Join(p.Name, folder, n)
		stat, err := p.e.Stat(n)
		if err == nil {
			if stat.IsDir() {
				err = p.syncContent(e, path.Join(folder, n))
			} else {
				err = storage.CopyFile(e, n, p.e, n)
				core.Info("copied '%s' from '%s' to '%s'", p.e, e)
			}
		}
		core.IsErr(err, "cannot clone '%s': %v", n)
	}

	return nil
}
