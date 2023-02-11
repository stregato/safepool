package registry

import (
	"encoding/json"

	"github.com/code-to-go/safepool/core"
	"github.com/code-to-go/safepool/sql"
)

func sqlSetInvite(pool string, ctime int64, i Invite) error {
	content, err := json.Marshal(i)
	if core.IsErr(err, "cannot marshal invite: %v") {
		return err
	}

	_, err = sql.Exec("SET_INVITE", sql.Args{"pool": pool, "ctime": ctime,
		"valid":   i.Config != nil,
		"content": content})
	if core.IsErr(err, "cannot set invite on db: %v") {
		return err
	}

	return err
}

func sqlGetInvites(pool string, since int64, mustBeValid bool) ([]Invite, error) {
	var q string
	if mustBeValid {
		q = "GET_INVITES_VALID"
	} else {
		q = "GET_INVITES"
	}

	rows, err := sql.Query(q, sql.Args{"pool": pool, "since": since})
	if core.IsErr(err, "cannot query invites from db: %v") {
		return nil, err
	}
	var invites []Invite
	for rows.Next() {
		var content []byte
		err = rows.Scan(&content)
		if !core.IsErr(err, "cannot scan row in Invites: %v", err) {
			var i Invite
			err = json.Unmarshal(content, &i)
			if err == nil {
				invites = append(invites, i)
			}
		}
	}
	return invites, nil
}

func sqlGetCTime(pool string) int64 {
	var ctime int64
	err := sql.QueryRow("GET_INDEX_CTIME", sql.Args{"pool": pool}, &ctime)
	if err == nil {
		return ctime
	} else {
		return -1
	}
}
