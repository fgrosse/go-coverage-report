# Go Coverage Report GitHub action

## Usage

The `go-coverage-report` tool ships with a GitHub Action that you can easily include in your own Workflows:

```yaml
name: CI

# This setup assumes that you run the unit tests with code coverage in the same
# workflow that will also print the coverage report as comment to the pull request. 
# Therefore, you need to trigger this workflow when a pull request is (re)opened or
# when new code is pushed to the branch of the pull request. In addition, you also
# need to trigger this workflow when new code is pushed to the main branch because 
# we need to upload the code coverage results as artifact for the main branch as
# well because it will be the baseline code coverage.
# 
# We do not want to trigger the workflow for pushes to *any* branch because this
# would trigger our jobs twice on pull requests (once from "push" event and once
# from "pull_request->synchronize")
on:
  pull_request:
    types: [opened, reopened, synchronize]
  push:
    branches:
      - 'main'

jobs:
  unit_tests:
    name: "Unit tests"
    runs-on: ubuntu-latest
    steps:
      - name: Checkout repository
        uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: ^1.22

      # When you execute your unit tests, make sure to use the "-coverprofile" flag to write a 
      # coverage profile to a file. You will need the name of the file (e.g. "coverage.txt")
      # in the next step as well as the next job.
      - name: Test
        run: go test -cover -coverprofile=coverage.txt ./...

      - name: Archive code coverage results
        uses: actions/upload-artifact@v4
        with:
          name: code-coverage
          path: coverage.txt # Make sure to use the same file name you chose for the "-coverprofile" in the "Test" step

  code_coverage:
    name: "Code coverage report"
    if: github.event_name == 'pull_request' # Do not run when workflow is triggered by push to main branch
    runs-on: ubuntu-latest
    needs: unit_tests # Depends on the artifact uploaded by the "unit_tests" job
    steps:
      - uses: fgrosse/go-coverage-report@v0.3.3 # Consider using a Git revision for maximum security
        with:
          coverage-artifact-name: "code-coverage"
          coverage-file-name: "coverage.txt"
```
