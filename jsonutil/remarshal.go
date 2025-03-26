package jsonutil

import (
	"encoding/json"
	"fmt"
)

func Remarshal(v1, v2 any) error {
	b1, err := json.Marshal(v1)
	if err != nil {
		return fmt.Errorf("marshal v1: %w", err)
	}
	if err := json.Unmarshal(b1, v2); err != nil {
		return fmt.Errorf("unmarshal v2: %w", err)
	}
	return nil
}
