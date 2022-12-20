package openapi

import "encoding/json"

type encoding struct {
	marshal   func(any) ([]byte, error)
	unmarshal func([]byte, any) error
}

type encoder map[string]encoding

func (e encoder) get(mime string) (encoding, bool) {
	enc, ok := e[mime]
	return enc, ok
}

func (e encoder) has(mime string) bool {
	_, ok := e.get(mime)
	return ok
}

var serde = encoder{
	"application/json": encoding{
		marshal:   json.Marshal,
		unmarshal: json.Unmarshal,
	},
}
