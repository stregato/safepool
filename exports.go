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
	"time"

	"github.com/code-to-go/safepool/api"
	"github.com/code-to-go/safepool/core"
	"github.com/code-to-go/safepool/pool"
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
func start(dbPath *C.char) C.Result {
	p := C.GoString(dbPath)
	return cResult(nil, api.Start(p))
}

//export stop
func stop() C.Result {
	return cResult(nil, nil)
}

//export getSelfId
func getSelfId() C.Result {
	return cResult(api.Self.Id(), nil)
}

//export getSelf
func getSelf() C.Result {
	return cResult(api.Self, nil)
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
func chatReceive(poolName *C.char, after, before C.long, limit C.int) C.Result {
	messages, err := api.ChatReceive(C.GoString(poolName), time.UnixMicro(int64(after)),
		time.UnixMicro(int64(before)), int(limit))
	return cResult(messages, err)
}

//export chatSend
func chatSend(poolName *C.char, contentType *C.char, text *C.char, binary *C.char) C.Result {
	bs, err := base64.StdEncoding.DecodeString(C.GoString(binary))
	if core.IsErr(err, "invalid binary in message: %v") {
		return cResult(nil, err)
	}

	id, err := api.ChatSend(C.GoString(poolName), C.GoString(contentType),
		C.GoString(text), bs)
	if core.IsErr(err, "cannot post message: %v") {
		return cResult(nil, err)
	}
	return cResult(id, nil)
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

//export libraryReceive
func libraryReceive(poolName *C.char, id C.long, localPath *C.char) C.Result {
	p, l := C.GoString(poolName), C.GoString(localPath)
	err := api.LibraryReceive(p, uint64(id), l)
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

	err = api.LibrarySend(p, l, n, solveConflicts == 0, tags...)
	if core.IsErr(err, "cannot add document '%s' in pool '%s': %v", p, n) {
		return cResult(nil, err)
	}
	return cResult(nil, nil)
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
