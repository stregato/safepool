package pool

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/code-to-go/safepool/core"
	"github.com/code-to-go/safepool/security"
	"github.com/code-to-go/safepool/sql"
	"github.com/code-to-go/safepool/storage"
)

type Bandwidth int

const (
	LowBandwidth Bandwidth = iota
	MediumBandwidth
	HighBandwith
)

var AvailableBandwidth Bandwidth = HighBandwith

var ErrNoExchange = errors.New("no Exchange available")
var ErrNotTrusted = errors.New("the author is not a trusted user")
var ErrNotAuthorized = errors.New("no authorization for this file")
var ErrAlreadyExist = errors.New("pool already exists")
var ErrInvalidToken = errors.New("provided token is invalid: missing name or configs")
var ErrInvalidId = errors.New("provided id not a valid ed25519 public key")
var ErrInvalidConfig = errors.New("provided config is invalid: missing name or configs")
var ErrInvalidName = errors.New("provided pool has invalid name")
var ErrNoSyncClock = errors.New("cannot sync with global time server")

type Consumer interface {
	TimeOffset(s *Pool) time.Time
	Accept(s *Pool, h Head) bool
}

type Config struct {
	Name          string   `json:"name"`
	Public        []string `json:"public"`
	Private       []string `json:"private"`
	Apps          []string `json:"apps"`
	LifeSpanHours int      `json:"lifeSpan"`
}

type Pool struct {
	Name          string            `json:"name"`
	Id            uint64            `json:"id"`
	Self          security.Identity `json:"self"`
	Apps          []string          `json:"apps"`
	LifeSpanHours int               `json:"lifeSpanHours"`
	Trusted       bool              `json:"trusted"`
	Connection    string            `json:"connection"`

	e                  storage.Storage
	exchangers         []storage.Storage
	masterKeyId        uint64
	masterKey          []byte
	lastAccessSync     time.Time
	lastReadAccessFile string
	lastHouseKeeping   time.Time
	ctime              int64
	mutex              sync.Mutex
}

type Head struct {
	Id        uint64    `json:"id"`
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	Hash      []byte    `json:"hash"`
	ModTime   time.Time `json:"modTime"`
	AuthorId  string    `json:"authorId"`
	Signature []byte    `json:"signature"`
	Meta      []byte    `json:"meta"`
	CTime     int64     `json:"-"`
	Slot      string    `json:"-"`
}

const (
	ID_CREATE       = 0x0
	ID_FORCE_CREATE = 0x1
)

var ForceCreation = false
var HouseKeepingPeriod = 10 * time.Minute
var CacheSizeMB = 16
var FeedDateFormat = "20060102"

func List() []string {
	names, _ := sqlListPool()
	return names
}

type AcceptFunc func(feed Head)

const All = ""

func (p *Pool) List(ctime int64) ([]Head, error) {
	hs, err := sqlGetFeeds(p.Name, ctime)
	if core.IsErr(err, "cannot read Pool feeds: %v") {
		return nil, err
	}
	return hs, err
}

func (p *Pool) Close() {
	p.mutex.Lock()
	for _, e := range p.exchangers {
		_ = e.Close()
	}
	p.mutex.Unlock()
}

func (p *Pool) Delete() {
	p.mutex.Lock()
	for _, e := range p.exchangers {
		e.Delete(p.Name)
	}
	p.mutex.Unlock()
}

func (p *Pool) Users() ([]security.Identity, error) {
	identities, _, err := p.sqlGetAccesses(false)
	return identities, err
}

func (p *Pool) Leave() error {
	err := sqlReset(p.Name)
	if core.IsErr(err, "cannot reset pool %s: %v", p) {
		return err
	}
	sql.DelConfigs(fmt.Sprintf("pool/%s", p.Name))
	return nil
}

func (p *Pool) ToString() string {
	return fmt.Sprintf("%s [%v]", p.Name, p.e)
}

var ctimeLock sync.Mutex

func (p *Pool) getCTime() int64 {
	var ctime int64

	ctimeLock.Lock()
	for ctime <= p.ctime {
		ctime = time.Now().UnixMicro()
	}
	p.ctime = ctime
	ctimeLock.Unlock()
	return ctime
}
