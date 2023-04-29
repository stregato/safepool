package pool

import (
	"path"
	"sort"
	"time"

	"github.com/code-to-go/safepool/core"
	"github.com/code-to-go/safepool/storage"
)

func (p *Pool) listReplicaSlots() []string {
	m := map[string]bool{}

	for _, e := range p.exchangers {
		fs, _ := e.ReadDir(path.Join(p.Name, FeedsFolder), 0)
		for _, f := range fs {
			name := f.Name()
			if f.IsDir() && name >= p.lastReplicaSlot {
				m[name] = true
			}
		}
	}
	var slots []string
	for s := range m {
		slots = append(slots, s)
	}
	sort.Strings(slots)
	return slots
}

func (p *Pool) startReplica() {
	ticker := time.NewTicker(10 * time.Second)
	p.quitReplica = make(chan bool)
	go func() {
		for {
			select {
			case <-ticker.C:
				if core.Since(p.lastReplica) > HouseKeepingPeriods[AvailableBandwidth] {
					p.replica()
					p.lastReplica = core.Now()
				}
			case <-p.quitReplica:
				ticker.Stop()
				return
			}
		}
	}()
}
func (p *Pool) stopReplica() {
	if p.quitReplica != nil {
		p.quitReplica <- true
	}
}

func (p *Pool) replica() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	slots := p.listReplicaSlots()
	for _, e := range p.exchangers {
		if e != p.e {
			for _, s := range slots {
				err := p.syncContent(e, path.Join(FeedsFolder, s))
				core.IsErr(err, "cannot sync slot %s for secondary %s during replica: %v", s, e)
			}
			err := p.syncContent(e, identityFolder)
			core.IsErr(err, "cannot sync identities for secondary %s during replica: %v", e)
		}
	}
	p.lastReplicaSlot = core.If(len(slots) > 0, slots[len(slots)-1], "")
}

func (p *Pool) syncContent(e storage.Storage, folder string) error {
	folder = path.Join(p.Name, folder)
	ls, _ := p.e.ReadDir(folder, 0)
	m := map[string]bool{}
	for _, l := range ls {
		n := l.Name()
		if n[0] != '.' {
			m[l.Name()] = true
		}
	}

	ls, _ = e.ReadDir(folder, 0)
	for _, l := range ls {
		n := l.Name()
		if n[0] != '.' && !m[n] {
			fn := path.Join(folder, n)
			err := storage.CopyFile(p.e, fn, e, fn)
			core.IsErr(err, "cannot clone '%s': %v", fn)
			core.Info("copied '%s' from '%s' to '%s'", fn, e, p.e)
		}
		delete(m, n)
	}

	for n := range m {
		n = path.Join(folder, n)
		err := storage.CopyFile(e, n, p.e, n)
		core.Info("copied '%s' from '%s' to '%s'", p.e, e)
		core.IsErr(err, "cannot clone '%s': %v", n)
	}

	return nil
}
