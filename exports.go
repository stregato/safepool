package main

/*
typedef struct Result{
    char* res;
	char* err;
} Result;

typedef struct App {
	void (*feed)(char* name, char* data, int eof);
} App;

#include <stdlib.h>
*/
import "C"
import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/code-to-go/safepool/api"
	"github.com/code-to-go/safepool/apps/chat"
	"github.com/code-to-go/safepool/core"
	"github.com/code-to-go/safepool/pool"
	"github.com/code-to-go/safepool/security"
	"github.com/sirupsen/logrus"
	"github.com/skratchdot/open-golang/open"
)

func cResult(v any, err error) C.Result {
	var res []byte

	if err != nil {
		return C.Result{nil, C.CString(err.Error())}
	}
	if v == nil {
		return C.Result{nil, nil}
	}

	res, err = json.Marshal(v)
	if err == nil {
		return C.Result{C.CString(string(res)), nil}
	}
	return C.Result{nil, C.CString(err.Error())}
}

func cInput(err error, i *C.char, v any) error {
	if err != nil {
		return err
	}
	data := C.GoString(i)
	return json.Unmarshal([]byte(data), v)
}

//export start
func start(dbPath *C.char, cachePath *C.char, availableBandwith *C.char) C.Result {
	var ab pool.Bandwidth
	p := C.GoString(dbPath)
	switch C.GoString(availableBandwith) {
	case "low":
		ab = pool.LowBandwidth
	case "medium":
		ab = pool.MediumBandwidth
	case "high":
		ab = pool.HighBandwith
	default:
		cResult(nil, fmt.Errorf("invalid bandwidth option %s. Valid options are low, medium, high", C.GoString(availableBandwith)))
	}
	pool.CachePath = C.GoString(cachePath)
	return cResult(nil, api.Start(p, ab))
}

//export stop
func stop() C.Result {
	err := api.Stop()
	return cResult(nil, err)
}

//export factoryReset
func factoryReset() C.Result {
	err := api.FactoryReset()
	return cResult(nil, err)
}

//export securitySelfId
func securitySelfId() C.Result {
	return cResult(api.Self.Id(), nil)
}

//export securityGetSelf
func securityGetSelf() C.Result {
	return cResult(api.Self, nil)
}

//export securitySetSelf
func securitySetSelf(identity *C.char) C.Result {
	var i security.Identity
	data := []byte(C.GoString(identity))
	err := json.Unmarshal(data, &i)
	if core.IsErr(err, "cannot unmarshal json from C: %v") {
		return cResult(nil, err)
	}
	return cResult(nil, api.SetSelf(i))
}

//export securityIdentityFromId
func securityIdentityFromId(id *C.char) C.Result {
	identity, err := security.IdentityFromId(C.GoString(id))
	return cResult(identity, err)
}

//export poolList
func poolList() C.Result {
	return cResult(pool.List(), nil)
}

//export poolCreate
func poolCreate(config *C.char, apps *C.char) C.Result {
	var c pool.Config
	var apps_ []string

	err := cInput(nil, config, &c)
	err = cInput(err, apps, &apps_)
	if err != nil {
		return cResult(nil, err)
	}

	err = api.PoolCreate(c, apps_)
	return cResult(nil, err)
}

//export poolJoin
func poolJoin(token *C.char) C.Result {
	c, err := api.PoolJoin(C.GoString(token))
	return cResult(c, err)
}

//export poolLeave
func poolLeave(name *C.char) C.Result {
	err := api.PoolLeave(C.GoString(name))
	return cResult(nil, err)
}

//export poolSub
func poolSub(name *C.char, sub *C.char, idsList *C.char, appsList *C.char) C.Result {
	p := C.GoString(name)
	s := C.GoString(sub)

	var ids []string
	err := json.Unmarshal([]byte(C.GoString(idsList)), &ids)
	if core.IsErr(err, "invalid ids list: %v") {
		return cResult(nil, err)
	}
	var apps []string
	err = json.Unmarshal([]byte(C.GoString(appsList)), &apps)
	if core.IsErr(err, "invalid apps list: %v") {
		return cResult(nil, err)
	}

	token, err := api.PoolSub(p, s, ids, apps)
	return cResult(token, err)
}

//export poolInvite
func poolInvite(poolName *C.char, idsList *C.char, invitePool *C.char) C.Result {
	p := C.GoString(poolName)
	ip := C.GoString(invitePool)

	var ids []string
	err := json.Unmarshal([]byte(C.GoString(idsList)), &ids)
	if core.IsErr(err, "invalid ids list: %v") {
		return cResult(nil, err)
	}
	token, err := api.PoolInvite(p, ids, ip)
	return cResult(token, err)
}

//export poolGet
func poolGet(name *C.char) C.Result {
	p, err := api.PoolGet(C.GoString(name))
	return cResult(p, err)
}

//export poolUsers
func poolUsers(poolName *C.char) C.Result {
	identities, err := api.PoolUsers(C.GoString(poolName))
	return cResult(identities, err)
}

