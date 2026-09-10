#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
desktop_dir="$repo_root/cmd/gryphdash-desktop"
output_dir="$repo_root/bin"
output_file="$output_dir/gryphdash-desktop-windows-amd64.exe"
currency_output_file="$output_dir/gryphdash-provider-currency-windows-amd64.exe"
codex_output_file="$output_dir/gryphdash-provider-codex-windows-amd64.exe"

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

GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build \
  -o "$currency_output_file" \
  ./cmd/gryphdash-provider-currency
printf 'Built %s\n' "$currency_output_file"

GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build \
  -o "$codex_output_file" \
  ./cmd/gryphdash-provider-codex
printf 'Built %s\n' "$codex_output_file"
