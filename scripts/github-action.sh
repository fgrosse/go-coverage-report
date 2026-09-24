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
- EVENT_NAME: The event that triggered the workflow (default: push)
- REQUESTED_BASELINE_RUN_ID: Use exactly this workflow run as baseline instead of searching for one (e.g. for scripting; optional)
- REQUESTED_BASELINE_SHA: Use the latest successful baseline workflow run for this full commit SHA (e.g. the base
  commit of the pull request). If there is none, or if this is empty, the latest successful run on TARGET_BRANCH
  is used instead (optional)
- COVERAGE_ARTIFACT_NAME: The name of the artifact containing the code coverage results (default: code-coverage)
- COVERAGE_FILE_NAME: The name of the file containing the code coverage results (default: coverage.txt)
- CHANGED_FILES_PATH: The path to the file containing the list of changed files (default: .github/outputs/all_modified_files.json)
- ROOT_PACKAGE: The import path of the tested repository to add as a prefix to all paths of the changed files (optional)
- TRIM_PACKAGE: Trim a prefix in the \"Impacted Packages\" column of the markdown report (optional)
- EXCLUDE: Exclude files matching the given regular expression from the report (optional)
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
EVENT_NAME=${EVENT_NAME:-push}
REQUESTED_BASELINE_RUN_ID=${REQUESTED_BASELINE_RUN_ID:-}
REQUESTED_BASELINE_SHA=${REQUESTED_BASELINE_SHA:-}
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
    { set +x; } 2>/dev/null
    echo "::endgroup::"
}

start_group "Download code coverage results from current run"
gh run download "$GITHUB_RUN_ID" --name="$COVERAGE_ARTIFACT_NAME" --dir="/tmp/gh-run-download-$GITHUB_RUN_ID"
mv "/tmp/gh-run-download-$GITHUB_RUN_ID/$COVERAGE_FILE_NAME" $NEW_COVERAGE_PATH
rm -r "/tmp/gh-run-download-$GITHUB_RUN_ID"
end_group

# jq program that turns a single workflow run into one tab separated line:
# <run id> <head sha> <head branch> <created at> <url> <age>
# The age is computed in jq (and not with GNU date) so this works on all runners.
# shellcheck disable=SC2016 # $s is a jq variable, not a shell variable
RUN_DETAILS_JQ='[.databaseId, .headSha, .headBranch, .createdAt, .url,
  ([now - (.createdAt | sub("\\.[0-9]+Z$"; "Z") | fromdate), 0] | max | floor) as $s
  | if $s >= 86400 then "\($s / 86400 | floor)d \($s % 86400 / 3600 | floor)h"
    elif $s >= 3600 then "\($s / 3600 | floor)h \($s % 3600 / 60 | floor)m"
    else "\($s / 60 | floor)m" end
  ] | @tsv'

# find_baseline_run prints the details of the latest successful run of the baseline workflow
# that matches the given additional "gh run list" filters, or nothing if there is none.
find_baseline_run(){
  gh run list --status=success --workflow="$GITHUB_BASELINE_WORKFLOW" --event="$EVENT_NAME" \
    --json=databaseId,headSha,headBranch,createdAt,url --limit=1 -q ".[] | $RUN_DETAILS_JQ" "$@"
}

start_group "Download code coverage results from target branch"
# The selected baseline run is described by the following variables:
# BASELINE_RUN_ID, BASELINE_SHA, BASELINE_BRANCH, BASELINE_CREATED_AT, BASELINE_RUN_URL, BASELINE_AGE
# and BASELINE_SOURCE ("explicit run id", "base sha" or "latest run fallback").
RUN_DETAILS=""
BASELINE_SOURCE=""
if [ -n "$REQUESTED_BASELINE_RUN_ID" ]; then
  if ! RUN_DETAILS=$(gh run view "$REQUESTED_BASELINE_RUN_ID" --json=databaseId,headSha,headBranch,createdAt,url -q "$RUN_DETAILS_JQ"); then
    echo "::error::Could not find the requested baseline run $REQUESTED_BASELINE_RUN_ID"
    exit 1
  fi
  BASELINE_SOURCE="explicit run id"
else
  if [ -n "$REQUESTED_BASELINE_SHA" ]; then
    RUN_DETAILS=$(find_baseline_run --branch="$TARGET_BRANCH" --commit="$REQUESTED_BASELINE_SHA")
    if [ -n "$RUN_DETAILS" ]; then BASELINE_SOURCE="base sha"; fi
  fi
  if [ -z "$RUN_DETAILS" ]; then
    RUN_DETAILS=$(find_baseline_run --branch="$TARGET_BRANCH")
    if [ -n "$RUN_DETAILS" ]; then BASELINE_SOURCE="latest run fallback"; fi
  fi
fi

BASELINE_RUN_ID="" BASELINE_SHA="" BASELINE_BRANCH="" BASELINE_CREATED_AT="" BASELINE_RUN_URL="" BASELINE_AGE=""
if [ -n "$RUN_DETAILS" ]; then
  IFS=$'\t' read -r BASELINE_RUN_ID BASELINE_SHA BASELINE_BRANCH BASELINE_CREATED_AT BASELINE_RUN_URL BASELINE_AGE <<< "$RUN_DETAILS" || true
fi

