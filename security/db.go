package security

import (
	"github.com/code-to-go/safepool/core"
	"github.com/code-to-go/safepool/sql"
)

func sqlSetIdentity(i Identity) error {
	i64, err := i.Base64()
	if core.IsErr(err, "cannot serialize identity: %v") {
		return err
	}

	_, err = sql.Exec("SET_IDENTITY", sql.Args{
		"id":  i.Id(),
		"i64": i64,
	})
	return err
}

func sqlGetIdentity(id string) (Identity, error) {
	var i64, alias string
	var identity Identity
	err := sql.QueryRow("GET_IDENTITY", sql.Args{"id": id}, &i64, &alias)
	if err == nil {
		identity, err = IdentityFromBase64(i64)
		if core.IsErr(err, "corrupted identity on db: %v") {
			return identity, err
		}
	}
	return identity, err
}

func sqlGetIdentities(onlyTrusted bool) ([]Identity, error) {
	var q string
	if onlyTrusted {
		q = "GET_TRUSTED"
	} else {
		q = "GET_IDENTITIES"
	}

	rows, err := sql.Query(q, sql.Args{})
	if core.IsErr(err, "cannot get trusted identities from db: %v") {
		return nil, err
	}
	defer rows.Close()

	var identities []Identity
	for rows.Next() {
		var i64 string
		var alias string
		err = rows.Scan(&i64, &alias)
		if core.IsErr(err, "cannot read pool feeds from db: %v") {
			continue
		}

		i, err := IdentityFromBase64(string(i64))
		if core.IsErr(err, "invalid identity record '%s': %v", i64) {
			continue
		}

		if alias != "" {
			i.Nick = alias
		}
		identities = append(identities, i)
	}
	return identities, nil
}

func sqlSetTrust(i Identity, trusted bool) error {
	_, err := sql.Exec("SET_TRUSTED", sql.Args{
		"id":      i.Id(),
		"trusted": trusted,
	})
	return err
}

func sqlSetAlias(i Identity, alias string) error {
	_, err := sql.Exec("SET_TRUSTED", sql.Args{
		"id":    i.Id(),
		"alias": alias,
	})
	return err
}
