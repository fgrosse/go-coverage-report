package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReport_Markdown(t *testing.T) {
	oldCov, err := ParseCoverage("testdata/01-old-coverage.txt", nil)
	require.NoError(t, err)

	newCov, err := ParseCoverage("testdata/01-new-coverage.txt", nil)
	require.NoError(t, err)

	changedFiles, err := ParseChangedFiles("testdata/01-changed-files.json", "github.com/fgrosse/prioqueue")
	require.NoError(t, err)

	report := NewReport(oldCov, newCov, changedFiles)
	actual := report.Markdown()

	expected := `### Merging this branch will **decrease** overall coverage

| Impacted Packages | Coverage Δ | :robot: |
|-------------------|------------|---------|
| github.com/fgrosse/prioqueue | 90.20% (**-9.80%**) | :thumbsdown: |
| github.com/fgrosse/prioqueue/foo/bar | 0.00% (ø) |  |

---

<details>

<summary>Coverage by file</summary>

### Changed files (no unit tests)

| Changed File | Coverage Δ | Total | Covered | Missed | :robot: |
|--------------|------------|-------|---------|--------|---------|
| github.com/fgrosse/prioqueue/foo/bar/baz.go | 0.00% (ø) | 0 | 0 | 0 |  |
| github.com/fgrosse/prioqueue/min_heap.go | 80.77% (**-19.23%**) | 52 (+2) | 42 (-8) | 10 (+10) | :skull:  |

_Please note that the "Total", "Covered", and "Missed" counts above refer to ***code statements*** instead of lines of code. The value in brackets refers to the test coverage of that file in the old version of the code._

</details>`
	assert.Equal(t, expected, actual)
}

func TestReport_Markdown_OnlyChangedUnitTests(t *testing.T) {
	oldCov, err := ParseCoverage("testdata/02-old-coverage.txt", nil)
	require.NoError(t, err)

	newCov, err := ParseCoverage("testdata/02-new-coverage.txt", nil)
	require.NoError(t, err)

	changedFiles, err := ParseChangedFiles("testdata/02-changed-files.json", "github.com/fgrosse/prioqueue")
	require.NoError(t, err)

	report := NewReport(oldCov, newCov, changedFiles)
	actual := report.Markdown()

	expected := `### Merging this branch will **increase** overall coverage

| Impacted Packages | Coverage Δ | :robot: |
|-------------------|------------|---------|
| github.com/fgrosse/prioqueue | 99.02% (**+8.82%**) | :thumbsup: |

---

<details>

<summary>Coverage by file</summary>

### Changed unit test files

- github.com/fgrosse/prioqueue/min_heap_test.go

</details>`
	assert.Equal(t, expected, actual)
}
