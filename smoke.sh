#!/usr/bin/env bash
set -euo pipefail
bin=${1:?pass binary path}
scratch=$(mktemp -d)
trap 'rm -rf "$scratch"' EXIT
export REM_HOME="$scratch/state"
"$bin" --version
"$bin" doctor --json
"$bin" setup --dry-run --json
if [[ -n ${REM_LIVE_TARGET:-} ]]; then
  "$bin" "$REM_LIVE_TARGET" --only website,seo --no-ai --out "$scratch/first"
  "$bin" "$REM_LIVE_TARGET" --only website,seo --no-ai --out "$scratch/second"
  "$bin" validate "$scratch/first/evidence.json" "$scratch/first/report.md" --json
  "$bin" diff "$scratch/first/evidence.json" "$scratch/second/evidence.json" --json
fi
