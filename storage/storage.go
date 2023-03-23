package storage

import (
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/code-to-go/safepool/core"
)

type Source struct {
	Name   string
	Data   []byte
	Reader io.Reader
	Size   int64
}

const SizeAll = -1

type ListOption uint32

const (
	// IncludeHiddenFiles includes hidden files in a list operation
	IncludeHiddenFiles ListOption = 1
)

type Range struct {
	From int64
	To   int64
}

// Storage is a low level interface to storage services such as S3 or SFTP
type Storage interface {
	// Read reads data from a file into a writer
	Read(name string, rang *Range, dest io.Writer, progress chan int64) error

	// Write writes data to a file name. An existing file is overwritten
	Write(name string, source io.ReadSeeker, size int64, progress chan int64) error

	//ReadDir returns the entries of a folder content
	ReadDir(name string, opts ListOption) ([]fs.FileInfo, error)

	// Stat provides statistics about a file
	Stat(name string) (os.FileInfo, error)

	// Rename a file. Overwrite an existing file if present
	Rename(old, new string) error

	// Delete deletes a file
	Delete(name string) error

	// Close releases resources
	Close() error

	// String returns a human-readable representation of the storer (e.g. sftp://user@host.cc/path)
	String() string
}

// OpenStorage creates a new exchanger giving a provided configuration
func OpenStorage(connectionUrl string) (Storage, error) {
	switch {
	case strings.HasPrefix(connectionUrl, "sftp://"):
		return OpenSFTP(connectionUrl)
	case strings.HasPrefix(connectionUrl, "s3://"):
		return OpenS3(connectionUrl)
	case strings.HasPrefix(connectionUrl, "file:/"):
		return OpenLocal(connectionUrl)
	}

	return nil, core.ErrNoDriver
}
