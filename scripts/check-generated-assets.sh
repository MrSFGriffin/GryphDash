#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"
npm ci
npm run check
npm test
npm run build
git diff --exit-code -- web/dashboard.js
