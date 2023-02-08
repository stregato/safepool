package pool

import (
	"io"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	"github.com/code-to-go/safe/safepool/core"
	"github.com/code-to-go/safe/safepool/transport"
)

var CachePath string

type CacheWriter struct {
	w       io.Writer
	f       *os.File
	name    string
	failed  bool
	started bool
}

func (p *Pool) getCachePath(name string) (string, error) {
	name = filepath.Join("safepool", p.Self.Id(), p.Name, filepath.FromSlash(name))
	if CachePath == "" {
		return xdg.CacheFile(name)
	} else {
		name = filepath.Join(CachePath, name)
		err := os.MkdirAll(filepath.Dir(name), 0755)
		return name, err
	}
}

func (p *Pool) getFromCache(name string, rang *transport.Range, w io.Writer) (bool, error) {
	if CacheSizeMB == 0 {
		return false, nil
	}
	name, err := p.getCachePath(name)
	if core.IsErr(err, "cannot create folder for cache file '%s': %v", name) {
		return false, nil
	}

	stat, err := os.Stat(name)
	if err != nil {
		return false, nil
	}

	f, err := os.Open(name)
	if err != nil {
		return false, nil
	}
	defer f.Close()

	if rang == nil {
		_, err = io.Copy(w, f)
	} else {
		f.Seek(rang.From, 0)
		to := rang.To
		if to > stat.Size() {
			to = stat.Size()
		}
		_, err = io.CopyN(w, f, to-rang.From)
	}

	return true, err
}

func (p *Pool) cacheWriter(name string, w io.Writer) (*CacheWriter, error) {
	if CacheSizeMB == 0 {
		return nil, io.ErrShortBuffer
	}

	name, err := p.getCachePath(name)
	if core.IsErr(err, "cannot create folder for cache file '%s': %v", name) {
		return nil, err
	}

	f, err := os.Create(name)
	if core.IsErr(err, "cannot create cache file '%s': %v", name) {
		return nil, err
	}

	return &CacheWriter{
		w:    w,
		f:    f,
		name: name,
	}, nil
}

func (c *CacheWriter) Write(p []byte) (n int, err error) {
	c.started = true
	_, err = c.f.Write(p)
	c.failed = c.failed || err != nil
	return c.w.Write(p)
}

func (c *CacheWriter) Close() {
	c.f.Close()
	if c.failed || !c.started {
		os.Remove(c.name)
	}
}
