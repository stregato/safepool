package storage

import (
	"io"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/code-to-go/safepool/core"
)

type FirebaseConfig struct {
	Base string `json:"base" yaml:"base"`
}

type Firebase struct {
	base string
	url  string
}

func NewFirebase(connectionUrl string) (Storage, error) {
	u, err := url.Parse(connectionUrl)
	if core.IsErr(err, "invalid URL: %v") {
		return nil, err
	}

	base := u.Path
	if base == "" {
		base = "/"
	}
	return &Local{base, connectionUrl, map[string]time.Time{}}, nil
}

func (l *Firebase) GetCheckpoint(name string) int64 {
	stat, err := l.Stat(name)
	if err != nil {
		return -1
	}
	return stat.ModTime().UnixMicro()
}

func (l *Firebase) SetCheckpoint(name string) (int64, error) {
	err := l.Write(name, core.NewBytesReader(nil), 0, nil)
	if core.IsErr(err, "cannot write checkpoint '%s': %v", name) {
		return 0, err
	}
	return l.GetCheckpoint(name), nil
}

func (l *Firebase) Read(name string, rang *Range, dest io.Writer, progress chan int64) error {
	f, err := os.Open(path.Join(l.base, name))
	if core.IsErr(err, "cannot open file on %v:%v", l) {
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
			if rang.From-rang.To > 4096 {
				sz = 4096
			} else {
				sz = rang.From - rang.To
			}
			_, err = f.Read(b[0:sz])
			dest.Write(b[0:sz])
			left -= sz
		}
	}
	if core.IsErr(err, "cannot read from %s/%s:%v", l, name) {
		return err
	}

	return nil
}

func (l *Firebase) Write(name string, source io.ReadSeeker, size int64, progress chan int64) error {
	n := filepath.Join(l.base, name)
	err := createDir(n)
	if core.IsErr(err, "cannot create parent of %s: %v", n) {
		return err
	}

	f, err := os.Create(n)
	if core.IsErr(err, "cannot create file on %v:%v", l) {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, source)
	core.IsErr(err, "cannot copy file on %v:%v", l)
	return err
}

func (l *Firebase) ReadDir(dir string, opts ListOption) ([]fs.FileInfo, error) {
	result, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var infos []fs.FileInfo
	for _, item := range result {
		info, err := item.Info()
		if err == nil {
			infos = append(infos, info)
		}
	}

	return infos, nil
}

func (l *Firebase) Stat(name string) (os.FileInfo, error) {
	return os.Stat(path.Join(l.base, name))
}

func (l *Firebase) Rename(old, new string) error {
	return os.Rename(path.Join(l.base, old), path.Join(l.base, new))
}

func (l *Firebase) Delete(name string) error {
	return os.Remove(path.Join(l.base, name))
}

func (l *Firebase) Close() error {
	return nil
}

func (l *Firebase) String() string {
	return l.url
}
