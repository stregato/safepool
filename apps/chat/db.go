package chat

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/code-to-go/safepool/core"
	"github.com/code-to-go/safepool/sql"
)

func sqlSetMessage(pool, chat string, id uint64, author string, privateId string, m Message) error {
	message, err := json.Marshal(m)
	if core.IsErr(err, "cannot marshal chat message: %v") {
		return err
	}

	_, err = sql.Exec("SET_CHAT_MESSAGE", sql.Args{"pool": pool, "chat": chat, "id": id, "author": author, "privateId": privateId,
		"message": message, "time": sql.EncodeTime(m.Time)})
	core.IsErr(err, "cannot set message %d on db: %v", id)
	return err
}

func sqlGetMessages(pool, chat string, after, before time.Time, privateId string, limit int) ([]Message, error) {
	var messages []Message
	rows, err := sql.Query("GET_CHAT_MESSAGES", sql.Args{"pool": pool, "chat": chat, "after": sql.EncodeTime(after), "before": sql.EncodeTime(before), "privateId": privateId,
		"limit": limit})
	if err == nil {
		for rows.Next() {
			var data []byte
			var m Message
			err = rows.Scan(&data)
			if !core.IsErr(err, "cannot read message from db: %v", err) {
				err = json.Unmarshal(data, &m)
				if err == nil {
					messages = append(messages, m)
				}
			}
		}
	}

	return messages, err
}

func sqlReset(pool string) error {
	_, err := sql.Exec("DELETE_CHAT", sql.Args{"pool": pool})
	return err
}

func sqlListPrivates(pool string, chat string) ([]Private, error) {
	var privates []Private
	rows, err := sql.Query("GET_CHAT_PRIVATES", sql.Args{"pool": pool, "chat": chat})
	if core.IsErr(err, "cannot read privates of %s/%s from db: %v", pool, chat) {
		return nil, err
	}
	for rows.Next() {
		var userIds string

		err = rows.Scan(&userIds)
		if core.IsErr(err, "cannot read private of %s/%s from db: %v", pool, chat) {
			return nil, err
		}
		privates = append(privates, strings.Split(userIds, ":"))
	}
	return privates, err
}
