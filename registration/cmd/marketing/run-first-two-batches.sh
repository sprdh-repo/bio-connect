#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
STATE="$ROOT/var/marketing/first-two-batches"
CAMPAIGN="invitation-september-2026-first-two-batches"
AGGAPPE="${AGGAPPE_CONTACTS:-/home/mshiyaf/Downloads/Aggappe Invitation sent by MD KSIDC.xlsx}"
BANGALORE="${BANGALORE_CONTACTS:-/home/mshiyaf/Downloads/banglore tech lead.xlsx}"

if [[ ${1:-} != "" && ${1:-} != "--send" ]]; then
  echo "Usage: $0 [--send]" >&2
  exit 2
fi
for file in "$AGGAPPE" "$BANGALORE"; do
  if [[ ! -f $file ]]; then
    echo "Contact file not found: $file" >&2
    exit 1
  fi
done

mkdir -p "$STATE"
cd "$ROOT"

go run ./cmd/marketing-prepare \
  --output "$STATE/validated-contacts.csv" \
  --report "$STATE/validation.csv" \
  "$AGGAPPE" "$BANGALORE"

if [[ ${1:-} != "--send" ]]; then
  go run ./cmd/marketing \
    --contacts "$STATE/validated-contacts.csv" \
    --campaign "$CAMPAIGN" \
    --state "$STATE"
  echo
  echo "Validation only. Review $STATE/validation.csv and $STATE/preview.html"
  echo "No email was sent. Run $0 --send only after reviewing both files."
  exit 0
fi

if [[ -f "$ROOT/ops/secrets/bioconnect-infra.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "$ROOT/ops/secrets/bioconnect-infra.env"
  set +a
fi

go run ./cmd/marketing \
  --contacts "$STATE/validated-contacts.csv" \
  --campaign "$CAMPAIGN" \
  --state "$STATE" \
  --send
