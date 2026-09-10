#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
output_dir="$repo_root/bin"
provider_repo="${GRYPHDASH_PROVIDERS_DIR:-$repo_root/../GryphDash-Providers}"
if [[ ! -d "$provider_repo/cmd/gryphdash-provider-currency" ]]; then
  echo "GryphDash-Providers repository not found: $provider_repo" >&2
  exit 1
fi

mkdir -p "$output_dir"

for provider in currency codex openrouter; do
  output_file="$output_dir/gryphdash-provider-$provider"
  (cd "$provider_repo" && go build -o "$output_file" "./cmd/gryphdash-provider-$provider")
  printf 'Built %s\n' "$output_file"
done

export GRYPHDASH_PROVIDER_DIR="$output_dir"
exec go run "$repo_root" web
