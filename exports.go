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

	"github.com/code-to-go/safe/safepool/core"
	"github.com/code-to-go/safe/safepool/pool"
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
	return cResult(nil, safepool.Start(p))
}

//export stop
func stop() C.Result {
	return cResult(nil, nil)
}

//export getSelfId
func getSelfId() C.Result {
	return cResult(safepool.Self.Id(), nil)
}

//export getSelf
func getSelf() C.Result {
	return cResult(safepool.Self, nil)
}

//export getPoolList
func getPoolList() C.Result {
	return cResult(pool.List(), nil)
}

//export createPool
func createPool(config *C.char, apps *C.char) C.Result {
	var c pool.Config
	var apps_ []string

	err := cInput(nil, config, &c)
	err = cInput(err, apps, &apps_)
	if err != nil {
		return cResult(nil, err)
	}

	err = safepool.CreatePool(c, apps_)
	return cResult(nil, err)
}

//export addPool
func addPool(token *C.char) C.Result {
	c, err := safepool.AddPool(C.GoString(token))
	return cResult(c, err)
}

//export getPool
func getPool(name *C.char) C.Result {
	p, err := safepool.GetPool(C.GoString(name))
	return cResult(p, err)
}

//export getMessages
func getMessages(poolName *C.char, afterIdS, beforeIdS C.long, limit C.int) C.Result {
	messages, err := safepool.GetMessages(C.GoString(poolName), uint64(afterIdS),
		uint64(int64(beforeIdS)), int(limit))
	return cResult(messages, err)
}

//export postMessage
func postMessage(poolName *C.char, contentType *C.char, text *C.char, binary *C.char) C.Result {
	bs, err := base64.StdEncoding.DecodeString(C.GoString(binary))
	if core.IsErr(err, "invalid binary in message: %v") {
		return cResult(nil, err)
	}

	id, err := safepool.PostMessage(C.GoString(poolName), C.GoString(contentType),
		C.GoString(text), bs)
	if core.IsErr(err, "cannot post message: %v") {
		return cResult(nil, err)
	}
	return cResult(id, nil)
}
