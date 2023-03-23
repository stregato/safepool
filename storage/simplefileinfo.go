package storage

import (
	"io/fs"
	"time"
)

type simpleFileInfo struct {
	name    string
	size    int64
	isDir   bool
	modTime time.Time
}

func (f simpleFileInfo) Name() string {
	return f.name
}

func (f simpleFileInfo) Size() int64 {
	return f.size
}

func (f simpleFileInfo) Mode() fs.FileMode {
	return 0644
}

func (f simpleFileInfo) ModTime() time.Time {
	return f.modTime
}

func (f simpleFileInfo) IsDir() bool {
	return f.isDir
}

func (f simpleFileInfo) Sys() interface{} {
	return nil
}
