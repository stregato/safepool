package pool

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/code-to-go/safepool/core"
	"github.com/code-to-go/safepool/security"
	"github.com/code-to-go/safepool/transport"

	"github.com/godruoyi/go-snowflake"
)

const SafeConfigFile = ".safepool-pool.json"

var ErrNoExchange = errors.New("no Exchange available")
var ErrInvalidSignature = errors.New("signature is invalid")
var ErrNotTrusted = errors.New("the author is not a trusted user")
var ErrNotAuthorized = errors.New("no authorization for this file")
var ErrAlreadyExist = errors.New("pool already exists")
var ErrInvalidToken = errors.New("provided token is invalid: missing name or configs")
var ErrInvalidId = errors.New("provided id not a valid ed25519 public key")
var ErrInvalidConfig = errors.New("provided config is invalid: missing name or configs")

type Consumer interface {
	TimeOffset(s *Pool) time.Time
	Accept(s *Pool, h Feed) bool
}

type Pool struct {
	Name    string            `json:"name"`
	Self    security.Identity `json:"self"`
	Apps    []string          `json:"apps"`
	Trusted bool              `json:"trusted"`

	e                transport.Exchanger
	exchangers       []transport.Exchanger
	masterKeyId      uint64
	masterKey        []byte
	lastHouseKeeping time.Time
	accessHash       []byte
	config           Config
	// houseKeeping     *time.Ticker
	houseKeepingLock sync.Mutex
	// stopHouseKeeping chan bool
}

type Identity struct {
	security.Identity
	//Since is the keyId used when the identity was added to the Pool access
	Since uint64
	//AddedOn is the timestamp when the identity is stored on the local DB
	AddedOn time.Time
}

type Feed struct {
	Id        uint64
	Name      string
	Size      int64
	Hash      []byte
	ModTime   time.Time
	AuthorId  string
	Signature []byte
	Meta      []byte
	Offset    int    `json:"-"`
	Slot      string `json:"-"`
}

const (
	ID_CREATE       = 0x0
	ID_FORCE_CREATE = 0x1
)

var ForceCreation = false
var ReplicaPeriod = time.Hour
var CacheSizeMB = 16
var FeedDateFormat = "20060102"

type Config struct {
	Name    string
	Public  []string
	Private []string
}

func List() []string {
	names, _ := sqlListPool()
	return names
}

func Create(self security.Identity, name string, apps []string) (*Pool, error) {
	config, err := sqlGetPool(name)
	if core.IsErr(err, "unknown pool %s: %v", name) {
		return nil, err
	}

	p := &Pool{
		Name:             name,
		Self:             self,
		lastHouseKeeping: core.Now(),
		config:           config,
	}
	err = p.connectSafe(config)
	if err != nil {
		return nil, err
	}

	p.masterKeyId = snowflake.ID()
	p.masterKey = security.GenerateBytesKey(32)
	err = p.sqlSetKey(p.masterKeyId, p.masterKey)
	if core.IsErr(err, "çannot store master encryption key to db: %v") {
		return nil, err
	}

	access := Access{
		Id:      self.Id(),
		State:   Active,
		ModTime: core.Now(),
	}
	err = p.sqlSetAccess(access)
	if core.IsErr(err, "cannot link identity to pool '%s': %v", p.Name) {
		return nil, err
	}

	if !ForceCreation {
		_, err = p.e.Stat(path.Join(p.Name, ".access"))
		if err == nil {
			return nil, ErrAlreadyExist
		}
	}

	err = p.syncIdentities()
	if core.IsErr(err, "cannot sync own identity: %v") {
		return nil, err
	}

	err = p.exportAccessFile()
	if core.IsErr(err, "cannot export access file for pool '%s': %v", name) {
		return nil, err
	}

	return p, err
}

// Init initialized a domain on the specified exchangers
func Open(self security.Identity, name string) (*Pool, error) {
	config, err := sqlGetPool(name)
	if core.IsErr(err, "unknown pool %s: %v", name) {
		return nil, err
	}
	p := &Pool{
		Name:   name,
		Self:   self,
		config: config,
	}
	err = p.connectSafe(config)
	if err != nil {
		return nil, err
	}

	err = p.syncIdentities()
	if core.IsErr(err, "cannot sync own identity: %v") {
		return nil, err
	}

	_, err = p.sync(p.e)

	//	p.startHouseKeeping()
	return p, err
}

type AcceptFunc func(feed Feed)

const All = ""

func (p *Pool) List(offset int) ([]Feed, error) {
	hs, err := sqlGetFeeds(p.Name, offset)
	if core.IsErr(err, "cannot read Pool feeds: %v") {
		return nil, err
	}
	return hs, err
}

