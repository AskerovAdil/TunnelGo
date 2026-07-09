package xray

import (
	"encoding/json"
	"path/filepath"
)

func jsonUnmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func jsonMarshalIndent(v any) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}

func dirOf(path string) string {
	return filepath.Dir(path)
}
