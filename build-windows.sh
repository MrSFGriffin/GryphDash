#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
desktop_dir="$repo_root/cmd/gryphdash-desktop"
output_dir="$repo_root/bin"
output_file="$output_dir/gryphdash-desktop-windows-amd64.exe"
provider_repo="${GRYPHDASH_PROVIDERS_DIR:-$repo_root/../GryphDash-Providers}"
if [[ ! -d "$provider_repo/cmd/gryphdash-provider-currency" ]]; then
  echo "GryphDash-Providers repository not found: $provider_repo" >&2
  exit 1
fi
currency_output_file="$output_dir/gryphdash-provider-currency-windows-amd64.exe"
codex_output_file="$output_dir/gryphdash-provider-codex-windows-amd64.exe"
openrouter_output_file="$output_dir/gryphdash-provider-openrouter-windows-amd64.exe"

mkdir -p "$output_dir"

(
  cd "$desktop_dir"
  go run github.com/wailsapp/wails/v2/cmd/wails@v2.15.0 \
    build \
    -clean \
    -skipbindings \
    --platform windows/amd64
)

cp "$desktop_dir/build/bin/gryphdash-desktop.exe" "$output_file"
printf 'Built %s\n' "$output_file"

(cd "$provider_repo" && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build \
  -o "$currency_output_file" \
  ./cmd/gryphdash-provider-currency)
printf 'Built %s\n' "$currency_output_file"

(cd "$provider_repo" && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build \
  -o "$codex_output_file" \
  ./cmd/gryphdash-provider-codex)
printf 'Built %s\n' "$codex_output_file"

(cd "$provider_repo" && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build \
  -o "$openrouter_output_file" \
  ./cmd/gryphdash-provider-openrouter)
printf 'Built %s\n' "$openrouter_output_file"
