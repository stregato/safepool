package pool

import (
	"bytes"
	"encoding/json"

	"github.com/code-to-go/safepool/core"
	"github.com/code-to-go/safepool/security"
	"github.com/code-to-go/safepool/transport"
)

// func (p *Pool) list(prefix string, offset int) ([]Head, error) {
// 	hs, err := sqlGetHeads(p.Name, prefix, offset)
// 	if core.IsErr(err, "cannot read Pool heads: %v") {
// 		return nil, err
// 	}
// 	return hs, err
// }

func (p *Pool) readHead(e transport.Exchanger, name string) (Feed, error) {
	var b bytes.Buffer
	_, err := p.readFile(e, name, nil, &b)
	if core.IsErr(err, "cannot read header of %s in %s: %v", name, e) {
		return Feed{}, err
	}

	var h Feed
	err = json.Unmarshal(b.Bytes(), &h)
	if core.IsErr(err, "corrupted header for file %s", name) {
		return Feed{}, err
	}

	if !security.Verify(h.AuthorId, h.Hash, h.Signature) {
		return Feed{}, ErrNoExchange
	}

	return h, err
}
