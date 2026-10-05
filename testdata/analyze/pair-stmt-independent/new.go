package config

import (
	"fmt"
	"os"
)

func load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parse(b)
}

func save(path string, b []byte) error {
	err := os.WriteFile(path, b, 0o644)
	if err != nil {
		return fmt.Errorf("save %s: %w", path, err)
	}
	return nil
}
