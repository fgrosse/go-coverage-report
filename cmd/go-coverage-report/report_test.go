package main

import (
	"os"
	"strings"
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

func TestWriteMetrics(t *testing.T) {
	oldCov, err := ParseCoverage("testdata/01-old-coverage.txt", nil)
	require.NoError(t, err)

	newCov, err := ParseCoverage("testdata/01-new-coverage.txt", nil)
	require.NoError(t, err)

	changedFiles, err := ParseChangedFiles("testdata/01-changed-files.json", "github.com/fgrosse/prioqueue")
	require.NoError(t, err)

	report := NewReport(oldCov, newCov, changedFiles)
	path := t.TempDir() + "/metrics.txt"

	err = report.WriteMetrics(path)
	require.NoError(t, err)

	content, err := os.ReadFile(path)
	require.NoError(t, err)

	expected := "total_coverage=90.20\ncoverage_delta=-9.80\ncoverage_trend=decreased\ntotal_statements=102\ncovered_statements=92\nmissed_statements=10\n"
	assert.Equal(t, expected, string(content))
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

// TestReport_Markdown_DeletedUnitTestFile reproduces
// https://github.com/fgrosse/go-coverage-report/issues/42 for the case where the
// deleted test file lives in the same directory (i.e. the same Go package) as the
// code it covers, which is the common, idiomatic layout for Go tests.
func TestReport_Markdown_DeletedUnitTestFile(t *testing.T) {
	oldCov, err := ParseCoverage("testdata/04-old-coverage.txt", nil)
	require.NoError(t, err)

	newCov, err := ParseCoverage("testdata/04-new-coverage.txt", nil)
	require.NoError(t, err)

	changedFiles, err := ParseChangedFiles("testdata/04-changed-files.json", "github.com/fgrosse/prioqueue")
	require.NoError(t, err)

	report := NewReport(oldCov, newCov, changedFiles)
	actual := report.Markdown()

	expected := `### Merging this branch will **decrease** overall coverage

| Impacted Packages | Coverage Δ | :robot: |
|-------------------|------------|---------|
| github.com/fgrosse/prioqueue | 90.20% (**-8.82%**) | :thumbsdown: |

---

<details>

<summary>Coverage by file</summary>

### Changed unit test files

- github.com/fgrosse/prioqueue/min_heap_test.go

</details>`
	assert.Equal(t, expected, actual)
}

// TestReport_Markdown_CoverPkg uses profiles created via "go test -coverpkg=./..."
// for a module with the packages "calc", "other" and "integration". The latter
// contains only tests which exercise "calc". The only changed file extends
// these integration tests, which increases the coverage of "calc".
func TestReport_Markdown_CoverPkg(t *testing.T) {
	oldCov, err := ParseCoverage("testdata/05-old-coverage.txt", nil)
	require.NoError(t, err)

	newCov, err := ParseCoverage("testdata/05-new-coverage.txt", nil)
	require.NoError(t, err)

	changedFiles, err := ParseChangedFiles("testdata/05-changed-files.json", "example.com/demo")
	require.NoError(t, err)

	report := NewReport(oldCov, newCov, changedFiles)
	actual := report.Markdown()

	expected := `### Merging this branch will **increase** overall coverage

| Impacted Packages | Coverage Δ | :robot: |
|-------------------|------------|---------|
| example.com/demo/calc | 83.33% (**+50.00%**) | :star2: |
| example.com/demo/integration | 0.00% (ø) |  |

---

<details>

<summary>Coverage by file</summary>

### Changed unit test files

- example.com/demo/integration/integration_test.go

</details>`
	assert.Equal(t, expected, actual)
}

func TestReport_ImpactedPackages(t *testing.T) {
	cases := map[string]struct {
		oldProfile   string
		newProfile   string
		changedFiles []string
		expected     []string
	}{
		"package of changed file without coverage delta": {
			oldProfile:   "mode: set\nexample.com/a/a.go:1.1,2.2 1 1\n",
			newProfile:   "mode: set\nexample.com/a/a.go:1.1,2.2 1 1\n",
			changedFiles: []string{"example.com/a/a.go"},
			expected:     []string{"example.com/a"},
		},
		"package with coverage delta but without changed files": {
			oldProfile:   "mode: set\nexample.com/b/b.go:1.1,2.2 1 0\n",
			newProfile:   "mode: set\nexample.com/b/b.go:1.1,2.2 1 1\n",
			changedFiles: []string{"example.com/a/a_test.go"},
			expected:     []string{"example.com/a", "example.com/b"},
		},
		"package without coverage delta and without changed files": {
			oldProfile:   "mode: set\nexample.com/c/c.go:1.1,2.2 1 1\n",
			newProfile:   "mode: set\nexample.com/c/c.go:1.1,2.2 1 1\n",
			changedFiles: []string{"example.com/a/a_test.go"},
			expected:     []string{"example.com/a"},
		},
		"package missing in new coverage": {
			oldProfile:   "mode: set\nexample.com/b/b.go:1.1,2.2 1 1\n",
			newProfile:   "mode: set\n",
			changedFiles: []string{"example.com/a/a_test.go"},
			expected:     []string{"example.com/a"},
		},
		"no baseline coverage": {
			oldProfile:   "", // github-action.sh uses an empty file if the baseline is unavailable
			newProfile:   "mode: set\nexample.com/b/b.go:1.1,2.2 1 1\n",
			changedFiles: []string{"example.com/a/a_test.go"},
			expected:     []string{"example.com/a"},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			oldCov := parseCoverageString(t, c.oldProfile)
			newCov := parseCoverageString(t, c.newProfile)

			report := NewReport(oldCov, newCov, c.changedFiles)
			actual := report.impactedPackages(oldCov.ByPackage(), newCov.ByPackage())

			assert.Equal(t, c.expected, actual)
		})
	}
}

func parseCoverageString(t *testing.T, profile string) *Coverage {
	t.Helper()

	profiles, err := ParseProfilesFromReader(strings.NewReader(profile), nil)
	require.NoError(t, err)

	return New(profiles)
}
