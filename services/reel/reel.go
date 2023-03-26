package reel

import (
	"bytes"
	"encoding/json"
	"image/jpeg"
	"io"
	"os"
	"path"
	"time"

	"github.com/bakape/thumbnailer/v2"
	"github.com/code-to-go/safepool/core"
	"github.com/code-to-go/safepool/pool"
	"github.com/wailsapp/mimetype"
)

type Head struct {
	Id          uint64    `json:"id,string"`
	Name        string    `json:"name"`
	Author      string    `json:"author"`
	Time        time.Time `json:"time"`
	ContentType string    `json:"contentType"`
	Thumbnail   []byte    `json:"thumbnail"`
}

type Reel struct {
	Pool *pool.Pool
	Name string
}

type ReelPart []byte

func (r *Reel) ListThreads() ([]string, error) {
	return sqlListThreads(r.Pool.Name, r.Name)
}

func (r *Reel) List(thread string, from, to time.Time, limit int) ([]Head, error) {
	return sqlListReel(r.Pool.Name, r.Name, thread, from, to, limit)
}

func (r *Reel) Receive(id uint64, from, to int) error {
	// r.Pool.Receive(id, &storage.Range{})
	return nil
}

type meta struct {
	ContentType string `json:"contentType"`
	Thumbnail   []byte `json:"thumbnail"`
}

func (r *Reel) Send(thread string, name string) error {
	mime, _ := mimetype.DetectFile(name)

	f, err := os.Open(name)
	if core.IsErr(err, "cannot open file '%s': %v", name) {
		return err
	}

	t, _ := thumbnail(f, 128, 128)
	m, err := json.Marshal(meta{
		ContentType: mime.String(),
		Thumbnail:   t,
	})
	if core.IsErr(err, "cannot encode metadata for '%s': %v", name) {
		return err
	}
	f.Seek(0, 0)

	fs, _ := f.Stat()
	go func() {
		_, err = r.Pool.Send(path.Join(r.Name, thread, name), f, fs.Size(), m)
		core.IsErr(err, "cannot send file to reel %s/%s/%s: %v", r.Pool, r.Name, thread)
	}()

	return nil
}

func thumbnail(r io.ReadSeeker, w, h uint) ([]byte, error) {
	ctx, err := thumbnailer.NewFFContext(r)
	if core.IsErr(err, "cannot create FF context for preview: %v") {
		return nil, err
	}

	img, err := ctx.Thumbnail(thumbnailer.Dims{Width: w, Height: h})
	if core.IsErr(err, "cannot create thumbnail for preview: %v") {
		return nil, err
	}

	var buf bytes.Buffer
	err = jpeg.Encode(&buf, img, nil)
	if core.IsErr(err, "cannot encode preview to Jpeg: %v") {
		return nil, err
	}
	return buf.Bytes(), nil
}
