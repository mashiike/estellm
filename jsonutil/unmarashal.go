package jsonutil

import (
	"bytes"
	"encoding/json"
)

func UnmarshalFirstJSON(bs []byte, v any) error {
	var err error
	for i := range bs {
		dec := json.NewDecoder(bytes.NewReader(bs[i:]))
		err = dec.Decode(v)
		if err == nil {
			return nil
		}
	}
	return err
}
