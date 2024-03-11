package coverage

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func ParseChangedFiles(rootPackage, filename string) ([]string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var files []string
	err = json.Unmarshal(data, &files)
	if err != nil {
		return nil, err
	}

	for i, f := range files {
		files[i] = filepath.Join(rootPackage, f) // TODO: find a better way
	}

	return files, nil
}
