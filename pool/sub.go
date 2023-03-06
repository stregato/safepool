package pool

import (
	"fmt"
	"strings"

	"github.com/code-to-go/safepool/core"
)

func (p *Pool) Sub(sub string, ids []string, apps []string) (Config, error) {
	var name string
	parts := strings.Split(p.Name, "/")
	if len(parts) > 2 && parts[len(parts)-2] == "@" {
		name = fmt.Sprintf("%s/%s", strings.Join(parts[0:len(parts)-2], "/"), sub)
	} else {
		name = fmt.Sprintf("%s/@/%s", p.Name, sub)
	}

	c := Config{
		Name:    name,
		Public:  p.config.Public,
		Private: p.config.Private,
	}

	err := Define(c)
	if core.IsErr(err, "cannot define Forked pool %s: %v", name) {
		return Config{}, err
	}

	p2, err := Create(p.Self, name, apps)
	if core.IsErr(err, "cannot create Forked pool %s: %v", name) {
		return Config{}, err
	}
	defer p2.Close()

	for _, id := range ids {
		p2.SetAccess(id, Active)
	}

	return c, nil
}
