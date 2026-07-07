#!/usr/bin/env bash
set -euo pipefail

latest_tag="$(git tag --list 'v[0-9]*' --sort=-version:refname | head -n1)"

if [ -z "${latest_tag}" ]; then
  echo "v0.1.0"
  exit 0
fi

if ! [[ "${latest_tag}" =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
  echo "release: latest tag is not SemVer: ${latest_tag}" >&2
  exit 1
fi

major="${BASH_REMATCH[1]}"
minor="${BASH_REMATCH[2]}"
patch="${BASH_REMATCH[3]}"

range="${latest_tag}..HEAD"
if [ "$(git rev-list --count "${range}")" -eq 0 ]; then
  echo "release: no commits since ${latest_tag}" >&2
  exit 1
fi

subjects="$(git log --format=%s "${range}")"
messages="$(git log --format=%B "${range}")"

if grep -Eq '^[[:alpha:]][[:alnum:]_-]*(\([^)]*\))?!:' <<<"${subjects}" ||
  grep -Eq '^BREAKING[ -]CHANGE:' <<<"${messages}"; then
  if [ "${major}" -eq 0 ]; then
    minor=$((minor + 1))
    patch=0
  else
    major=$((major + 1))
    minor=0
    patch=0
  fi
elif grep -Eq '^feat(\([^)]*\))?:' <<<"${subjects}"; then
  minor=$((minor + 1))
  patch=0
else
  patch=$((patch + 1))
fi

echo "v${major}.${minor}.${patch}"
