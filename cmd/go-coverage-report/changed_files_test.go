package main

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExcludeFiles(t *testing.T) {
	files := []string{
		"example.com/demo/calc/calc.go",
		"example.com/demo/integration/integration_test.go",
	}

	cases := map[string]struct {
		exclude  *regexp.Regexp
		expected []string
	}{
		"no exclude": {
			exclude:  nil,
			expected: files,
		},
		"exclude matching one file": {
			exclude:  regexp.MustCompile(`integration/`),
			expected: []string{"example.com/demo/calc/calc.go"},
		},
		"exclude matching all files": {
			exclude:  regexp.MustCompile(`^example\.com/demo/`),
			expected: []string{},
		},
		"exclude matching no file": {
			exclude:  regexp.MustCompile(`vendor/`),
			expected: files,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			actual := excludeFiles(files, c.exclude)
			assert.Equal(t, c.expected, actual)
		})
	}

	assert.Len(t, files, 2, "input slice must not be modified")
}
