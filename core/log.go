package core

import (
	"fmt"
	"path"
	"path/filepath"
	"runtime"

	"github.com/sirupsen/logrus"
)

func Info(format string, args ...any) {
	if logrus.GetLevel() >= logrus.InfoLevel {
		msg := fmt.Sprintf(format, args...)
		pc, file, no, ok := runtime.Caller(1)
		details := runtime.FuncForPC(pc)
		if ok && details != nil {
			msg = fmt.Sprintf("%s[%s:%d] - %s", path.Base(details.Name()), filepath.Base(file), no, msg)
		}
		RecentLog = append(RecentLog, fmt.Sprintf("INFO: %s", msg))
		logrus.Info(msg)
	}
}

func Debug(format string, args ...any) {
	if logrus.GetLevel() >= logrus.DebugLevel {
		msg := fmt.Sprintf(format, args...)
		pc, file, no, ok := runtime.Caller(1)
		details := runtime.FuncForPC(pc)
		if ok && details != nil {
			msg = fmt.Sprintf("%s[%s:%d] - %s", path.Base(details.Name()), filepath.Base(file), no, msg)
		}
		if len(RecentLog) >= MaxRecentErrors {
			RecentLog = RecentLog[1 : MaxRecentErrors-1]
		}
		RecentLog = append(RecentLog, fmt.Sprintf("DEBU: %s", msg))
		logrus.Debug(msg)
	}
}
