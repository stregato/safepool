package transport

import (
	"bytes"
	"encoding/json"
	"hash"
	"io"

	"os"

	"github.com/code-to-go/safepool/core"
)

func ReadFile(e Exchanger, name string) ([]byte, error) {
	var b bytes.Buffer
	err := e.Read(name, nil, &b, nil)
	return b.Bytes(), err
}

func WriteFile(e Exchanger, name string, data []byte) error {
	b := core.NewBytesReader(data)
	return e.Write(name, b, int64(len(data)), nil)
}

func ReadJSON(e Exchanger, name string, v any, hash hash.Hash) error {
	data, err := ReadFile(e, name)
	if err == nil {
		if hash != nil {
			hash.Write(data)
		}

		err = json.Unmarshal(data, v)
	}
	return err
}

func WriteJSON(e Exchanger, name string, v any, hash hash.Hash) error {
	b, err := json.Marshal(v)
	if err == nil {
		if hash != nil {
			hash.Write(b)
		}
		err = e.Write(name, core.NewBytesReader(b), int64(len(b)), nil)
	}
	return err
}

const maxSizeForMemoryCopy = 1024 * 1024

func CopyFile(dest Exchanger, destName string, source Exchanger, sourceName string) error {
	stat, err := source.Stat(sourceName)
	if core.IsErr(err, "cannot stat %s/%s: %v", source, sourceName) {
		return err
	}

	var r io.ReadSeeker
	if stat.Size() <= maxSizeForMemoryCopy {
		buf := bytes.Buffer{}
		err = source.Read(sourceName, nil, &buf, nil)
		if core.IsErr(err, "cannot read %s/%s: %v", source, sourceName) {
			return err
		}
		r = core.NewBytesReader(buf.Bytes())
	} else {
		file, err := os.CreateTemp("", "safepool")
		if core.IsErr(err, "cannot create temporary file for CopyFile: %v") {
			return err
		}

		r = file
		defer func() {
			file.Close()
			os.Remove(file.Name())
		}()
	}

	err = dest.Write(destName, r, stat.Size(), nil)
	if core.IsErr(err, "cannot write %s/%s: %v", dest, destName) {
		return err
	}

	return nil
}
