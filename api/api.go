package api

import (
	_ "embed"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/code-to-go/safepool/apps/chat"
	"github.com/code-to-go/safepool/apps/invite"
	"github.com/code-to-go/safepool/apps/library"
	"github.com/code-to-go/safepool/core"
	"github.com/code-to-go/safepool/pool"
	"github.com/code-to-go/safepool/security"
	"github.com/code-to-go/safepool/sql"
	"github.com/patrickmn/go-cache"
)

var Apps = []string{
	"chat",
	"library",
}

var pools *cache.Cache
var Self security.Identity

//go:embed sqlite.sql
var sqlliteDDL string

//go:embed fish.png
var defaultAvatar []byte

// SetDbPath set the full path where the DB will be created. Useful on Android/iOS platforms
func SetDbPath(dbPath string) {
	sql.DbPath = dbPath
}

// SetDbName set the name of the DB file. Useful for test purpose
func SetDbName(dbName string) {
	sql.DbPath = dbName
}

func Start(dbPath string, availableBandwith pool.Bandwidth) error {
	sql.InitDDL = sqlliteDDL
	pool.AvailableBandwidth = availableBandwith

	pools = cache.New(time.Hour, time.Hour)
	pools.OnEvicted(func(name string, p interface{}) {
		core.Info("closing pool %s", name)
		p.(*pool.Pool).Close()
	})

	err := sql.OpenDB(dbPath)
	if core.IsErr(err, "cannot open DB: %v") {
		return err
	}

	s, _, _, ok := sql.GetConfig("", "SELF")
	if ok {
		Self, err = security.IdentityFromBase64(s)
	} else {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		name := fmt.Sprintf("%s%d", names[r.Intn(len(names))], r.Intn(100))
		identity, err := security.NewIdentity(name)
		identity.Avatar = defaultAvatar
		if core.IsErr(err, "cannot create identity:%v") {
			return err
		}
		err = setSelf(identity)
		if core.IsErr(err, "cannot update self:%v") {
			return err
		}

	}

	return err
}

func Stop() error {
	pools.Flush()
	err := sql.CloseDB()
	core.IsErr(err, "cannot close db '%s': %v", sql.DbPath)
	return err
}

func FactoryReset() error {
	err := Stop()
	core.IsErr(err, "cannot stop application: %v")

	err = sql.DeleteDB()
	if core.IsErr(err, "cannot delete db '%s': %v", sql.DbPath) {
		return err
	}

	err = Start(sql.DbPath, pool.AvailableBandwidth)
	core.IsErr(err, "cannot start application: %v")
	return err
}

func setSelf(identity security.Identity) error {
	s, err := identity.Base64()
	if err == nil {
		err = sql.SetConfig("", "SELF", s, 0, nil)
	}
	if core.IsErr(err, "cannot save identity to db: %v") {
		panic("cannot save identity in db")
	}

	err = security.SetIdentity(identity)
	if core.IsErr(err, "cannot save identity to db: %v") {
		panic("cannot save identity in db")
	}

	err = security.Trust(identity, true)
	if core.IsErr(err, "cannot set trust of '%s' on db: %v", identity.Nick) {
		panic("cannot trust Self in db")
	}
	Self = identity

	go func() {
		for _, name := range pool.List() {
			p, err := pool.Open(Self, name)
			if err == nil {
				p.ExportSelf(true)
				p.Close()
			}
		}
	}()

	return nil
}

func SetSelf(identity security.Identity) error {
	if identity.Id() != Self.Id() {
		Stop()
		setSelf(identity)
		return Start(sql.DbPath, pool.AvailableBandwidth)
	} else {
		return setSelf(identity)
	}
}

func PoolCreate(c pool.Config, apps []string) error {
	err := pool.Define(c)
	if core.IsErr(err, "cannot define pool %v: %s", c.Name) {
		return err
	}

	p, err := pool.Create(Self, c.Name, apps)
	if core.IsErr(err, "cannot create pool %v: %s", c.Name) {
		return err
	}
	p.Close()
	return nil
}

