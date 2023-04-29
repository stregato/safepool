package storage

import (
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path"

	"github.com/code-to-go/safepool/core"
	"github.com/studio-b12/gowebdav"
)

type WebDAV struct {
	c   *gowebdav.Client
	p   string
	url string
}

func OpenWebDAV(connectionUrl string) (Storage, error) {
	u, err := url.Parse(connectionUrl)
	if core.IsErr(err, "invalid url '%s': %v", connectionUrl) {
		return nil, err
	}

	var conn string
	switch u.Scheme {
	case "dav":
		conn = fmt.Sprintf("http://%s:%s/%s", u.Host, u.Port(), u.Path)
	case "davs":
		conn = fmt.Sprintf("https://%s:%s/%s", u.Host, u.Port(), u.Path)
	default:
		core.IsErr(os.ErrInvalid, "invalid scheme %s: %v", u.Scheme)
		return nil, os.ErrInvalid
	}

	password, _ := u.User.Password()
	c := gowebdav.NewClient(conn, u.User.Username(), password)
	err = c.Connect()
	if core.IsErr(err, "cannot connect to WebDAV '%s': %v", connectionUrl) {
		return nil, err
	}

	w := &WebDAV{
		c:   c,
		p:   u.Path,
		url: connectionUrl,
	}

	return w, nil
}

func (w *WebDAV) Read(name string, rang *Range, dest io.Writer, progress chan int64) error {
	p := path.Join(w.p, name)

	r, err := w.c.ReadStream(p)
	if core.IsErr(err, "cannot read WebDAV file %s: %v", p) {
		return err
	}

	var written int64
	if rang == nil {
		for err == nil {
			written, err = io.CopyN(dest, r, 1024*1024)
			if progress != nil {
				progress <- written
			}
		}
	} else {
		written, err = io.CopyN(io.Discard, r, rang.From)
		if core.IsErr(err, "cannot discard %n bytes in range GET on %s: %v", rang.From, p) {
			return err
		}
		if written != rang.From {
			core.IsErr(io.ErrShortWrite, "Cannot read %d bytes in GET %s: %v")
			return io.ErrShortWrite
		}

		written, err = io.CopyN(dest, r, rang.To-rang.From)
		if progress != nil {
			progress <- written
		}
	}
	if err != nil && err != io.EOF {
		core.IsErr(err, "cannot read from GET response on %s: %v", p)
		return err
	}
	r.Close()
	return nil
}

func (w *WebDAV) Write(name string, source io.ReadSeeker, size int64, progress chan int64) error {
	p := path.Join(w.p, name)

	err := w.c.WriteStream(p, source, 0)
	if core.IsErr(err, "cannot write WebDAV file %s: %v", p) {
		return err
	}

	return nil
}

func (w *WebDAV) ReadDir(dir string, opts ListOption) ([]fs.FileInfo, error) {
	p := path.Join(w.p, dir)

	ls, err := w.c.ReadDir(p)
	if _, ok := err.(*fs.PathError); ok {
		return nil, os.ErrNotExist
	}
	if core.IsErr(err, "cannot read WebDAV folder %s: %v", p) {
		return nil, err
	}
	return ls, err
}

func (w *WebDAV) Stat(name string) (fs.FileInfo, error) {
	p := path.Join(w.p, name)

	f, err := w.c.Stat(p)
	if _, ok := err.(*fs.PathError); ok {
		return nil, os.ErrNotExist
	}

	if core.IsErr(err, "cannot read WebDAV folder %s: %v", p) {
		return nil, err
	}
	return f, err
}

func (w *WebDAV) Rename(old, new string) error {
	o := path.Join(w.p, old)
	n := path.Join(w.p, new)
	return w.c.Rename(o, n, true)
}

func (w *WebDAV) Delete(name string) error {
	p := path.Join(w.p, name)
	return w.c.RemoveAll(p)
}

func (w *WebDAV) Close() error {
	return nil
}

func (w *WebDAV) String() string {
	return w.url
}
