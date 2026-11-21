#!/bin/sh
set -eu

repo=ranakdinesh/epcerp
run_id=${1:-}
destination=${2:-}
case "$run_id" in
  ''|*[!0-9]*) printf 'Usage: sh fetch-main-release.sh GITHUB_RUN_ID NEW_DIRECTORY\n' >&2; exit 2 ;;
esac
if [ -z "$destination" ] || [ -e "$destination" ]; then
  printf 'Choose a new destination directory.\n' >&2
  exit 2
fi

query='select(.path == ".github/workflows/release.yml" and .head_repository.full_name == "ranakdinesh/epcerp" and .head_branch == "main" and .conclusion == "success" and (.event == "push" or .event == "workflow_dispatch")) | .head_sha'
commit=$(gh api "repos/$repo/actions/runs/$run_id" --jq "$query")
if ! printf '%s\n' "$commit" | grep -Eq '^[0-9a-f]{40}$'; then
  printf 'Expected a successful main release workflow run.\n' >&2
  exit 1
fi

umask 077
mkdir "$destination"
gh run download "$run_id" --repo "$repo" --name "epcerp-main-$commit" --dir "$destination"
(
  cd "$destination"
  test "$(cat commit.txt)" = "$commit"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum -c images.sha256
  else
    shasum -a 256 -c images.sha256
  fi
)
printf 'Verified main release %s in %s\n' "$commit" "$destination"
