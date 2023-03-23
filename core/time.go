package core

import (
	"time"

	"github.com/beevik/ntp"
)

var NtpServers = []string{
	"time.google.com",
	"0.beevik-ntp.pool.ntp.org",
}

var NtpRetries = 10
var ClockOffset time.Duration
var timeIsSync bool
var timeTicker *time.Ticker

func init() {
	timeTicker = time.NewTicker(30 * time.Minute)
	go func() {
		for ; true; <-timeTicker.C {
			timeIsSync = syncTime()
		}
	}()
}

func syncTime() bool {
	for i := 0; i < NtpRetries; i++ {
		for _, s := range NtpServers {
			r, err := ntp.Query(s)
			if err == nil {
				ClockOffset = r.ClockOffset
				Info("clock offset %v from %s ", ClockOffset, s)
				return true
			}
		}
	}
	return false
}

func TimeIsSync() bool {
	if !timeIsSync {
		timeIsSync = syncTime()
	}
	return timeIsSync
}

func Now() time.Time {
	return time.Now().Add(ClockOffset)
}

func Since(t time.Time) time.Duration {
	return time.Since(t) + ClockOffset
}