func (p *Pool) Send(name string, r io.Reader, meta []byte) (Feed, error) {
	id := snowflake.ID()
	slot := core.Now().Format(FeedDateFormat)
	n := path.Join(p.Name, FeedsFolder, slot, fmt.Sprintf("%d.body", id))
	hr, err := p.writeFile(n, r)
	if core.IsErr(err, "cannot post file %s to %s: %v", name, p.e) {
		return Feed{}, err
	}

	hash := hr.Hash()
	signature, err := security.Sign(p.Self, hash)
	if core.IsErr(err, "cannot sign file %s.body in %s: %v", name, p.e) {
		return Feed{}, err
	}
	f := Feed{
		Id:        id,
		Name:      name,
		Size:      hr.Size(),
		Hash:      hash,
		ModTime:   core.Now(),
		AuthorId:  p.Self.Id(),
		Signature: signature,
		Meta:      meta,
		Slot:      slot,
	}
	data, err := json.Marshal(f)
	if core.IsErr(err, "cannot marshal header to json: %v") {
		return Feed{}, err
	}

	n = path.Join(p.Name, FeedsFolder, slot, fmt.Sprintf("%d.head", id))
	_, err = p.writeFile(n, bytes.NewBuffer(data))
	core.IsErr(err, "cannot write header %s.head in %s: %v", name, p.e)

	return f, nil
}

func (p *Pool) Receive(id uint64, rang *transport.Range, w io.Writer) error {
	f, err := sqlGetFeed(p.Name, id)
	if core.IsErr(err, "cannot retrieve %d from pool %v: %v", id, p) {
		return err
	}

	bodyName := path.Join(p.Name, FeedsFolder, f.Slot, fmt.Sprintf("%d.body", id))
	cached, err := p.getFromCache(bodyName, rang, w)
	if cached {
		return err
	}
	cw, err := p.cacheWriter(bodyName, w)
	if err == nil {
		defer cw.Close()
		w = cw
	}

	hr, err := p.readFile(bodyName, rang, w)
	if core.IsErr(err, "cannot read body '%s': %v", bodyName) {
		return err
	}
	hash := hr.Hash()
	if !bytes.Equal(hash, f.Hash) {
		return ErrInvalidSignature
	}

	return nil
}

func (p *Pool) CreateBranch(sub string, ids []string, apps []string) (Config, error) {
	var name string
	parts := strings.Split(p.Name, "/")
	if len(parts) > 2 && parts[len(parts)-2] == "branches" {
		name = fmt.Sprintf("%s/%s", strings.Join(parts[0:len(parts)-2], "/"), sub)
	} else {
		name = fmt.Sprintf("%s/branches/%s", p.Name, sub)
	}

	c := Config{
		Name:    name,
		Public:  p.config.Public,
		Private: p.config.Private,
	}

	err := Define(c)
	if core.IsErr(err, "cannot define Forked pool %s: %v", name) {
		return Config{}, err
	}

	p2, err := Create(p.Self, name, apps)
	if core.IsErr(err, "cannot create Forked pool %s: %v", name) {
		return Config{}, err
	}
	defer p2.Close()

	for _, id := range ids {
		p2.SetAccess(id, Active)
	}

	return c, nil
}

func (p *Pool) Close() {
	// if p.houseKeeping != nil {
	// 	p.stopHouseKeeping <- true
	// 	p.houseKeepingLock.Lock()
	// 	defer p.houseKeepingLock.Unlock()
	// }

	for _, e := range p.exchangers {
		_ = e.Close()
	}
}

func (p *Pool) Delete() error {
	for _, e := range p.exchangers {
		err := e.Delete(p.Name)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Pool) Identities() ([]security.Identity, error) {
	identities, _, err := p.sqlGetAccesses(false)
	return identities, err
}

func (p *Pool) SetAccess(userId string, state State) error {
	_, ok, _ := security.GetIdentity(userId)
	if !ok {
		identity, err := security.IdentityFromId(userId)
		if core.IsErr(err, "id '%s' is invalid: %v") {
			return err
		}
		identity.Nick = "❓ Incognito..."
		err = security.SetIdentity(identity)
		if core.IsErr(err, "cannot save identity '%s' to db: %v", identity) {
			return err
		}
	}

	err := p.sqlSetAccess(Access{
		Id:      userId,
		State:   state,
		ModTime: core.Now(),
	})
	if core.IsErr(err, "cannot link identity '%s' to pool '%s': %v", userId, p.Name) {
		return err
	}

	return p.exportAccessFile()
}

func (p *Pool) ToString() string {
	return fmt.Sprintf("%s [%v]", p.Name, p.e)
}
