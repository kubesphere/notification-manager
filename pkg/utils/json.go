package utils

import (
	"io"

	json "github.com/json-iterator/go"
)

func MapToStruct(mv map[string]interface{}, v interface{}) error {
	bs, err := json.Marshal(mv)
	if err != nil {
		return err
	}

	err = json.Unmarshal(bs, &v)
	if err != nil {
		return err
	}

	return nil
}

func JsonDecode(reader io.Reader, v interface{}) error {
	if err := json.NewDecoder(reader).Decode(v); err != nil {
		return err
	}

	return nil
}

func JsonEncode(writer io.Writer, v interface{}) error {
	if err := json.NewEncoder(writer).Encode(v); err != nil {
		return err
	}

	return nil
}

func JsonMarshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func JsonMarshalIndent(v interface{}, prefix, indent string) ([]byte, error) {
	return json.MarshalIndent(v, prefix, indent)
}

func JsonUnmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
