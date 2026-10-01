package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The tests in this file execute scripts/github-action.sh with fake "gh" and "go-coverage-report"
// binaries from testdata/bin, which answer from fixture files instead of the GitHub API.
// They must not run in parallel, since the script downloads artifacts into fixed
// directories below /tmp that are named after the run ID.

// currentRunID is the ID of the workflow run that executes the action in all tests.
const currentRunID = 999

// workflowRun is a workflow run as returned by "gh run list --json".
type workflowRun struct {
	DatabaseID int    `json:"databaseId"`
	HeadSha    string `json:"headSha"`
	HeadBranch string `json:"headBranch"`
	Event      string `json:"event"`
	Conclusion string `json:"conclusion"`
	CreatedAt  string `json:"createdAt"`
	URL        string `json:"url"`
}

// actionTest describes the GitHub state that the fake gh binary exposes to the script.
type actionTest struct {
	Runs            []workflowRun
	NoArtifact      []int             // IDs of runs whose coverage artifact cannot be downloaded
	CommitsAPIFails bool              // if set, listing the ancestors of a commit fails
	Env             map[string]string // overrides the default environment of the script
}

type actionResult struct {
	ExitCode int
	Log      string   // stdout and stderr of the script
	Comment  string   // the generated pull request comment
	Calls    []string // all calls of the fake binaries
}

func TestBaselineSelection(t *testing.T) {
	tests := map[string]struct {
		test actionTest

		wantBaselineRun int    // the run whose coverage is used as baseline, or 0 if there is none
		wantSource      string // the BASELINE_SOURCE of the selected run
		wantCaution     string // part of the caution callout, or empty if there should be none
		wantDownloads   int    // number of attempted baseline downloads, or 0 to skip this check
	}{
		"base commit has a run": {
			test: actionTest{Runs: []workflowRun{
				newRun(108, 8, "success"),
				newRun(109, 9, "success"),
			}},
			wantBaselineRun: 109,
			wantSource:      "base sha",
			wantDownloads:   1,
		},
		"newest successful run of the base commit": {
			test: actionTest{Runs: []workflowRun{
				newRun(109, 9, "success"),
				newRun(119, 9, "success").createdAt("2026-09-30T11:00:00Z"),
				newRun(129, 9, "failure").createdAt("2026-09-30T12:00:00Z"),
			}},
			wantBaselineRun: 119,
			wantSource:      "base sha",
		},
		"base run failed and runs of other branches or events are ignored": {
			test: actionTest{Runs: []workflowRun{
				newRun(108, 8, "success"),
				newRun(109, 9, "failure"),
				newRun(119, 9, "success").on("main", "pull_request"),
				newRun(129, 9, "success").on("feature", "push"),
			}},
			wantBaselineRun: 108,
			wantSource:      "ancestor",
			wantCaution:     "compares against its ancestor c800000 (1 commit earlier)",
		},
		"artifact of base run expired": {
			test: actionTest{
				Runs: []workflowRun{
					newRun(107, 7, "success"),
					newRun(109, 9, "success"),
				},
				NoArtifact: []int{109},
			},
			wantBaselineRun: 107,
			wantSource:      "ancestor",
			wantCaution:     "compares against its ancestor c700000 (2 commits earlier)",
			wantDownloads:   2,
		},
		"no ancestor has a run": {
			test: actionTest{
				Runs: []workflowRun{
					newRun(109, 9, "success"),
					newRun(209, 9, "success").on("main", "pull_request").createdAt("2026-09-30T11:00:00Z"),
				},
				Env: map[string]string{"REQUESTED_BASELINE_SHA": commitSHA(2)},
			},
			wantBaselineRun: 109,
			wantSource:      "latest run fallback",
			wantCaution:     "No coverage for base commit c200000 was found, so this report compares against c900000 (latest run on `main`)",
		},
		"search depth limits the ancestors": {
			test: actionTest{
				Runs: []workflowRun{newRun(107, 7, "success")},
				Env:  map[string]string{"BASELINE_SEARCH_DEPTH": "2"},
			},
			wantBaselineRun: 107,
			wantSource:      "latest run fallback",
			wantCaution:     "(latest run on `main`)",
		},
		"listing the ancestors fails": {
			test: actionTest{
				Runs:            []workflowRun{newRun(108, 8, "success")},
				CommitsAPIFails: true,
			},
			wantBaselineRun: 108,
			wantSource:      "latest run fallback",
			wantCaution:     "(latest run on `main`)",
		},
		"empty baseline sha uses the latest run": {
			test: actionTest{
				Runs: []workflowRun{
					newRun(109, 9, "success"),
					newRun(103, 3, "success").createdAt("2026-09-30T12:00:00Z"),
				},
				Env: map[string]string{"REQUESTED_BASELINE_SHA": ""},
			},
			wantBaselineRun: 103,
			wantSource:      "latest run fallback",
		},
		"no matching run": {
			test: actionTest{Runs: []workflowRun{
				newRun(309, 9, "success").on("feature", "push"),
			}},
			wantCaution:   "No coverage from the `ci.yml` workflow was found on `main`",
			wantDownloads: -1,
		},
		"no artifact can be downloaded": {
			test: actionTest{
				Runs: []workflowRun{
					newRun(109, 9, "success"),
					newRun(108, 8, "success"),
					newRun(107, 7, "success"),
					newRun(106, 6, "success"),
				},
				NoArtifact: []int{109, 108, 107, 106},
				Env:        map[string]string{"BASELINE_MAX_DOWNLOADS": "3"},
			},
			wantCaution:   "The `code-coverage` artifact of run [#109](https://github.com/owner/repo/actions/runs/109) for commit c900000 could not be downloaded",
			wantDownloads: 3,
		},
		"explicit run id": {
			test: actionTest{
				Runs: []workflowRun{
					newRun(109, 9, "success"),
					newRun(105, 5, "success"),
				},
				Env: map[string]string{"REQUESTED_BASELINE_RUN_ID": "105"},
			},
			wantBaselineRun: 105,
			wantSource:      "explicit run id",
			wantDownloads:   1,
		},
		"explicit run id without artifact": {
			test: actionTest{
				Runs:       []workflowRun{newRun(105, 5, "success")},
				NoArtifact: []int{105},
				Env:        map[string]string{"REQUESTED_BASELINE_RUN_ID": "105"},
			},
			wantCaution:   "The `code-coverage` artifact of run [#105]",
			wantDownloads: 1,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			res := runAction(t, tc.test)
			require.Equal(t, 0, res.ExitCode, res.Log)

			reportCalls := res.callsWithPrefix("go-coverage-report ")
			require.Len(t, reportCalls, 1, res.Log)

			if tc.wantBaselineRun == 0 {
				assert.Contains(t, res.Comment, "Baseline: []", res.Log)
				assert.NotContains(t, reportCalls[0], "-baseline-", res.Log)
			} else {
				assert.Contains(t, res.Comment, fmt.Sprintf("Baseline: [coverage of run %d]", tc.wantBaselineRun), res.Log)
				assert.Contains(t, reportCalls[0], fmt.Sprintf("-baseline-run-id=%d ", tc.wantBaselineRun), res.Log)
				assert.Contains(t, res.Log, fmt.Sprintf("::notice::Using baseline run %d (source: %s)", tc.wantBaselineRun, tc.wantSource))
			}

			if tc.wantCaution == "" {
				assert.NotContains(t, res.Comment, "[!CAUTION]", res.Log)
			} else {
				assert.True(t, strings.HasPrefix(res.Comment, "> [!CAUTION]\n"), res.Comment)
				assert.Contains(t, res.Comment, tc.wantCaution, res.Log)
			}

			// Every test also downloads the coverage of the current run.
			downloads := len(res.callsWithPrefix("gh run download ")) - 1
			switch {
			case tc.wantDownloads > 0:
				assert.Equal(t, tc.wantDownloads, downloads, res.Log)
			case tc.wantDownloads < 0:
				assert.Zero(t, downloads, res.Log)
			}
		})
	}
}

