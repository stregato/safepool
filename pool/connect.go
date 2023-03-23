package pool

import (
	"bytes"
	"fmt"
	"math"
	"math/rand"
	"path"
	"time"

	"github.com/code-to-go/safepool/core"
	"github.com/code-to-go/safepool/storage"

	"github.com/sirupsen/logrus"
)

func pingExchanger(e storage.Storage, pool string, data []byte) (time.Duration, error) {
	start := core.Now()
	name := path.Join(pool, fmt.Sprintf(pingName, start.UnixMilli()))
	err := e.Write(name, core.NewBytesReader(data), int64(len(data)), nil)
	if err != nil {
		return 0, err
	}

	var buf bytes.Buffer
	err = e.Read(name, nil, &buf, nil)
	if err != nil {
		return 0, err
	}
	e.Delete(name)

	if bytes.Equal(data, buf.Bytes()) {
		return core.Since(start), nil
	} else {
		return 0, err
	}
}

func (p *Pool) createExchangers(config Config) {
	for _, e := range p.exchangers {
		e.Close()
	}
	p.exchangers = nil

	urls := append(config.Public, config.Private...)
	for _, url := range urls {
		e, err := storage.OpenStorage(url)
		if core.IsErr(err, "cannot connect to exchange %s in Pool.createExchangers: %v", url) {
			continue
		}
		p.exchangers = append(p.exchangers, e)
	}
}

func (p *Pool) findPrimary() {
	if len(p.exchangers) == 1 {
		p.e = p.exchangers[0]
		return
	}

	min := time.Duration(math.MaxInt64)

	data := make([]byte, 4192)
	rand.Seed(time.Now().Unix())
	rand.Read(data)

	p.e = nil
	for _, e := range p.exchangers {
		ping, err := pingExchanger(e, p.Name, data)
		if err != nil {
			logrus.Warnf("no connection to %v", e)
			continue
		}
		if ping < min {
			min = ping
			p.e = e
		}
	}
}

func (p *Pool) connectSafe(config Config) error {
	p.createExchangers(config)
	p.findPrimary()
	if p.e == nil {
		logrus.Warnf("no available exchange for domain %s", p.Name)
		return ErrNoExchange
	} else {
		p.Connection = p.e.String()
		return nil
	}
}
