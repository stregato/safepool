package pool

import (
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/code-to-go/safe/safepool/core"
)

const FeedsFolder = "feeds"

func (p *Pool) getSlots() ([]string, error) {
	last, _ := sqlGetSlot(p.Name, p.e.String())

	fs, err := p.e.ReadDir(path.Join(p.Name, FeedsFolder), 0)
	if core.IsErr(err, "cannot list slots in '%v': %v", p) {
		return nil, err
	}

	var slots []string
	for _, f := range fs {
		if f.Name() >= last {
			slots = append(slots, f.Name())
		}
	}

	sort.Strings(slots)
	return slots, nil
}

func (p *Pool) Sync() error {
	if !p.e.Touched(p.Name + "/") {
		return nil
	}
	hs, _ := p.List(0)

	feeds := map[uint64]Feed{}
	for _, h := range hs {
		feeds[h.Id] = h
	}

	slots, err := p.getSlots()
	if err != nil {
		return err
	}

	thresold := p.BaseId()
	for _, slot := range slots {
		fs, err := p.e.ReadDir(path.Join(p.Name, FeedsFolder, slot), 0)
		if core.IsErr(err, "cannot read content in slot %s in pool", slot, p) {
			continue
		}
		for _, f := range fs {
			name := f.Name()
			if !strings.HasSuffix(name, ".head") {
				continue
			}

			id, err := strconv.ParseInt(name[0:len(name)-len(".head")], 10, 64)
			if err != nil {
				continue
			}
			if _, found := feeds[uint64(id)]; found {
				continue
			}

			if id < int64(thresold) {
				continue
			}

			n := path.Join(p.Name, FeedsFolder, slot, name)
			f, err := p.readHead(n)
			if core.IsErr(err, "cannot read file %s from %s: %v", n, p.e) {
				continue
			}
			f.Slot = slot
			_ = sqlAddFeed(p.Name, f)
			hs = append(hs, f)
		}
		sqlSetSlot(p.Name, p.e.String(), slot)
	}

	if time.Until(p.lastHouseKeeping) > ReplicaPeriod {
		p.HouseKeeping()
		p.replica()
		p.lastHouseKeeping = core.Now()
	}
	return nil
}
