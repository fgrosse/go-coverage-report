# Release Process

This document describes the steps to cut a new release of `go-coverage-report`.

## Versioning

This project follows [Semantic Versioning](https://semver.org/):
- **Patch** (`v1.x.Y`): bug fixes only
- **Minor** (`v1.Y.0`): new features, backwards-compatible
- **Major** (`vX.0.0`): breaking changes

## Steps

### 1. Update `CHANGELOG.md`

- Rename the `## [Unreleased]` section to `## [vX.Y.Z] - YYYY-MM-DD`
- Add a new `## [Unreleased]` section at the top with the content `_Nothing yet_`
- Update the comparison links at the bottom of the file:
  ```
  [Unreleased]: https://github.com/fgrosse/go-coverage-report/compare/vX.Y.Z...HEAD
  [vX.Y.Z]: https://github.com/fgrosse/go-coverage-report/compare/vA.B.C...vX.Y.Z
  ```

### 2. Update version references

Bump the version to `vX.Y.Z` in:
- `action.yml` — the `version` input default value
- `README.md` — the `version` input default value in the inputs reference, and the `uses:` example in the usage section

### 3. Commit and push to main

```bash
git add CHANGELOG.md action.yml README.md
git commit -m "Release vX.Y.Z"
git push origin main
```

### 4. Create and push a signed tag

```bash
git tag -s vX.Y.Z -m "Release vX.Y.Z"
git push origin vX.Y.Z
```

> **Important:** The tag must point to the commit that includes all the version bumps above.
> If you need to move the tag (e.g. you pushed it too early), run:
> ```bash
> git tag -f -s vX.Y.Z -m "Release vX.Y.Z"
> git push --force origin vX.Y.Z
> ```
> Only do this before running goreleaser, as force-pushing a tag after a GitHub Release is published will cause issues.

### 5. Run goreleaser

Extract the release notes for the current version from `CHANGELOG.md` and pass them to goreleaser:

```bash
VERSION=vX.Y.Z
awk "/^## \[$VERSION\]/{found=1; next} /^## \[v/{if(found) exit} found" CHANGELOG.md > /tmp/release-notes.md
goreleaser release --clean --release-notes=/tmp/release-notes.md
```

This will:
- Build binaries for Linux, macOS, and Windows
- Create tarballs and a `checksums.txt`
- Publish a GitHub Release using the curated `CHANGELOG.md` notes

## Checklist

- [ ] `CHANGELOG.md` updated (unreleased → version + date, new empty unreleased section, links)
- [ ] `action.yml` default version bumped
- [ ] `README.md` version references bumped
- [ ] Changes committed and pushed to `main`
- [ ] Signed tag created and pushed (pointing to the version-bump commit)
- [ ] `goreleaser release --clean --release-notes=/tmp/release-notes.md` run successfully
