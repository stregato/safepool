package chat

import (
	"encoding/json"
	"time"

	"github.com/code-to-go/safepool/core"
	"github.com/code-to-go/safepool/sql"
)

func sqlSetMessage(pool string, id uint64, author string, m Message) error {
	message, err := json.Marshal(m)
	if core.IsErr(err, "cannot marshal chat message: %v") {
		return err
	}

	_, err = sql.Exec("SET_CHAT_MESSAGE", sql.Args{"pool": pool, "id": id, "author": author, "message": message, "time": sql.EncodeTime(m.Time)})
	core.IsErr(err, "cannot set message %d on db: %v", id)
	return err
}

func sqlGetMessages(pool string, after, before time.Time, limit int) ([]Message, error) {
	var messages []Message
	rows, err := sql.Query("GET_CHAT_MESSAGES", sql.Args{"pool": pool, "after": sql.EncodeTime(after), "before": sql.EncodeTime(before), "limit": limit})
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
