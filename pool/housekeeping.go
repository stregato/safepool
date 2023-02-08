package pool

import (
	"fmt"
	"log"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/code-to-go/safe/safepool/core"
	"github.com/code-to-go/safe/safepool/transport"
	"github.com/godruoyi/go-snowflake"
)

// LifeSpan is the maximal time data should stay in the pool. It is default to 30 days.
var LifeSpan = time.Hour //30 * 24 * time.Hour

// func (p *Pool) startHouseKeeping() {
// 	if LifeSpan < sevenDays {
// 		LifeSpan = sevenDays
// 	}

// 	LifeSpan = time.Hour

// 	go func() {
// 		rand.Seed(time.Now().UnixNano())
// 		n := rand.Intn(600)
// 		time.Sleep(time.Duration(n) * time.Second)
// 		p.stopHouseKeeping = make(chan bool)
// 		p.houseKeeping = time.NewTicker(time.Hour)

// 		for {
// 			p.HouseKeeping()
// 			select {
// 			case <-p.stopHouseKeeping:
// 				return
// 			case <-p.houseKeeping.C:
// 				continue
// 			}
// 		}
// 	}()
// }

func (p *Pool) getAllSlots(e transport.Exchanger) []string {
	fs, err := e.ReadDir(path.Join(p.Name, FeedsFolder), 0)
	if core.IsErr(err, "cannot read content in pool %s exchange %s", p.Name, e) {
		return nil
	}
	var slots []string
	for _, f := range fs {
		slots = append(slots, f.Name())
	}
	return slots
}

// HouseKeeping removes old files from the pool. It is called automatically when you use Sync after an hour;
// use explicitly only when your application does not use sync or does not live longer than 1 hour
func (p *Pool) HouseKeeping() {
	p.houseKeepingLock.Lock()
	defer p.houseKeepingLock.Unlock()

	thresoldId := p.BaseId()
	for _, e := range p.exchangers {
		slots := p.getAllSlots(e)
		for _, slot := range slots {
			fs, err := e.ReadDir(path.Join(p.Name, FeedsFolder, slot), 0)
			if core.IsErr(err, "cannot read content in pool %s/%s", e, p.Name) {
				continue
			}
			for _, f := range fs {
				name := f.Name()
				if !strings.HasSuffix(name, ".head") {
					continue
				}

				name = name[0 : len(name)-len(".head")]
				id, err := strconv.ParseInt(name, 10, 64)
				if err != nil {
					continue
				}

				if uint64(id) < thresoldId {
					p.e.Delete(path.Join(p.Name, FeedsFolder, slot, fmt.Sprintf("%s.head", name)))
					p.e.Delete(path.Join(p.Name, FeedsFolder, slot, fmt.Sprintf("%s.body", name)))
				}
			}
		}
	}

	sqlDelFeedBefore(p.Name, int64(thresoldId))
}

func (p *Pool) BaseId() uint64 {
	thresold := (core.Since(core.SnowFlakeStart) - LifeSpan) / time.Millisecond

	if thresold < 0 {
		thresold = 0
	}
	if thresold >= 1<<41 {
		log.Fatalf("Current time %v is bigger that longest possible with the current snowFlake start %v", core.Now(), core.SnowFlakeStart)
	}
	return uint64(thresold) << (snowflake.SequenceLength + snowflake.MachineIDLength)
}
