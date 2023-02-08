package core

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

var ErrNotInitialized = fmt.Errorf("safepool not initialized")
var ErrNoDriver = fmt.Errorf("no driver found for the provided configuration")
var ErrInvalidSignature = fmt.Errorf("signature does not match the user id")
var ErrInvalidSize = fmt.Errorf("provided slice has not enough data")
var ErrInvalidVersion = fmt.Errorf("version of protocol is not compatible")
var ErrInvalidChangeFilePath = fmt.Errorf("a change file is not in a valid Safepool folder")
var ErrInvalidFilePath = fmt.Errorf("a file is not in a valid Safepool folder")
var ErrNoExchange = fmt.Errorf("no exchange reachable for the domain")
var ErrNotAuthorized = fmt.Errorf("user is not authorized in the domain")
var ErrInvalidId = fmt.Errorf("the id is invalid")

func IsErr(err error, msg string, args ...interface{}) bool {

	if err != nil {
		args = append(args, err)
		logrus.Warnf(msg, args...)
		return true
	}
	return false
}

func FatalIf(err error, msg string, args ...interface{}) {
	if err != nil {
		args = append(args, err)
		logrus.Fatalf(msg, args...)
		panic(err)
	}
}
