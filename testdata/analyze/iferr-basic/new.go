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
	c, err := parse(b)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return c, nil
}