BASELINE_AVAILABLE=true
BASELINE_UNAVAILABLE_REASON=""
if [ -z "$BASELINE_RUN_ID" ]; then
  if [ -n "$REQUESTED_BASELINE_SHA" ]; then
    echo "::warning::No successful run of workflow \"$GITHUB_BASELINE_WORKFLOW\" (event \"$EVENT_NAME\") found on branch \"$TARGET_BRANCH\" (searched for base commit ${REQUESTED_BASELINE_SHA:0:7} first, then for the latest run). Coverage cannot be compared against a baseline."
  else
    echo "::warning::No successful run of workflow \"$GITHUB_BASELINE_WORKFLOW\" (event \"$EVENT_NAME\") found on branch \"$TARGET_BRANCH\". Coverage cannot be compared against a baseline."
  fi
  BASELINE_AVAILABLE=false
  BASELINE_UNAVAILABLE_REASON=no-run
else
  if [ -n "$REQUESTED_BASELINE_SHA" ] && [ "$BASELINE_SOURCE" = "latest run fallback" ]; then
    echo "::warning::No successful run of workflow \"$GITHUB_BASELINE_WORKFLOW\" (event \"$EVENT_NAME\") found for base commit ${REQUESTED_BASELINE_SHA:0:7}, so the latest run on \"$TARGET_BRANCH\" is used instead (${BASELINE_SHA:0:7}). Coverage changes may include commits that are not part of this pull request."
  fi
  echo "::notice::Using baseline run $BASELINE_RUN_ID (source: $BASELINE_SOURCE) for commit ${BASELINE_SHA:0:7} on \"$BASELINE_BRANCH\", created at $BASELINE_CREATED_AT ($BASELINE_AGE ago): $BASELINE_RUN_URL"

  # Try to download the baseline artifact, but don't fail if it's unavailable
  if gh run download "$BASELINE_RUN_ID" --name="$COVERAGE_ARTIFACT_NAME" --dir="/tmp/gh-run-download-$BASELINE_RUN_ID"; then
    mv "/tmp/gh-run-download-$BASELINE_RUN_ID/$COVERAGE_FILE_NAME" $OLD_COVERAGE_PATH
    rm -r "/tmp/gh-run-download-$BASELINE_RUN_ID"
  else
    echo "::warning::Could not download artifact \"$COVERAGE_ARTIFACT_NAME\" from baseline run $BASELINE_RUN_ID ($BASELINE_RUN_URL), which was created $BASELINE_AGE ago. The artifact may have expired or the artifact name may be wrong."
    BASELINE_AVAILABLE=false
    BASELINE_UNAVAILABLE_REASON=no-artifact
  fi
fi
end_group

start_group "Compare code coverage results"
if [ "$BASELINE_AVAILABLE" = "false" ]; then
  # No baseline available - create an empty one for comparison
  echo "::notice::Generating coverage report without baseline comparison"
  touch "$OLD_COVERAGE_PATH"
fi

# The baseline commit and run are shown in the details section of the report.
BASELINE_FLAGS=()
if [ "$BASELINE_AVAILABLE" = "true" ]; then
  BASELINE_FLAGS=(-baseline-commit="$BASELINE_SHA" -baseline-run-id="$BASELINE_RUN_ID" -baseline-run-url="$BASELINE_RUN_URL")
fi

METRICS_OUTPUT=$(mktemp)

go-coverage-report \
    -root="$ROOT_PACKAGE" \
    -trim="$TRIM_PACKAGE" \
    ${EXCLUDE:+-exclude="$EXCLUDE"} \
    -metrics-file="$METRICS_OUTPUT" \
    "${BASELINE_FLAGS[@]}" \
    "$OLD_COVERAGE_PATH" \
    "$NEW_COVERAGE_PATH" \
    "$CHANGED_FILES_PATH" \
  > $COVERAGE_COMMENT_PATH

cat "$METRICS_OUTPUT" >> "$GITHUB_OUTPUT"
rm -f "$METRICS_OUTPUT"

# If the report does not compare against the base commit of the pull request,
# explain why in a single line callout at the very top of the report.
CAUTION=""
if [ "$BASELINE_UNAVAILABLE_REASON" = "no-run" ]; then
  CAUTION="No coverage from the \`$GITHUB_BASELINE_WORKFLOW\` workflow was found on \`$TARGET_BRANCH\`, so this report only shows the current coverage of the changed files"
elif [ "$BASELINE_UNAVAILABLE_REASON" = "no-artifact" ]; then
  CAUTION="The \`$COVERAGE_ARTIFACT_NAME\` artifact of run [#$BASELINE_RUN_ID]($BASELINE_RUN_URL) for commit ${BASELINE_SHA:0:7} could not be downloaded (it may have expired), so this report only shows the current coverage of the changed files"
elif [ -n "$REQUESTED_BASELINE_SHA" ] && [ "$BASELINE_SOURCE" = "latest run fallback" ]; then
  CAUTION="No coverage for base commit ${REQUESTED_BASELINE_SHA:0:7} was found, so this report compares against ${BASELINE_SHA:0:7} (latest run on \`$TARGET_BRANCH\`)"
fi

if [ ! -s $COVERAGE_COMMENT_PATH ]; then
  if [ "$BASELINE_AVAILABLE" = "false" ]; then
    # No changed Go files - skip posting a comment since there's nothing to report
    echo "::notice::No changed Go files detected and no baseline available - skipping coverage comment"
  fi
elif [ -n "$CAUTION" ]; then
  mv $COVERAGE_COMMENT_PATH $COVERAGE_COMMENT_PATH.tmp
  {
    echo "> [!CAUTION]"
    echo "> $CAUTION"
    echo ""
    cat $COVERAGE_COMMENT_PATH.tmp
  } > $COVERAGE_COMMENT_PATH
  rm $COVERAGE_COMMENT_PATH.tmp
fi
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
