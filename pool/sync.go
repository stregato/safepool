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
	"github.com/code-to-go/safepool/sql"
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
		if f.Name() >= last && f.IsDir() {
			slots = append(slots, f.Name())
		}
	}

	sort.Strings(slots)
	return slots, nil
}

func (p *Pool) syncFeeds() ([]Head, error) {
	configNode := fmt.Sprintf("pool/%s", p.Name)
	checkpointKey := fmt.Sprintf("checkpoints/%s", p.e.String())
	slotKey := fmt.Sprintf("slots/%s", p.e.String())
	_, lastCheckpoint, _, _ := sql.GetConfig(configNode, checkpointKey)
	lastSlot, _, _, _ := sql.GetConfig(configNode, slotKey)

	var checkpoint int64
	if stat, err := p.e.Stat(path.Join(p.Name, FeedsFolder, ".touch")); err == nil {
		checkpoint = stat.ModTime().UnixMilli()
	}
	if lastCheckpoint > 0 && checkpoint <= lastCheckpoint {
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
	skippedFeeds := 0
	idThresold := p.BaseId()
	slotThresold := p.baseSlot()
	for _, slot := range slots {
		fs, err := p.e.ReadDir(path.Join(p.Name, FeedsFolder, slot), 0)
		if core.IsErr(err, "cannot read content in slot %s in pool", slot, p) {
			continue
		}
		if slot < slotThresold {
			for _, f := range fs {
				p.e.Delete(f.Name())
			}
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
			if id < int64(idThresold) {
				core.Debug("file '%s' has id %d lower than thresold %d; delete it", name, id, idThresold)
				p.e.Delete(n)
				continue
			}

			f, err := p.readHead(p.e, n)
			if core.IsErr(err, "cannot read file %s from %s: %v", n, p.e) {
				skippedFeeds++
				continue
			}

			_, ok, _ := security.GetIdentity(f.AuthorId)
			if !ok {
				skippedFeeds++
				core.Info("feed with unknown id '%s', skip sync", f.AuthorId)
				continue
			}

			f.Slot = slot
			f.CTime = p.getCTime()
			_ = sqlAddFeed(p.Name, f)
			core.Debug("file '%s' has old id; skip", name)
			hs = append(hs, f)
		}
		lastSlot = slot
		if skippedFeeds == 0 {
			sql.SetConfig(configNode, slotKey, slot, 0, nil)
		}
	}
	core.Info("sync completed, %d new heads, pendingFeeds %d, slot '%s', modTime %d", len(hs), skippedFeeds,
		lastSlot, lastCheckpoint)
	return hs, nil
}

func (p *Pool) Sync() ([]Head, error) {
	if core.Since(p.lastAccessSync) >= SyncAccessFrequency {
		p.SyncAccess(false)
		p.lastAccessSync = core.Now()
	}

	hs, err := p.syncFeeds()
	if err != nil {
		return nil, err
	}

	// if core.Since(p.lastReplica) > HouseKeepingPeriods[AvailableBandwidth] {
	// 	go func() {
	// 		//			p.HouseKeeping()
	// 		time.Sleep(5 * time.Second)
	// 		p.replica()
	// 		p.lastReplica = core.Now()
	// 	}()
	// }
	return hs, nil
}
