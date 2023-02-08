package pool

import (
	"bytes"
	"fmt"
	"math"
	"math/rand"
	"path"
	"time"

	"github.com/code-to-go/safe/safepool/core"
	"github.com/code-to-go/safe/safepool/transport"

	"github.com/sirupsen/logrus"
)

func pingExchanger(e transport.Exchanger, pool string, data []byte) (time.Duration, error) {
	start := core.Now()
	name := path.Join(pool, fmt.Sprintf(pingName, start.UnixMilli()))
	err := e.Write(name, bytes.NewBuffer(data))
	if err != nil {
		return 0, err
	}

	var buf bytes.Buffer
	err = e.Read(name, nil, &buf)
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
		e, err := transport.NewExchanger(url)
		if core.IsErr(err, "cannot connect to exchange %s: %v", url) {
			continue
		}
		p.exchangers = append(p.exchangers, e)
	}
}

func (p *Pool) findPrimary() {
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
		return nil
	}
}