// PoolJoin adds a pool by using the provided invite token
func PoolJoin(token string) (pool.Config, error) {
	i, err := invite.Decode(Self, token)
	if core.IsErr(err, "invalid token: %v") {
		return pool.Config{}, err
	}

	if i.Storages == nil {
		return pool.Config{}, core.ErrNotAuthorized
	}
	PoolLeave(i.Name)
	err = i.Join()
	if core.IsErr(err, "cannot join pool '%s': %v", i.Name) {
		return pool.Config{}, err
	}

	return pool.Config{Name: i.Name, Public: i.Storages}, nil
}

func PoolLeave(name string) error {
	p, err := PoolGet(name)
	if core.IsErr(err, "cannot get pool '%s'", name) {
		return err
	}

	c := chat.Get(p, "chat")
	c.Reset()

	l := library.Get(p, "library")
	l.Reset()

	pools.Delete(name)
	return p.Leave()
}

func PoolGet(name string) (*pool.Pool, error) {
	v, ok := pools.Get(name)
	if ok {
		return v.(*pool.Pool), nil
	}

	p, err := pool.Open(Self, name)
	if core.IsErr(err, "cannot open pool '%s': %v", name) {
		return nil, err
	}

	pools.Add(name, p, time.Hour)
	return p, nil
}

func PoolSub(name string, sub string, ids []string, apps []string) (string, error) {
	p, err := PoolGet(name)
	if core.IsErr(err, "cannot get pool '%s': %v", name) {
		return "", err
	}

	c, err := p.Sub(sub, ids, apps)
	if core.IsErr(err, "cannot sub pool '%s': %v", name) {
		return "", err
	}

	i := invite.Invite{
		Sender:       p.Self,
		Name:         c.Name,
		Storages:     c.Public,
		RecipientIds: ids,
	}

	token, err := invite.Encode(i)
	if core.IsErr(err, "cannot create token: %v") {
		return "", err
	}
	return token, nil
}

func PoolInvite(name string, ids []string, invitePool string) (string, error) {
	p, err := PoolGet(name)
	if core.IsErr(err, "cannot get pool '%s' for invite", name) {
		return "", err
	}

	var t *pool.Pool
	if invitePool != "" {
		t, err = PoolGet(invitePool)
		if core.IsErr(err, "cannot get invite pool '%s'", name) {
			return "", err
		}
	}

	c, err := pool.GetConfig(name)
	if core.IsErr(err, "cannot read config for '%s': %v", name) {
		return "", err
	}

	for _, id := range ids {
		err = p.SetAccess(id, pool.Active)
		if core.IsErr(err, "cannot set access for id '%s' in pool '%s': %v", id, p.Name) {
			return "", err
		}
	}
	p.SyncAccess()

	i := invite.Invite{
		Sender:       p.Self,
		Name:         c.Name,
		Storages:     c.Public,
		RecipientIds: ids,
	}

	token, err := invite.Encode(i)
	if core.IsErr(err, "cannot create token: %v") {
		return "", err
	}

	if t != nil {
		invite.Add(t, i)
	}
	return token, err
}

// PoolParseInvite checks the validity of the provided invite token and returns the token object
func PoolParseInvite(token string) (invite.Invite, error) {
	i, err := invite.Decode(Self, token)
	if core.IsErr(err, "invalid token: %v") {
		return invite.Invite{}, err
	}
	return i, err
}

func PoolUsers(poolName string) ([]security.Identity, error) {
	p, err := PoolGet(poolName)
	if core.IsErr(err, "cannot get pool '%s' for chat app", poolName) {
		return nil, err
	}

	return p.Users()
}

func getChatName(private chat.Private) string {
	if len(private) == 0 {
		return "chat"
	} else {
		return "private"
	}
}

func ChatPrivates(poolName string) ([]chat.Private, error) {
	p, err := PoolGet(poolName)
	if core.IsErr(err, "cannot get pool '%s' for chat app", poolName) {
		return nil, err
	}

	ch := chat.Get(p, "private")
	return ch.Privates()
}

