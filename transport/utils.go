package transport

import (
	"bytes"
	"encoding/json"
	"hash"
	"io"
)

func ReadFile(e Exchanger, name string) ([]byte, error) {
	var b bytes.Buffer
	err := e.Read(name, nil, &b)
	return b.Bytes(), err
}

func WriteFile(e Exchanger, name string, data []byte) error {
	b := bytes.NewBuffer(data)
	return e.Write(name, b)
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
		err = e.Write(name, bytes.NewBuffer(b))
	}
	return err
}

func CopyFile(dest Exchanger, destName string, source Exchanger, sourceName string) error {
	pr, pw := io.Pipe()
	defer pr.Close()
	var err error
	go func() {
		err = source.Read(sourceName, nil, pw)
		pw.Close()
	}()

	err2 := dest.Write(destName, pr)
	if err != nil {
		return err
	} else {
		return err2
	}

}