func TestBaselineSelection_UnknownRunID(t *testing.T) {
	res := runAction(t, actionTest{
		Runs: []workflowRun{newRun(109, 9, "success")},
		Env:  map[string]string{"REQUESTED_BASELINE_RUN_ID": "42"},
	})

	assert.Equal(t, 1, res.ExitCode, res.Log)
	assert.Contains(t, res.Log, "::error::Could not find the requested baseline run 42")
}

func TestBaselineSelection_InvalidInputs(t *testing.T) {
	tests := map[string]struct {
		env       map[string]string
		wantError string
	}{
		"search depth is not a number": {
			env:       map[string]string{"BASELINE_SEARCH_DEPTH": "abc"},
			wantError: `::error::BASELINE_SEARCH_DEPTH must be a number between 1 and 100, got "abc"`,
		},
		"search depth is zero": {
			env:       map[string]string{"BASELINE_SEARCH_DEPTH": "0"},
			wantError: `::error::BASELINE_SEARCH_DEPTH must be a number between 1 and 100, got "0"`,
		},
		"search depth is too large": {
			env:       map[string]string{"BASELINE_SEARCH_DEPTH": "101"},
			wantError: `::error::BASELINE_SEARCH_DEPTH must be a number between 1 and 100, got "101"`,
		},
		"max downloads is zero": {
			env:       map[string]string{"BASELINE_MAX_DOWNLOADS": "0"},
			wantError: `::error::BASELINE_MAX_DOWNLOADS must be a positive number, got "0"`,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			res := runAction(t, actionTest{Env: tc.env})
			assert.Equal(t, 1, res.ExitCode, res.Log)
			assert.Contains(t, res.Log, tc.wantError)
			assert.Empty(t, res.Calls, "the script should fail before calling gh")
		})
	}
}

