package pool

import (
	"os"
	"path"
	"sync"

	"github.com/code-to-go/safepool/core"
	"github.com/code-to-go/safepool/transport"
)

var Connections = map[string]transport.Exchanger{}
var ConnectionsMutex = &sync.Mutex{}

const pingName = ".reserved.ping.%d.test"

func (p *Pool) checkExisting() error {
	for _, e := range p.exchangers {
		_, err := p.e.Stat(path.Join(p.Name, ".access"))
		if os.IsNotExist(err) {
			continue
		}

		if ForceCreation {
			err = e.Delete(p.Name)
			if core.IsErr(err, "cannot delete %s: %v", p.Name) {
				return err
			}
		} else {
			core.IsErr(err, "pool already exist in %s or other issue: %v", e.String())
			return err
		}
	}
	return nil
}