//export poolParseInvite
func poolParseInvite(token *C.char) C.Result {
	i, err := api.PoolParseInvite(C.GoString(token))
	return cResult(i, err)
}

//export chatReceive
func chatReceive(poolName *C.char, after, before C.long, limit C.int, private *C.char) C.Result {
	var private_ chat.Private
	err := json.Unmarshal([]byte(C.GoString(private)), &private_)
	if core.IsErr(err, "cannot unmarshal private: %v") {
		return cResult(nil, err)
	}

	messages, err := api.ChatReceive(C.GoString(poolName), time.UnixMicro(int64(after)),
		time.UnixMicro(int64(before)), int(limit), private_)
	return cResult(messages, err)
}

//export chatSend
func chatSend(poolName *C.char, contentType *C.char, text *C.char, binary *C.char, private *C.char) C.Result {
	bs, err := base64.StdEncoding.DecodeString(C.GoString(binary))
	if core.IsErr(err, "invalid binary in message: %v") {
		return cResult(nil, err)
	}

	var private_ chat.Private
	err = json.Unmarshal([]byte(C.GoString(private)), &private_)
	if core.IsErr(err, "cannot unmarshal private: %v") {
		return cResult(nil, err)
	}

	id, err := api.ChatSend(C.GoString(poolName), C.GoString(contentType),
		C.GoString(text), bs, private_)
	if core.IsErr(err, "cannot post message: %v") {
		return cResult(nil, err)
	}
	return cResult(id, nil)
}

//export chatPrivates
func chatPrivates(poolName *C.char) C.Result {
	privates, err := api.ChatPrivates(C.GoString(poolName))
	return cResult(privates, err)
}

//export libraryList
func libraryList(poolName *C.char, folder *C.char) C.Result {
	p, f := C.GoString(poolName), C.GoString(folder)
	ls, err := api.LibraryList(p, f)
	if core.IsErr(err, "cannot read documents in folder '%s' in pool '%s': %v", p, f) {
		return cResult(nil, err)
	}
	return cResult(ls, nil)
}

//export libraryFind
func libraryFind(poolName *C.char, id C.long) C.Result {
	p := C.GoString(poolName)
	f, err := api.LibraryFind(p, uint64(id))
	return cResult(f, err)
}

//export libraryReceive
func libraryReceive(poolName *C.char, id C.long, localPath *C.char) C.Result {
	p, l := C.GoString(poolName), C.GoString(localPath)
	err := api.LibraryReceive(p, uint64(id), l)
	return cResult(nil, err)
}

//export librarySave
func librarySave(poolName *C.char, id C.long, localPath *C.char) C.Result {
	p, l := C.GoString(poolName), C.GoString(localPath)
	err := api.LibrarySave(p, uint64(id), l)
	return cResult(nil, err)
}

//export librarySend
func librarySend(poolName *C.char, localPath *C.char, name *C.char, solveConflicts C.int, tagsList *C.char) C.Result {
	p, l, n := C.GoString(poolName), C.GoString(localPath), C.GoString(name)
	var tags []string
	err := json.Unmarshal([]byte(C.GoString(tagsList)), &tags)
	if core.IsErr(err, "cannot unmarshal tags in addDocument C call: %v", err) {
		return cResult(nil, err)
	}

	f, err := api.LibrarySend(p, l, n, solveConflicts == 0, tags...)
	if core.IsErr(err, "cannot add document '%s' in pool '%s': %v", p, n) {
		return cResult(nil, err)
	}
	return cResult(f, nil)
}

//export inviteReceive
func inviteReceive(poolName *C.char, after, onlyMine C.int) C.Result {
	invites, err := api.InviteReceive(C.GoString(poolName), int64(after), onlyMine == 1)
	return cResult(invites, err)
}

//export notifications
func notifications(ctime C.long) C.Result {
	notifications := api.Notifications(int64(ctime))
	return cResult(notifications, nil)
}

//export fileOpen
func fileOpen(filePath *C.char) C.Result {
	return cResult(nil, open.Start(C.GoString(filePath)))
}

//export dump
func dump() C.Result {
	d := api.Dump()
	return cResult(d, nil)
}

//export setLogLevel
func setLogLevel(level C.int) C.Result {
	logrus.SetLevel(logrus.Level(level))
	core.Info("log level set to %d", level)
	return cResult(nil, nil)
}

//export setAvailableBandwidth
func setAvailableBandwidth(availableBandwidth *C.char) C.Result {
	switch C.GoString(availableBandwidth) {
	case "low":
		pool.AvailableBandwidth = pool.LowBandwidth
	case "medium":
		pool.AvailableBandwidth = pool.MediumBandwidth
	case "high":
		pool.AvailableBandwidth = pool.HighBandwith
	default:
		cResult(nil, fmt.Errorf("invalid bandwidth option %s. Valid options are low, medium, high", C.GoString(availableBandwidth)))
	}

	return cResult(nil, nil)
}
