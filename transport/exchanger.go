package transport

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

// Exchanger is a low level interface to storage services such as S3 or SFTP
type Exchanger interface {

	//Touched returns true when some data has been written to the exchanger since the last time Touched was called
	Touched(name string) bool

	// Read reads data from a file into a writer
	Read(name string, rang *Range, dest io.Writer) error

	// Write writes data to a file name. An existing file is overwritten
	Write(name string, source io.Reader) error

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

// NewExchanger creates a new exchanger giving a provided configuration
func NewExchanger(connectionUrl string) (Exchanger, error) {
	switch {
	case strings.HasPrefix(connectionUrl, "sftp://"):
		return NewSFTP(connectionUrl)
	case strings.HasPrefix(connectionUrl, "s3://"):
		return NewS3(connectionUrl)
	case strings.HasPrefix(connectionUrl, "file:/"):
		return NewLocal(connectionUrl)
	}

	return nil, core.ErrNoDriver
}
