package config

import "os"

func load(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func save(path string, b []byte) error {
	err := os.WriteFile(path, b, 0o644)
	if err != nil {
		return err
	}
	return nil
}
