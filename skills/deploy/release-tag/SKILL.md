---
name: release-tag
description: >
  Create a new semver release tag and optionally push it to origin.
  Trigger phrases: "create release", "tag release", "cut a release",
  "new version", "bump version", "release tag"
---

# Release Tag

Create a new semantic version tag on the current branch and optionally push it to origin.

## User Input
```text
$ARGUMENTS
```

## Steps

1. **Check preconditions**:
   - Working tree must be clean (`git status --porcelain` returns empty)
   - Current branch should be `main` (warn if not, proceed if user confirms)

2. **Determine next version**:
   - List existing tags: `git tag -l --sort=-v:refname | head -1`
   - Current convention is `v0.0.X` (patch-only bumps)
   - If user specifies a version (e.g., `v0.1.0`), use that
   - Otherwise auto-increment the patch number from the latest tag

3. **Show changelog since last tag**:
   ```bash
   git log $(git tag -l --sort=-v:refname | head -1)..HEAD --oneline
   ```
   - If no commits since last tag, abort with message

4. **Confirm with user**: Display the proposed tag and changelog, ask for confirmation before proceeding.

5. **Run verification**:
   ```bash
   make verify
   make lint
   make test
   ```
   - All must pass before tagging

6. **Create the tag**:
   ```bash
   git tag -a <version> -m "<summary of changes since last tag>"
   ```

7. **Push to origin** (if user confirms or passes `--push`):
   ```bash
   git push origin <version>
   ```

8. **Report**: Display the new tag, commit it points to, and the changelog.

## Version Conventions

- Tags follow semver: `vMAJOR.MINOR.PATCH`
- Current pattern: `v0.0.X` (pre-1.0, patch increments)
- Breaking changes should bump MINOR at minimum
- Tags are annotated with a message summarizing changes

## Prerequisites
- Clean working tree
- All tests passing
- On `main` branch (recommended)
