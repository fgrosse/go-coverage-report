package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

func ParseChangedFiles(filename, prefix, projectPath string) ([]string, error) {
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
		fileAtProject := strings.Replace(file, projectPath, "", 1)
		files[i] = filepath.Join(prefix, fileAtProject)
	}

	return files, nil
}
