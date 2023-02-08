package transport

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"time"

	"os"
	"path"

	"github.com/code-to-go/safepool/core"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type SFTPConfig struct {
	Addr     string `json:"addr" yaml:"addr"`
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
	KeyPath  string `json:"keyPath" yaml:"keyPath"`
	Base     string `json:"base" yaml:"base"`
}

type SFTP struct {
	c     *sftp.Client
	base  string
	url   string
	touch map[string]time.Time
}

func ParseSFTPUrl(s string) (SFTPConfig, error) {
	u, err := url.Parse(s)
	if err != nil {
		return SFTPConfig{}, err
	}

	password, _ := u.User.Password()
	return SFTPConfig{
		Addr:     u.Host,
		Username: u.User.Username(),
		Password: password,
		Base:     u.Path,
	}, nil
}

func ToUrl(config SFTPConfig) string {
	return fmt.Sprintf("sftp://%s@%s/%s", config.Username, config.Addr, config.Base)
}

// NewSFTP create a new Exchanger. The url is in the format sftp://
func NewSFTP(connectionUrl string) (Exchanger, error) {
	u, err := url.Parse(connectionUrl)
	if err != nil {
		return nil, err
	}

	addr := u.Host
	if u.Port() == "" {
		addr = fmt.Sprintf("%s:22", addr)
	}

	params := u.Query()

	var repr string
	var auth []ssh.AuthMethod

	password, hasPassword := u.User.Password()
	if hasPassword {
		auth = append(auth, ssh.Password(password))
		repr = fmt.Sprintf("sftp://%s@%s/%s", u.User.Username(), addr, u.Path)
	}

	if key := params.Get("key"); key != "" {
		pkey, err := base64.StdEncoding.DecodeString(key)
		if core.IsErr(err, "private key is invalid: %v") {
			return nil, err
		}

		signer, err := ssh.ParsePrivateKey(pkey)
		if err != nil {
			return nil, fmt.Errorf("invalid key: %v", err)
		}
		auth = append(auth, ssh.PublicKeys(signer))
		repr = fmt.Sprintf("sftp://PKEY@%s/%s", addr, u.Path)
	}

	if len(auth) == 0 {
		return nil, fmt.Errorf("no auth method provided for sftp connection to %s", addr)
	}

	cc := &ssh.ClientConfig{
		User:            u.User.Username(),
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp", addr, cc)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to %s: %v", addr, err)
	}
	c, err := sftp.NewClient(client)
	if err != nil {
		return nil, fmt.Errorf("cannot create a sftp client for %s: %v", addr, err)
	}

	base := u.Path
	if base == "" {
		base = "/"
	}
	return &SFTP{c, base, repr, map[string]time.Time{}}, nil
}

func (s *SFTP) Touched(name string) bool {
	touchFile := path.Join(s.base, fmt.Sprintf("%s.touch", name))
	stat, err := s.Stat(touchFile)
	if err != nil {
		return true
	}
	if stat.ModTime().After(s.touch[name]) {
		if !core.IsErr(s.Write(touchFile, &bytes.Buffer{}), "cannot write touch file: %v") {
			s.touch[name] = core.Now()
		}
		return true
	}
	return false
}

func (s *SFTP) Read(name string, rang *Range, dest io.Writer) error {
	f, err := s.c.Open(path.Join(s.base, name))
	if core.IsErr(err, "cannot open file on sftp server %v:%v", s) {
		return err
	}

	if rang == nil {
		_, err = io.Copy(dest, f)
	} else {
		left := rang.To - rang.From
		f.Seek(rang.From, 0)
		var b [4096]byte

		for left > 0 && err == nil {
			var sz int64
			if rang.To-rang.From > 4096 {
				sz = 4096
			} else {
				sz = rang.To - rang.From
			}
			n, err := f.Read(b[0:sz])
			dest.Write(b[0:n])
			left -= int64(n)
			if err != nil {
				break
			}
		}
	}
	if err != io.EOF && core.IsErr(err, "cannot read from %s/%s:%v", s, name) {
		return err
	}

	return nil
}

func (s *SFTP) Write(name string, source io.Reader) error {
	name = path.Join(s.base, name)

	f, err := s.c.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC)
	if os.IsNotExist(err) {
		dir := path.Dir(name)
		s.c.MkdirAll(dir)
		f, err = s.c.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC)
	}
	if core.IsErr(err, "cannot create SFTP file '%s': %v", name) {
		return err
	}

	_, err = io.Copy(f, source)
	core.IsErr(err, "cannot write SFTP file '%s': %v", name)
	return err
}

func (s *SFTP) ReadDir(dir string, opts ListOption) ([]fs.FileInfo, error) {
	dir = path.Join(s.base, dir)
	infos, err := s.c.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	return infos, nil
}

func (s *SFTP) Stat(name string) (os.FileInfo, error) {
	return s.c.Stat(path.Join(s.base, name))
}

func (s *SFTP) Rename(old, new string) error {
	return s.c.Rename(path.Join(s.base, old), path.Join(s.base, new))
}

func (s *SFTP) Delete(name string) error {
	return s.c.Remove(path.Join(s.base, name))
}

func (s *SFTP) Close() error {
	return s.c.Close()
}

func (s *SFTP) String() string {
	return s.url
}
