#!/usr/bin/env bash

set -e -o pipefail

type gh > /dev/null 2>&1 || { echo >&2 'ERROR: Script requires "gh" (see https://cli.github.com)'; exit 1; }
type go-coverage-report > /dev/null 2>&1 || { echo >&2 'ERROR: Script requires "go-coverage-report" binary in PATH'; exit 1; }

USAGE="$0: Execute go-coverage-report as GitHub action.

This script is meant to be used as a GitHub action and makes use of Workflow commands as
described in https://docs.github.com/en/actions/using-workflows/workflow-commands-for-github-actions

Usage:
    $0 github_repository github_pull_request_number github_run_id

Example:
    $0 fgrosse/prioqueue 12 8221109494

You can largely rely on the default environment variables set by GitHub Actions. The script should be invoked like
this in the workflow file:

    -name: Code coverage report
     run: github-action.sh \${{ github.repository }} \${{ github.event.pull_request.number }} \${{ github.run_id }}
     env: …

You can use the following environment variables to configure the script:
- GITHUB_BASELINE_WORKFLOW: The name of the GitHub actions Workflow that produces the baseline coverage (default: CI)
- GITHUB_BASELINE_WORKFLOW_REF: The ref path to the workflow to use instead of GITHUB_BASELINE_WORKFLOW (optional)
- TARGET_BRANCH: The base branch to compare the coverage results against (default: main)
- COVERAGE_ARTIFACT_NAME: The name of the artifact containing the code coverage results (default: code-coverage)
- COVERAGE_FILE_NAME: The name of the file containing the code coverage results (default: coverage.txt)
- CHANGED_FILES_PATH: The path to the file containing the list of changed files (default: .github/outputs/all_modified_files.json)
- ROOT_PACKAGE: The import path of the tested repository to add as a prefix to all paths of the changed files (optional)
- TRIM_PACKAGE: Trim a prefix in the \"Impacted Packages\" column of the markdown report (optional)
- SKIP_COMMENT: Skip creating or updating the pull request comment (default: false)
"

if [[ $# != 3 ]]; then
  echo -e "Error: script requires exactly three arguments\n"
  echo "$USAGE"
  exit 1
fi

GITHUB_REPOSITORY=$1
GITHUB_PULL_REQUEST_NUMBER=$2
GITHUB_RUN_ID=$3
GITHUB_BASELINE_WORKFLOW=${GITHUB_BASELINE_WORKFLOW:-CI}
TARGET_BRANCH=${TARGET_BRANCH:-main}
COVERAGE_ARTIFACT_NAME=${COVERAGE_ARTIFACT_NAME:-code-coverage}
COVERAGE_FILE_NAME=${COVERAGE_FILE_NAME:-coverage.txt}

OLD_COVERAGE_PATH=.github/outputs/old-coverage.txt
NEW_COVERAGE_PATH=.github/outputs/new-coverage.txt
COVERAGE_COMMENT_PATH=.github/outputs/coverage-comment.md
CHANGED_FILES_PATH=${CHANGED_FILES_PATH:-.github/outputs/all_modified_files.json}
SKIP_COMMENT=${SKIP_COMMENT:-false}

if [[ -z ${GITHUB_REPOSITORY+x} ]]; then
    echo "Missing github_repository argument"
    exit 1
fi

if [[ -z ${GITHUB_PULL_REQUEST_NUMBER+x} ]]; then
    echo "Missing github_pull_request_number argument"
    exit 1
fi

if [[ -z ${GITHUB_RUN_ID+x} ]]; then
    echo "Missing github_run_id argument"
    exit 1
fi

if [[ -z ${GITHUB_OUTPUT+x} ]]; then
    echo "Missing GITHUB_OUTPUT environment variable"
    exit 1
fi

# If GITHUB_BASELINE_WORKFLOW_REF is defined, extract the workflow file path from it and use it instead of GITHUB_BASELINE_WORKFLOW
if [[ -n ${GITHUB_BASELINE_WORKFLOW_REF+x} ]]; then
    GITHUB_BASELINE_WORKFLOW=$(basename "${GITHUB_BASELINE_WORKFLOW_REF%%@*}")
fi

export GH_REPO="$GITHUB_REPOSITORY"

start_group(){
    echo "::group::$*"
    { set -x; return; } 2>/dev/null
}

end_group(){
    { set +x; return; } 2>/dev/null
    echo "::endgroup::"
}

start_group "Download code coverage results from current run"
gh run download "$GITHUB_RUN_ID" --name="$COVERAGE_ARTIFACT_NAME" --dir="/tmp/gh-run-download-$GITHUB_RUN_ID"
mv "/tmp/gh-run-download-$GITHUB_RUN_ID/$COVERAGE_FILE_NAME" $NEW_COVERAGE_PATH
rm -r "/tmp/gh-run-download-$GITHUB_RUN_ID"
end_group

start_group "Download code coverage results from target branch"
LAST_SUCCESSFUL_RUN_ID=$(gh run list --status=success --branch="$TARGET_BRANCH" --workflow="$GITHUB_BASELINE_WORKFLOW" --event=push --json=databaseId --limit=1 -q '.[] | .databaseId')
if [ -z "$LAST_SUCCESSFUL_RUN_ID" ]; then
  echo "::error::No successful run found on the target branch"
  exit 1
fi

gh run download "$LAST_SUCCESSFUL_RUN_ID" --name="$COVERAGE_ARTIFACT_NAME" --dir="/tmp/gh-run-download-$LAST_SUCCESSFUL_RUN_ID"
mv "/tmp/gh-run-download-$LAST_SUCCESSFUL_RUN_ID/$COVERAGE_FILE_NAME" $OLD_COVERAGE_PATH
rm -r "/tmp/gh-run-download-$LAST_SUCCESSFUL_RUN_ID"
end_group

start_group "Compare code coverage results"
go-coverage-report \
    -root="$ROOT_PACKAGE" \
    -trim="$TRIM_PACKAGE" \
    "$OLD_COVERAGE_PATH" \
    "$NEW_COVERAGE_PATH" \
    "$CHANGED_FILES_PATH" \
  > $COVERAGE_COMMENT_PATH
end_group

if [ ! -s $COVERAGE_COMMENT_PATH ]; then
  echo "::notice::No coverage report to output"
  exit 0
fi

# Output the coverage report as a multiline GitHub output parameter
echo "Writing GitHub output parameter to \"$GITHUB_OUTPUT\""
{
  echo "coverage_report<<END_OF_COVERAGE_REPORT"
  cat "$COVERAGE_COMMENT_PATH"
  echo "END_OF_COVERAGE_REPORT"
} >> "$GITHUB_OUTPUT"

if [ "$SKIP_COMMENT" = "true" ]; then
  echo "Skipping pull request comment (\$SKIP_COMMENT=true))"
  exit 0
fi

start_group "Comment on pull request"
COMMENT_ID=$(gh api "repos/${GITHUB_REPOSITORY}/issues/${GITHUB_PULL_REQUEST_NUMBER}/comments" -q '.[] | select(.user.login=="github-actions[bot]" and (.body | test("Coverage Δ")) ) | .id' | head -n 1)
if [ -z "$COMMENT_ID" ]; then
  echo "Creating new coverage report comment"
else
  echo "Replacing old coverage report comment"
  gh api -X DELETE "repos/${GITHUB_REPOSITORY}/issues/comments/${COMMENT_ID}"
fi

gh pr comment "$GITHUB_PULL_REQUEST_NUMBER" --body-file=$COVERAGE_COMMENT_PATH
end_group
