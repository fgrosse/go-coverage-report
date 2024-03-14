## Using as GitHub Action

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
      - uses: fgrosse/go-coverage-report@v0.2.0 # Consider using a Git revision for maximum security
        with:
          coverage-artifact-name: "code-coverage"
          coverage-file-name: "coverage.txt"
```

## Creating your own GitHub Actions Job

For maximum control and security, you may choose to create your own GitHub Actions job to create a code coverage report:

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

      - name: Test
        run: go test -cover -coverprofile=coverage.txt ./...

      - name: Archive code coverage results
        uses: actions/upload-artifact@v4
        with:
          name: code-coverage
          path: coverage.txt  # Make sure to use the same file name you chose for the "-coverprofile" in the "Test" step

  code_coverage:
    name: "Code coverage report"
    if: github.event_name == 'pull_request' # Do not run when workflow is triggered by push to main branch
    runs-on: ubuntu-latest
    needs: unit_tests # Depends on the artifact uploaded by the "unit_tests" job
    env:
      # Export variables used by the GitHub CLI application ("gh")
      GH_REPO: ${{ github.repository }}
      GH_TOKEN: ${{ github.token }}
    steps:
      
      # Setup Go so we can install the "go-coverage-report" tool in the next step
      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: ^1.22

      # Install the go-coverage-report binary. It is recommended to pin the used version here.
      - name: Install go-coverage-report
        run: go install github.com/fgrosse/go-coverage-report@v0.1.0

      # We need to create a list of changed files. The "tj-actions/changed-files" action is
      # a good choice for this. We pin it to a specific Git commit for security reasons.
      # Note that we ignore test files and the "vendor" directory because they are not relevant
      # in the context of creating a code coverage for our own code.
      - name: Determine changed files
        id: changed-files
        uses: tj-actions/changed-files@aa08304bd477b800d468db44fe10f6c61f7f7b11 # v42.1.0
        with:
          write_output_files: true
          json: true
          files: |
            **.go
          files_ignore: |
            **_test.go
            vendor/**

      # Download code coverage results from the "unit_tests" job. We chose to download the
      # file into ".github/outputs" just to follow best practices and not override any file
      # in the repository.
      - name: Download code coverage results from current run
        uses: actions/download-artifact@v4
        with:
          name: code-coverage
          path: .github/outputs

      # Rename the code coverage results file from the current run to "new-coverage.txt"
      # Make sure to use the same name you chose for the "-coverprofile" in the "Test" step.
      - name: Rename code coverage results file from current run
        run: mv .github/outputs/coverage.txt .github/outputs/new-coverage.txt

      # Download code coverage results from the target branch so we compare our new coverage
      # profile with the old one. Again we chose to download into ".github/outputs" to follow
      # best practices. When renaming the file, make sure to use the same name you chose for
      # the "-coverprofile" in the "Test" step.
      - name: Download code coverage results from target branch
        run: |
          TARGET_BRANCH="${{ github.base_ref }}"
          LAST_SUCCESSFUL_RUN_ID=$(gh run list --status=success --branch="$TARGET_BRANCH" --workflow=CI --event=push --json=databaseId --limit=1 -q '.[] | .databaseId')
          if [ -z "$LAST_SUCCESSFUL_RUN_ID" ]; then
            echo "No successful run found on the target branch \"$TARGET_BRANCH\""
            exit 1
          else
            echo "Last successful run on the target branch: $LAST_SUCCESSFUL_RUN_ID"
          fi
          
          gh run download $LAST_SUCCESSFUL_RUN_ID --name=code-coverage --dir=.github/outputs
          mv .github/outputs/coverage.txt .github/outputs/old-coverage.txt

      # Finally we compare the code coverage results and create our code coverage report.
      # You need to adjust the "prefix" flag to match the import path of your package.
      # For more information see the usage of the "go-coverage-report" tool.
      - name: Compare code coverage results
        run: |
          go-coverage-report \
            -prefix=github.com/fgrosse/prioqueue \
            .github/outputs/old-coverage.txt \
            .github/outputs/new-coverage.txt \
            .github/outputs/all_changed_files.json \
          > .github/outputs/coverage-comment.md

      # Now that we have the code coverage report as file, we can use it to create a comment
      # on the pull request using the GitHub action token. When new commits are pushed to the
      # pull requests, the test coverage will be recalculated and we will delete our original
      # comment in favor of a new comment added at the bottom of the PR. 
      - name: Comment on pull request
        run: |
          COMMENT_ID=$(gh api repos/${{ github.repository }}/issues/${{ github.event.pull_request.number }}/comments -q '.[] | select(.user.login=="github-actions[bot]" and (.body | test("Coverage Δ")) ) | .id' | head -n 1)
          if [ -z "$COMMENT_ID" ]; then
            echo "Creating new coverage report comment"
          else
            echo "Replacing old coverage report comment (ID: $COMMENT_ID)"
            gh api -X DELETE repos/${{ github.repository }}/issues/comments/$COMMENT_ID
          fi

          gh pr comment ${{ github.event.number }} --body-file=.github/outputs/coverage-comment.md
```
