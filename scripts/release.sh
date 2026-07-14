#!/bin/sh
# Publish a new release: bump the version tag and push it, which triggers
# the GoReleaser workflow (.github/workflows/release.yml) to build and
# publish the binaries.
#
#   scripts/release.sh patch|minor|major [--dry-run]
#
# There is no version file to edit — the tag is the version (goreleaser
# stamps it into the binary), so this only tags and pushes.
set -eu

err() { printf 'release: %s\n' "$1" >&2; exit 1; }

bump="${1:-}"
dry_run="${2:-}"
case "$bump" in
  patch|minor|major) ;;
  *) err "usage: scripts/release.sh patch|minor|major [--dry-run]" ;;
esac

# Releases are cut from an up-to-date, clean main: the tag must point at a
# commit that is already on origin, or the workflow builds something else
# than what you have.
branch=$(git rev-parse --abbrev-ref HEAD)
[ "$branch" = "main" ] || err "must be on main (currently on $branch)"
[ -z "$(git status --porcelain)" ] || err "working tree is not clean — commit or stash first"
git fetch origin main --tags --quiet
[ "$(git rev-parse HEAD)" = "$(git rev-parse origin/main)" ] || \
  err "main is not in sync with origin/main — push (or pull) first"

last=$(git describe --tags --abbrev=0 2>/dev/null || echo v0.0.0)
case "$last" in
  v[0-9]*.[0-9]*.[0-9]*) ;;
  *) err "last tag $last is not vX.Y.Z" ;;
esac
major=$(echo "${last#v}" | cut -d. -f1)
minor=$(echo "${last#v}" | cut -d. -f2)
patch=$(echo "${last#v}" | cut -d. -f3)
case "$bump" in
  major) major=$((major + 1)); minor=0; patch=0 ;;
  minor) minor=$((minor + 1)); patch=0 ;;
  patch) patch=$((patch + 1)) ;;
esac
next="v$major.$minor.$patch"

printf 'Releasing %s (%s bump from %s). Commits since %s:\n\n' "$next" "$bump" "$last" "$last"
git log --oneline "$last"..HEAD | sed 's/^/    /'
printf '\n'

if [ "$dry_run" = "--dry-run" ]; then
  printf 'Dry run — would tag %s and push it.\n' "$next"
  exit 0
fi

printf 'Tag and publish? [y/N] '
read -r answer
case "$answer" in
  y|Y|yes) ;;
  *) printf 'Aborted.\n'; exit 1 ;;
esac

git tag -a "$next" -m "$next"
git push origin "$next"

printf '\n✔ Pushed %s — GoReleaser is building the release.\n' "$next"
printf 'Watch it:  gh run watch  (or https://github.com/itapulab/itapu-cli/actions)\n'