// runAction executes github-action.sh against the given fake GitHub state.
func runAction(t *testing.T, at actionTest) actionResult {
	t.Helper()

	for _, tool := range []string{"bash", "jq"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is required to test github-action.sh", tool)
		}
	}

	script, err := filepath.Abs("../../scripts/github-action.sh")
	require.NoError(t, err)
	binDir, err := filepath.Abs("testdata/bin")
	require.NoError(t, err)

	workDir := t.TempDir()
	t.Log("workDir:", workDir)

	require.NoError(t, os.MkdirAll(filepath.Join(workDir, ".github", "outputs"), 0o750))
	writeFile(t, filepath.Join(workDir, ".github", "outputs", "all_modified_files.json"), "[]")
	fakeDir := writeFixtures(t, at)

	env := map[string]string{
		"PATH":                     binDir + string(os.PathListSeparator) + os.Getenv("PATH"),
		"HOME":                     os.Getenv("HOME"),
		"GH_FAKE_DIR":              fakeDir,
		"GH_FAKE_CURRENT_RUN":      fmt.Sprint(currentRunID),
		"GITHUB_OUTPUT":            filepath.Join(workDir, "github-output"),
		"GITHUB_BASELINE_WORKFLOW": "ci.yml",
		"TARGET_BRANCH":            "main",
		"EVENT_NAME":               "push",
		"SKIP_COMMENT":             "true",
		"REQUESTED_BASELINE_SHA":   commitSHA(9),
	}
	for k, v := range at.Env {
		env[k] = v
	}

	// The only variable is the absolute path of github-action.sh.
	cmd := exec.Command("bash", script, "owner/repo", "1", fmt.Sprint(currentRunID)) //nolint:gosec
	cmd.Dir = workDir
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	out, err := cmd.CombinedOutput()
	res := actionResult{Log: string(out)}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		res.ExitCode = exitErr.ExitCode()
	} else {
		require.NoError(t, err)
	}

	if comment, err := os.ReadFile(filepath.Join(workDir, ".github", "outputs", "coverage-comment.md")); err == nil {
		res.Comment = string(comment)
	}
	if calls, err := os.ReadFile(filepath.Join(fakeDir, "calls")); err == nil {
		res.Calls = strings.Split(strings.TrimSpace(string(calls)), "\n")
	}

	return res
}

// writeFixtures writes the fixture files of the fake gh binary into a new
// temporary directory and returns its path.
func writeFixtures(t *testing.T, at actionTest) string {
	t.Helper()
	dir := t.TempDir()

	var commits strings.Builder
	for n := 9; n >= 0; n-- {
		commits.WriteString(commitSHA(n) + "\n")
	}
	writeFile(t, filepath.Join(dir, "commits"), commits.String())

	runs, err := json.Marshal(append([]workflowRun{}, at.Runs...))
	require.NoError(t, err)
	writeFile(t, filepath.Join(dir, "runs.json"), string(runs))

	var noArtifact strings.Builder
	for _, id := range at.NoArtifact {
		fmt.Fprintln(&noArtifact, id)
	}
	writeFile(t, filepath.Join(dir, "no-artifact"), noArtifact.String())

	if at.CommitsAPIFails {
		writeFile(t, filepath.Join(dir, "commits-api-fails"), "")
	}

	return dir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

// callsWithPrefix returns all calls of the fake binaries that start with the given prefix.
func (r actionResult) callsWithPrefix(prefix string) []string {
	var calls []string
	for _, c := range r.Calls {
		if strings.HasPrefix(c, prefix) {
			calls = append(calls, c)
		}
	}
	return calls
}

// newRun returns a run of a push to main for the given commit, created
// at a time that is later for newer commits.
func newRun(id, commit int, conclusion string) workflowRun {
	return workflowRun{
		DatabaseID: id,
		HeadSha:    commitSHA(commit),
		HeadBranch: "main",
		Event:      "push",
		Conclusion: conclusion,
		CreatedAt:  fmt.Sprintf("2026-09-30T10:%02d:00Z", commit),
		URL:        fmt.Sprintf("https://github.com/owner/repo/actions/runs/%d", id),
	}
}

func (r workflowRun) on(branch, event string) workflowRun {
	r.HeadBranch, r.Event = branch, event
	return r
}

func (r workflowRun) createdAt(t string) workflowRun {
	r.CreatedAt = t
	return r
}

// commitSHA returns the full SHA of the fake commit with the given number.
// Higher numbers are newer commits, and commit 9 is the base commit by default.
func commitSHA(n int) string {
	return fmt.Sprintf("c%d%039d", n, 0)[:40]
}
