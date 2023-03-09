package pool

import (
	"fmt"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/code-to-go/safepool/core"
	"github.com/code-to-go/safepool/security"
)

const FeedsFolder = "feeds"
const SyncAccessFrequency = 5 * time.Minute

func (p *Pool) getSlots(last string) ([]string, error) {
	fs, err := p.e.ReadDir(path.Join(p.Name, FeedsFolder), 0)
	if os.IsNotExist(err) || core.IsErr(err, "cannot list slots in '%v': %v", p) {
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

func (p *Pool) Sync() ([]Head, error) {
	if core.Since(p.lastAccessSync) >= SyncAccessFrequency {
		p.syncAccess()
		p.lastAccessSync = core.Now()
	}

	tag := fmt.Sprintf("feeds@%s", p.e.String())
	lastSlot, modTime, err := sqlGetCheckpoint(p.Name, tag)
	if core.IsErr(err, "cannot read checkpoint for pool '%s': %v", p.Name) {
		return nil, err
	}

	if modTime > 0 && p.e.GetCheckpoint(path.Join(p.Name, FeedsFolder, ".touch")) <= modTime {
		core.Info("checkpoint is recent, skip sync")
		return nil, nil
	}
	hs, _ := p.List(0)

	feeds := map[uint64]Head{}
	for _, h := range hs {
		feeds[h.Id] = h
	}

	slots, err := p.getSlots(lastSlot)
	if err != nil {
		core.Info("no slots, skip sync")
		return nil, err
	}

	core.Debug("find slots: %v", strings.Join(slots, ","))
	pendingFeeds := 0
	thresold := p.BaseId()
	for _, slot := range slots {
		fs, err := p.e.ReadDir(path.Join(p.Name, FeedsFolder, slot), 0)
		if core.IsErr(err, "cannot read content in slot %s in pool", slot, p) {
			continue
		}
		core.Debug("%d files in folder %s/%s/%s", len(fs), p.Name, FeedsFolder, slot)
		for _, f := range fs {
			name := f.Name()
			if !strings.HasSuffix(name, ".head") {
				core.Debug("file '%s' is not an header", name)
				continue
			}

			id, err := strconv.ParseInt(name[0:len(name)-len(".head")], 10, 64)
			if err != nil {
				core.Debug("file '%s' has unexpected format", name)
				continue
			}
			if _, found := feeds[uint64(id)]; found {
				core.Debug("file '%s' has known id; skip", name)
				continue
			}

			n := path.Join(p.Name, FeedsFolder, slot, name)
			if id < int64(thresold) {
				core.Debug("file '%s' has id %d lower than thresold %d; delete it", name, id, thresold)
				p.e.Delete(n)
				continue
			}

			f, err := p.readHead(p.e, n)
			if core.IsErr(err, "cannot read file %s from %s: %v", n, p.e) {
				pendingFeeds++
				continue
			}

			identity, ok, _ := security.GetIdentity(f.AuthorId)
			if !ok || identity.Nick == "" {
				err = p.importIdentity(p.e, f.AuthorId)
				if err != nil {
					pendingFeeds++
					core.Info("feed with unknown id '%s', skip sync", f.AuthorId)
					continue
				}
			}

			f.Slot = slot
			f.CTime = p.getCTime()
			_ = sqlAddFeed(p.Name, f)
			core.Debug("file '%s' has old id; skip", name)
			hs = append(hs, f)
		}
		lastSlot = slot
	}
	if pendingFeeds == 0 {
		err = sqlSetCheckpoint(p.Name, fmt.Sprintf("feeds@%s", p.e.String()), lastSlot, modTime)
		core.IsErr(err, "cannot save checkpoint to db: %v")
	}
	core.Info("sync completed, %d new heads, pendingFeeds %d, slot '%s', modTime %d", len(hs), pendingFeeds,
		lastSlot, modTime)

	if AvailableBandwidth != LowBandwidth && core.Since(p.lastHouseKeeping) > HouseKeepingPeriod {
		go func() {
			p.HouseKeeping()
			p.replica()
			p.lastHouseKeeping = core.Now()
		}()
	}
	return hs, nil
}
