package library

import (
	"bytes"
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/code-to-go/safepool/core"
	pool "github.com/code-to-go/safepool/pool"
	"github.com/code-to-go/safepool/security"
	"github.com/code-to-go/safepool/services/common"
	"github.com/wailsapp/mimetype"
)

var HashChainMaxLength = 128

var Auto = ""

type State int

const (
	Sync State = 1 << iota
	Updated
	Modified
	Deleted
	Conflict
	New
)

// File includes information about a file stored on the library. Most information refers on the synchronized state with the exchange.
type File struct {
	Name        string    `json:"name"`
	Id          uint64    `json:"id"`
	ModTime     time.Time `json:"modTime"`
	Size        uint64    `json:"size"`
	AuthorId    string    `json:"authorId"`
	ContentType string    `json:"contentType"`
	Hash        []byte    `json:"hash"`
	HashChain   [][]byte  `json:"hashChain"`
	Tags        []string  `json:"tags"`
	CTime       int64
}

type Local struct {
	Id        uint64    `json:"id"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	AuthorId  string    `json:"authorId"`
	ModTime   time.Time `json:"modTime"`
	Size      uint64    `json:"size"`
	Hash      []byte    `json:"hash"`
	HashChain [][]byte  `json:"hashChain"`
}

type Version struct {
	AuthorId    string    `json:"authorId"`
	State       State     `json:"state"`
	Size        uint64    `json:"size"`
	ModTime     time.Time `json:"modTime"`
	ContentType string    `json:"contentType"`
	Hash        []byte    `json:"hash"`
	Tags        []string  `json:"tags"`
	Id          uint64    `json:"id"`
}

type Document struct {
	Name      string    `json:"name"`
	AuthorId  string    `json:"authorId"`
	LocalPath string    `json:"localPath"`
	Id        uint64    `json:"id"`
	ModTime   time.Time `json:"modTime"`
	State     State     `json:"state"`
	Hash      []byte    `json:"hash"`
	HashChain [][]byte  `json:"hashChain"`
	Versions  []Version `json:"versions"`
}

type List struct {
	Folder     string     `json:"folder"`
	Documents  []Document `json:"documents"`
	Subfolders []string   `json:"subfolders"`
}

type Library struct {
	Pool *pool.Pool
	Name string
}

type meta struct {
	ContentType string   `json:"contentType"`
	HashChain   [][]byte `json:"history"`
	Tags        []string `json:"tags"`
}

// Get returns a library app mounted on the provided path in the pool
func Get(p *pool.Pool, name string) Library {
	return Library{
		Pool: p,
		Name: name,
	}
}

func containHash(hashChain [][]byte, hash []byte) bool {
	for _, h := range hashChain {
		if bytes.Equal(hash, h) {
			return true
		}
	}
	return false
}

func (l *Library) getStateForLocal(lo Local) State {
	stat, err := os.Stat(lo.Path)
	if err != nil {
		return Deleted
	}

	diff := stat.ModTime().Sub(lo.ModTime)
	if diff < time.Second {
		return Sync
	}

	h, _ := security.FileHash(lo.Path)
	if !bytes.Equal(h, lo.Hash) {
		return Modified
	}

	lo.ModTime = stat.ModTime()
	sqlSetLocal(l.Pool.Name, l.Name, lo)

	return Sync
}

func (l *Library) getDocuments(files []File, locals []Local) ([]Document, error) {
	m := map[string]Document{}

	for _, lo := range locals {
		m[lo.Name] = Document{
			Name:      lo.Name,
			LocalPath: lo.Path,
			AuthorId:  lo.AuthorId,
			State:     l.getStateForLocal(lo),
			Hash:      lo.Hash,
			HashChain: lo.HashChain,
			Id:        lo.Id,
			ModTime:   lo.ModTime,
		}
	}

	for _, f := range files {
		d, ok := m[f.Name]
		if !ok {
			d = Document{
				Name:      f.Name,
				State:     New,
				LocalPath: "",
			}
		}
		v := Version{
			AuthorId:    f.AuthorId,
			Size:        f.Size,
			ModTime:     f.ModTime,
			ContentType: f.ContentType,
			Hash:        f.Hash,
			Tags:        f.Tags,
			Id:          f.Id,
		}

		switch {
		case d.LocalPath == "" || d.State == Deleted:
			v.State = Updated
		case f.Id == d.Id:
			continue
		case bytes.Equal(f.Hash, d.Hash):
			continue
		case containHash(f.HashChain, d.Hash):
			if d.State == Modified {
				v.State = Conflict
				d.State = Conflict
			} else {
				v.State = Updated
				d.State = Updated
			}
		case containHash(d.HashChain, f.Hash):
			continue
		default:
			v.State = Conflict
			d.State = Conflict
		}
		d.Versions = append(d.Versions, v)
		m[d.Name] = d
	}

	var documents []Document
	for _, d := range m {
		documents = append(documents, d)
	}
	sort.Slice(documents, func(i, j int) bool {
		return documents[i].Name < documents[j].Name
	})
	return documents, nil
}

// List returns the documents in provided folder
func (l *Library) List(folder string) (List, error) {
	l.Pool.Sync()
	ctime := common.GetBreakpoint(l.Pool.Name, l.Name)
	fs, _ := l.Pool.List(ctime)
	for _, f := range fs {
		l.accept(f)
		ctime = f.CTime
	}
	common.SetBreakpoint(l.Pool.Name, l.Name, ctime)

	subfolders, err := sqlGetSubfolders(l.Pool.Name, l.Name, folder)
	if core.IsErr(err, "cannot list subfolders in %s/%s/%s: %v", l.Pool.Name, l.Name, folder) {
		return List{}, err
	}
	files, err := sqlFilesInFolder(l.Pool.Name, l.Name, folder)
	if core.IsErr(err, "cannot list documents in %s/%s/%s: %v", l.Pool.Name, l.Name, folder) {
		return List{}, err
	}
	locals, err := sqlGetLocalsInFolder(l.Pool.Name, l.Name, folder)
	if core.IsErr(err, "cannot list locals in %s/%s/%s: %v", l.Pool.Name, l.Name, folder) {
		return List{}, err
	}
	documents, err := l.getDocuments(files, locals)
	if core.IsErr(err, "cannot join locals and files in %s/%s/%s: %v", l.Pool.Name, l.Name, folder) {
		return List{}, err
	}

	return List{
		Folder:     folder,
		Subfolders: subfolders,
		Documents:  documents,
	}, nil
}

func (l *Library) Save(id uint64, dest string) error {
	f, err := os.Create(dest)
	if core.IsErr(err, "cannot create '%s': %v", dest) {
		return err
	}
	defer f.Close()

	err = l.Pool.Receive(id, nil, f)
	if core.IsErr(err, "cannot get file with id %d: %v", id) {
		return err
	}
	return nil
}

func (l *Library) Find(id uint64) (File, error) {
	l.Pool.Sync()
	f, ok, err := sqlGetFileById(l.Pool.Name, l.Name, id)
	if core.IsErr(err, "cannot get document with id '%d': %v", id) {
		return File{}, err
	}
	if !ok {
		return File{}, core.ErrInvalidId
	}
	return f, nil
}

func (l *Library) Receive(id uint64, localPath string) (File, error) {
	os.MkdirAll(filepath.Dir(localPath), 0755)

	lf, err := os.Create(localPath + ".tmp")
	if core.IsErr(err, "cannot create '%s': %v", localPath) {
		return File{}, err
	}

	f, ok, err := sqlGetFileById(l.Pool.Name, l.Name, id)
	if core.IsErr(err, "cannot get document with id '%d': %v", id) {
		return File{}, err
	}
	if !ok {
		return File{}, core.ErrInvalidId
	}

	err = l.Pool.Receive(id, nil, lf)
	lf.Close()
	if core.IsErr(err, "cannot get file with id %d: %v", id) {
		os.Remove(localPath + ".tmp")
		return File{}, err
	}
	err = os.Rename(localPath+".tmp", localPath)
	if core.IsErr(err, "cannot overwrite old file %s: %v", localPath) {
		return File{}, err
	}

	stat, _ := os.Stat(localPath)
	lo := Local{
		Id:        id,
		Name:      f.Name,
		Path:      localPath,
		AuthorId:  f.AuthorId,
		ModTime:   stat.ModTime(),
		Size:      uint64(stat.Size()),
		Hash:      f.Hash,
		HashChain: f.HashChain,
	}

	err = sqlSetLocal(l.Pool.Name, l.Name, lo)
	if core.IsErr(err, "cannot update document for id %d: %v", id) {
		return File{}, err
	}
	return f, nil
}

func (l *Library) Delete(id uint64) error {
	return nil
}

func (l *Library) GetLocalPath(name string) (string, bool) {
	lo, ok, _ := sqlGetLocal(l.Pool.Name, l.Name, name)
	if ok {
		return lo.Path, true
	} else {
		return "", false
	}
}

// Send uploads the specified file localPath to the pool with the provided name. When solveConflicts is true
// the
func (l *Library) Send(localPath string, name string, solveConflicts bool, tags ...string) (File, error) {
	mime, err := mimetype.DetectFile(localPath)
	if core.IsErr(err, "cannot detect mime type of '%s': %v", localPath) {
		return File{}, err
	}

	stat, _ := os.Stat(localPath)

	var hashChain [][]byte
	lo, ok, err := sqlGetLocal(l.Pool.Name, l.Name, name)
	if core.IsErr(err, "db error in reading document %s: %v", name) {
		return File{}, err
	}
	if solveConflicts {
		hashChain, err = sqlGetFilesHashes(l.Pool.Name, l.Name, name, HashChainMaxLength)
		if core.IsErr(err, "cannot get hashes for file %s: %v", name) {
			return File{}, err
		}
	} else if ok {
		hashChain = append(lo.HashChain, lo.Hash)
		if len(hashChain) > HashChainMaxLength {
			hashChain = hashChain[len(hashChain)-HashChainMaxLength:]
		}
	}

	m, err := json.Marshal(meta{
		ContentType: mime.String(),
		Tags:        tags,
		HashChain:   hashChain,
	})
	if core.IsErr(err, "cannot marshal metadata to json: %v") {
		return File{}, err
	}

	f, err := os.Open(localPath)
	if core.IsErr(err, "cannot open '%s': %v", localPath) {
		return File{}, err
	}
	h, err := l.Pool.Send(path.Join(l.Name, name), f, stat.Size(), m)
	if core.IsErr(err, "cannot post content to pool '%s': %v", l.Pool.Name) {
		return File{}, err
	}

	l.Pool.Sync()
	lo = Local{
		Id:        h.Id,
		Name:      name,
		Path:      localPath,
		ModTime:   stat.ModTime(),
		Size:      uint64(stat.Size()),
		AuthorId:  h.AuthorId,
		Hash:      h.Hash,
		HashChain: hashChain,
	}
	err = sqlSetLocal(l.Pool.Name, l.Name, lo)
	return File{
		Name:        h.Name,
		Id:          h.Id,
		ModTime:     h.ModTime,
		Size:        uint64(h.Size),
		AuthorId:    h.AuthorId,
		ContentType: mime.String(),
		Hash:        h.Hash,
		Tags:        tags,
	}, err
}

func (l *Library) accept(feed pool.Head) {
	if !strings.HasPrefix(feed.Name, l.Name+"/") {
		return
	}

	var m meta
	err := json.Unmarshal(feed.Meta, &m)
	if core.IsErr(err, "invalid meta in feed: %v") {
		return
	}
	name := feed.Name[len(l.Name)+1:]

	f := File{
		Id:          feed.Id,
		Name:        name,
		ModTime:     feed.ModTime,
		Size:        uint64(feed.Size),
		AuthorId:    feed.AuthorId,
		ContentType: m.ContentType,
		CTime:       feed.CTime,
		Hash:        feed.Hash,
		HashChain:   m.HashChain,
	}

	err = sqlSetDocument(l.Pool.Name, l.Name, f)
	core.IsErr(err, "cannot save document to db: %v")
}

// Reset removes all the local content
func (l *Library) Reset() error {
	return sqlReset(l.Pool.Name, l.Name)
}
