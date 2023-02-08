package pool

type Header struct {
	Version float32 `json:"v"`
	Thread  uint64  `json:"t"`
	Key     uint64  `json:"k"`
	Hash    []byte  `json:"h"`
	Name    string  `json:"s"`
}
