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
- MIN_COVERAGE_NEW_CODE: Minimum coverage threshold for new code in percentage (default: 0, disabled)
- USE_GIT_DIFF: Use git diff for line-level coverage calculation (default: true)
- BASELINE_LOOKBACK_DAYS: Only consider target-branch runs created in this many days (default: 14)
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
MIN_COVERAGE_NEW_CODE=${MIN_COVERAGE_NEW_CODE:-0}
USE_GIT_DIFF=${USE_GIT_DIFF:-true}
BASELINE_LOOKBACK_DAYS=${BASELINE_LOOKBACK_DAYS:-14}

OLD_COVERAGE_PATH=.github/outputs/old-coverage.txt
NEW_COVERAGE_PATH=.github/outputs/new-coverage.txt
COVERAGE_COMMENT_PATH=.github/outputs/coverage-comment.md
DIFF_FILE_PATH=.github/outputs/pr-diff.patch
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
# GitHub caps filtered workflow-run searches at 1000 results. An unbounded
# list can return a stale run whose coverage artifact has already expired.
if date -u -d "${BASELINE_LOOKBACK_DAYS} days ago" +%Y-%m-%d >/dev/null 2>&1; then
  CREATED_SINCE=$(date -u -d "${BASELINE_LOOKBACK_DAYS} days ago" +%Y-%m-%d)
else
  CREATED_SINCE=$(date -u -v-"${BASELINE_LOOKBACK_DAYS}"d +%Y-%m-%d)
fi

echo "Looking for successful ${TARGET_BRANCH} runs of ${GITHUB_BASELINE_WORKFLOW} created on or after ${CREATED_SINCE}"
RUN_IDS=$(gh run list --status=success --branch="$TARGET_BRANCH" --workflow="$GITHUB_BASELINE_WORKFLOW" --event=push --created=">=${CREATED_SINCE}" --json=databaseId --limit=20 -q '.[].databaseId')
if [ -z "$RUN_IDS" ]; then
  echo "::error::No successful run found on the target branch in the last ${BASELINE_LOOKBACK_DAYS} days"
  exit 1
fi

LAST_SUCCESSFUL_RUN_ID=""
for RUN_ID in $RUN_IDS; do
  if gh run download "$RUN_ID" --name="$COVERAGE_ARTIFACT_NAME" --dir="/tmp/gh-run-download-$RUN_ID"; then
    LAST_SUCCESSFUL_RUN_ID=$RUN_ID
    break
  fi
  echo "Skipping run ${RUN_ID} (no ${COVERAGE_ARTIFACT_NAME} artifact)"
  rm -rf "/tmp/gh-run-download-$RUN_ID"
done

if [ -z "$LAST_SUCCESSFUL_RUN_ID" ]; then
  echo "::error::No successful run on the target branch in the last ${BASELINE_LOOKBACK_DAYS} days had a ${COVERAGE_ARTIFACT_NAME} artifact"
  exit 1
fi

mv "/tmp/gh-run-download-$LAST_SUCCESSFUL_RUN_ID/$COVERAGE_FILE_NAME" $OLD_COVERAGE_PATH
rm -r "/tmp/gh-run-download-$LAST_SUCCESSFUL_RUN_ID"
end_group

start_group "Generate git diff for line-level coverage"
if [ "$USE_GIT_DIFF" = "true" ]; then
  echo "Generating git diff between $TARGET_BRANCH and HEAD..."
  # Fetch the target branch to ensure we have it
  git fetch origin "$TARGET_BRANCH:refs/remotes/origin/$TARGET_BRANCH" || true
  # Generate unified diff for Go files only
  git diff "origin/$TARGET_BRANCH...HEAD" -- '*.go' > "$DIFF_FILE_PATH" || true
  
  if [ -s "$DIFF_FILE_PATH" ]; then
    echo "Git diff generated successfully ($(wc -l < "$DIFF_FILE_PATH") lines)"
  else
    echo "No diff generated or diff is empty, falling back to block-based comparison"
    rm -f "$DIFF_FILE_PATH"
  fi
else
  echo "Git diff disabled, using block-based comparison"
fi
end_group

start_group "Compare code coverage results"
# Capture the exit code but don't fail yet - we want to post the comment first
set +e

# Build the command arguments
COVERAGE_ARGS=(-root="$ROOT_PACKAGE" -trim="$TRIM_PACKAGE" -min-coverage="$MIN_COVERAGE_NEW_CODE")
if [ -f "$DIFF_FILE_PATH" ]; then
  COVERAGE_ARGS+=(-diff="$DIFF_FILE_PATH")
fi
COVERAGE_ARGS+=("$OLD_COVERAGE_PATH" "$NEW_COVERAGE_PATH" "$CHANGED_FILES_PATH")

go-coverage-report "${COVERAGE_ARGS[@]}" > "$COVERAGE_COMMENT_PATH" 2>"$COVERAGE_COMMENT_PATH.err"
COVERAGE_EXIT_CODE=$?
set -e
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

# Now check if the coverage report failed the threshold check
if [ $COVERAGE_EXIT_CODE -ne 0 ]; then
  echo "::error::Coverage check failed"
  if [ -s $COVERAGE_COMMENT_PATH.err ]; then
    cat $COVERAGE_COMMENT_PATH.err >&2
  fi
  exit $COVERAGE_EXIT_CODE
fi
