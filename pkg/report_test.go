package coverage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReport_Markdown(t *testing.T) {
	oldCov, err := Parse("testdata/old-coverage.txt")
	require.NoError(t, err)

	newCov, err := Parse("testdata/new-coverage.txt")
	require.NoError(t, err)

	changedFiles, err := ParseChangedFiles("github.com/fgrosse/prioqueue", "testdata/changed-files.json")
	require.NoError(t, err)

	report := NewReport(oldCov, newCov, changedFiles)
	actual := report.Markdown()

	expected := `### Merging this branch will **decrease** overall coverage

| Impacted Packages | Coverage Δ | :robot: |
|-------------------|------------|---------|
| github.com/fgrosse/prioqueue | 90.20% (**-9.80%**) | :thumbsdown: |
| github.com/fgrosse/prioqueue/foo/bar | 0.00% (ø) |  |
`
	assert.Equal(t, expected, actual)
}
