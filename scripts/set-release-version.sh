#!/usr/bin/env bash

set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: $0 <vMAJOR.MINOR.PATCH>" >&2
  exit 1
fi

version="${1#v}"
if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "invalid release version: $1" >&2
  exit 1
fi

script_dir="$(cd "$(dirname "$0")" && pwd)"
printf '%s\n' "$version" > "$script_dir/../VERSION"
