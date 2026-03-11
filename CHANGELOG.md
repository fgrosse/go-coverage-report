# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]
_Nothing yet_

## [v1.3.0] - 2026-03-11
- Add `event-name` and `target-branch` inputs to support workflows triggered by events other than `push` (fgrosse/go-coverage-report#58)
- Fix statement count being incorrect due to duplicate blocks (fgrosse/go-coverage-report#72)
- Add graceful fallback when baseline coverage artifact is unavailable/expired (fgrosse/go-coverage-report#71)
- Add `--exclude` flag to exclude files matching a regular expression from the report (fgrosse/go-coverage-report#59)
- Add structured outputs (`total_coverage`, `coverage_delta`, `coverage_trend`, `total_statements`, `covered_statements`, `missed_statements`) for use in downstream steps (fgrosse/go-coverage-report#64)

## [v1.2.0] - 2024-08-28
- Add input to configure GitHub Actions workflow filename (fgrosse/go-coverage-report#44)

## [v1.1.1] - 2024-08-23
- Fix issue when the workflow file is renamed (fgrosse/go-coverage-report#41)

## [v1.1.0] - 2024-07-25
- Document permissions needed to use this action (fgrosse/go-coverage-report#32)
- Add new `skip-comment` input to skip adding a comment to the PR (fgrosse/go-coverage-report#34)
- Fix issue with code coverage information missing when test files are deleted (fgrosse/go-coverage-report#35)

## [v1.0.2] - 2024-06-11
- Fix issue when coverage artifact contains more files than just the `coverage.txt` file (fgrosse/go-coverage-report#25)
- Improve `README.md` information about limitations of this action (fgrosse/go-coverage-report#24 and fgrosse/go-coverage-report#15)

## [v1.0.1] - 2024-04-26
- Show coverage report also if only test files changed (fgrosse/go-coverage-report#20)

## [v1.0.0] - 2024-03-18
- Initial release

[Unreleased]: https://github.com/fgrosse/go-coverage-report/compare/v1.3.0...HEAD
[v1.3.0]: https://github.com/fgrosse/go-coverage-report/compare/v1.2.0...v1.3.0
[v1.2.0]: https://github.com/fgrosse/go-coverage-report/compare/v1.1.1...v1.2.0
[v1.1.1]: https://github.com/fgrosse/go-coverage-report/compare/v1.1.0...v1.1.1
[v1.1.0]: https://github.com/fgrosse/go-coverage-report/compare/v1.0.2...v1.1.0
[v1.0.2]: https://github.com/fgrosse/go-coverage-report/compare/v1.0.1...v1.0.2
[v1.0.1]: https://github.com/fgrosse/go-coverage-report/compare/v1.0.0...v1.0.1
[v1.0.0]: https://github.com/fgrosse/go-coverage-report/releases/tag/v1.0.0