func ChatReceive(poolName string, after, before time.Time, limit int, private chat.Private) ([]chat.Message, error) {
	p, err := PoolGet(poolName)
	if core.IsErr(err, "cannot get pool '%s' for chat app", poolName) {
		return nil, err
	}

	ch := chat.Get(p, getChatName(private))
	return ch.Receive(after, before, limit, private)
}

func ChatSend(poolName string, contentType string, text string, bytes []byte, private chat.Private) (uint64, error) {
	p, err := PoolGet(poolName)
	if core.IsErr(err, "cannot get pool '%s' for chat app", poolName) {
		return 0, err
	}

	c := chat.Get(p, getChatName(private))
	id, err := c.SendMessage(contentType, text, bytes, private)
	if core.IsErr(err, "cannot post chat message: %v") {
		return 0, err
	}
	return id, nil
}

func LibraryList(poolName string, folder string) (library.List, error) {
	p, err := PoolGet(poolName)
	if core.IsErr(err, "cannot get pool '%s' for chat app", poolName) {
		return library.List{}, err
	}

	l := library.Get(p, "library")
	ls, err := l.List(folder)
	if core.IsErr(err, "cannot list folder '%s' in pool '%s': %v", folder, poolName) {
		return library.List{}, err
	}
	return ls, err
}

func LibraryFind(poolName string, id uint64) (library.File, error) {
	p, err := PoolGet(poolName)
	if core.IsErr(err, "cannot get pool '%s' for chat app", poolName) {
		return library.File{}, err
	}

	l := library.Get(p, "library")
	return l.Find(id)
}

func LibraryReceive(poolName string, id uint64, localPath string) error {
	p, err := PoolGet(poolName)
	if core.IsErr(err, "cannot get pool '%s' for chat app", poolName) {
		return err
	}

	l := library.Get(p, "library")
	_, err = l.Receive(id, localPath)
	return err
}

func LibrarySave(poolName string, id uint64, dest string) error {
	p, err := PoolGet(poolName)
	if core.IsErr(err, "cannot get pool '%s' for chat app", poolName) {
		return err
	}

	l := library.Get(p, "library")
	err = l.Save(id, dest)
	return err
}

func LibrarySend(poolName string, localPath string, name string, solveConflicts bool, tags ...string) (library.File, error) {
	p, err := PoolGet(poolName)
	if core.IsErr(err, "cannot get pool '%s' for library app", poolName) {
		return library.File{}, err
	}
	l := library.Get(p, "library")
	f, err := l.Send(localPath, name, solveConflicts, tags...)
	return f, err
}

func InviteReceive(poolName string, after int64, onlyMine bool) ([]invite.Invite, error) {
	p, err := PoolGet(poolName)
	if core.IsErr(err, "cannot get pool '%s' for invite app", poolName) {
		return nil, err
	}
	return invite.Receive(p, after, onlyMine)
}

type Notification struct {
	Pool    string `json:"pool"`
	App     string `json:"app"`
	Message string `json:"message"`
	Count   int    `json:"count"`
}

func Notifications(ctime int64) []Notification {
	var ns []Notification

	for _, name := range pool.List() {
		p, err := pool.Open(Self, name)
		if err != nil {
			continue
		}

		appsCount := map[string]int{}
		p.Sync()

		feeds, _ := p.List(ctime)
		for _, f := range feeds {
			parts := strings.SplitN(f.Name, "/", 2)
			if len(parts) == 2 {
				appsCount[parts[0]] += 1
			}
		}

		for app, count := range appsCount {
			ns = append(ns, Notification{
				Pool:  p.Name,
				App:   app,
				Count: count,
			})
		}
	}

	return ns
}

func Dump() map[string]any {
	m := map[string]any{}

	var poolsDumps []map[string]any
	for _, i := range pools.Items() {
		p := i.Object.(*pool.Pool)
		poolsDumps = append(poolsDumps, p.Dump())
	}
	m["pools"] = poolsDumps
	m["availableBandwidth"] = pool.AvailableBandwidth

	var logBuilder strings.Builder
	for _, l := range core.RecentLog {
		logBuilder.WriteString(l)
		logBuilder.WriteRune('\n')
	}
	m["log"] = logBuilder.String()
	return m
}
