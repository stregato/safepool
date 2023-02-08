package core

import (
	"time"

	"github.com/godruoyi/go-snowflake"
)

func init() {
	snowflake.SetStartTime(SnowFlakeStart)
}

var SnowFlakeStart = time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)

const UsersFilename = "U"
const UsersFilesign = "U.sign"
const DomainFilelock = "D.lock"
