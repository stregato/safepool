package pool

import (
	"path"

	"github.com/code-to-go/safe/safepool/core"
	"github.com/code-to-go/safe/safepool/transport"
)

func (p *Pool) replica() {
	for _, e := range p.exchangers {
		if e != p.e {
			_, err := p.sync(e)
			if !core.IsErr(err, "cannot sync access between %s and %s: %v", p.e, e) {
				p.syncContent(e)
			}
		}
	}
}

func (p *Pool) syncContent(e transport.Exchanger) error {
	ls, err := p.e.ReadDir(p.Name, 0)
	if core.IsErr(err, "cannot read file list from %s: %v", p.e) {
		return err
	}

	m := map[string]bool{}
	for _, l := range ls {
		n := l.Name()
		if n[0] != '.' {
			m[l.Name()] = true
		}
	}

	ls, _ = e.ReadDir(p.Name, 0)
	for _, l := range ls {
		n := l.Name()
		if n[0] != '.' && !m[n] {
			n = path.Join(p.Name, n)
			_ = transport.CopyFile(p.e, n, e, n)
		}
		delete(m, n)
	}

	for n := range m {
		n = path.Join(p.Name, n)
		err = transport.CopyFile(e, n, p.e, n)
		core.IsErr(err, "cannot clone '%s': %v", n)
	}

	return nil
}
