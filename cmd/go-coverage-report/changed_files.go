package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func ParseChangedFiles(filename, prefix string) ([]string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var files []string
	err = json.Unmarshal(data, &files)
	if err != nil {
		return nil, err
	}

	for i, file := range files {
		files[i] = filepath.Join(prefix, file)
	}

	return files, nil
}
