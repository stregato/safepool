package duologue

import (
	"time"

	"github.com/code-to-go/safepool/apps/chat"
	"github.com/code-to-go/safepool/pool"
)

type Duologue struct {
	Pool *pool.Pool
	Name string
}

func (d Duologue) GetMessages(peerId string, before, after time.Time) ([]chat.Message, error) {
	return nil, nil
}

func (d Duologue) AddMessage(peerId string, message chat.Message) error {
	return nil
}
