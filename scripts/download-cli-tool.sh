#!/usr/bin/env bash

set -e -o pipefail

type curl > /dev/null 2>&1 || { echo >&2 'ERROR: Script requires "curl"'; exit 1; }
type sha256sum > /dev/null 2>&1 || { echo >&2 'ERROR: Script requires "sha256sum"'; exit 1; }
type tar > /dev/null 2>&1 || { echo >&2 'ERROR: Script requires "tar"'; exit 1; }

USAGE="$0: Download the go-coverage-report binary from GitHub.

This script is meant to be used as part of a GitHub action and makes use of Workflow commands as
described in https://docs.github.com/en/actions/using-workflows/workflow-commands-for-github-actions

Usage:
    $0 version [sha256sum]

Example:
    $0 Linux X64

You can use the following environment variables to configure the script:
- RUNNER_OS: The operating system of the runner (default: Linux)
- RUNNER_ARCH: The architecture of the runner (default: X64)
- DRY_RUN: Do not actually move the binary to /usr/bin (default: false)
"

if [[ $# == 0 ]]; then
  echo -e "Error: script requires at least one argument\n"
  echo "$USAGE"
  exit 1
fi

VERSION=$1
RUNNER_OS=${RUNNER_OS:-Linux}
RUNNER_ARCH=${RUNNER_ARCH:-X64}

if [[ -z ${VERSION+x} ]]; then
    echo "Missing version argument"
    exit 1
fi

if [[ $# == 2 ]]; then
  SHA256SUM=$2
fi

sudo_or_dry_run="sudo"
if [[ "$DRY_RUN" == "true" ]]; then
  sudo_or_dry_run='echo [DRY-RUN]: sudo'
fi

start_group(){
    echo "::group::$*"
    { set -x; return; } 2>/dev/null
}

end_group(){
    { set +x; return; } 2>/dev/null
    echo "::endgroup::"
}

if [[ $VERSION == "local" ]]; then
  start_group "Installing go-coverage-report from local source"
  go install -v ./cmd/go-coverage-report
  end_group
  exit 0
fi

if [[ ${#VERSION} == 40 ]]; then
  start_group "Installing go-coverage-report from remote source"
  go install -v "github.com/fgrosse/go-coverage-report@$VERSION"
  end_group
  exit 0
fi

start_group "Determining runner architecture"
if [ "$RUNNER_ARCH" = "ARM64" ]; then
  ARCH="arm64"
elif [ "$RUNNER_ARCH" = "ARM" ]; then
  ARCH="arm"
elif [ "$RUNNER_ARCH" = "X86" ]; then
  ARCH="386"
elif [ "$RUNNER_ARCH" = "X64" ]; then
  ARCH="amd64"
else
  ARCH="amd64"
fi
end_group

start_group "Downloading tar archive from GitHub"
mkdir -p .github/outputs
OS=$(echo "$RUNNER_OS" | tr '[:upper:]' '[:lower:]')
FILENAME="go-coverage-report-${VERSION}-${OS}-${ARCH}.tar.gz"
URL="https://github.com/fgrosse/go-coverage-report/releases/download/${VERSION}/${FILENAME}"
curl --fail --location "$URL" --output ".github/outputs/$FILENAME"
end_group

if ! [[ "$SHA256SUM" ]] ; then
  start_group "Checking checksum using checksums.txt file from GitHub release"
  URL="https://github.com/fgrosse/go-coverage-report/releases/download/${VERSION}/checksums.txt"
  cd .github/outputs
  curl -fsSL "$URL" | sha256sum -c --ignore-missing
  cd -
  end_group
else
  start_group "Checking checksum using provided SHA256 hash"
  echo "Actual sha256:"
  sha256sum ".github/outputs/$FILENAME"
  echo "Checking checksum"
  echo "$SHA256SUM  .github/outputs/$FILENAME" | sha256sum -c
  end_group
fi

start_group "Decompressing tar archive"
tar -xzf ".github/outputs/$FILENAME" -C .github/outputs/ go-coverage-report
rm ".github/outputs/$FILENAME"
end_group

start_group "Move binary to /usr/bin"
$sudo_or_dry_run mv .github/outputs/go-coverage-report /usr/bin
end_group
