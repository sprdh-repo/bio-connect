#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
# Reuse the campaign and durable log so recipients from earlier lists are skipped.
STATE="$ROOT/var/marketing/first-two-batches"
CAMPAIGN="invitation-september-2026-first-two-batches"
CONTACTS="${LIFESCIENCE_STARTUPS_CONTACTS:-/home/mshiyaf/Downloads/List of Startups in Lifescience sector, Kerala.xlsx}"
CLEAN="$STATE/lifescience-startups-validated-contacts.csv"
VALIDATION="$STATE/lifescience-startups-validation.csv"

if [[ ${1:-} != "" && ${1:-} != "--send" ]]; then
  echo "Usage: $0 [--send]" >&2
  exit 2
fi
if [[ ! -f $CONTACTS ]]; then
  echo "Contact file not found: $CONTACTS" >&2
  exit 1
fi

mkdir -p "$STATE"
cd "$ROOT"

go run ./cmd/marketing-prepare \
  --output "$CLEAN" \
  --report "$VALIDATION" \
  --correct-address "jibsepl@gmailcom=jibsepl@gmail.com" \
  --correct-address "binu.augustin@hekagmail.com=info@hekamedicals.com" \
  "$CONTACTS"

if [[ ${1:-} != "--send" ]]; then
  go run ./cmd/marketing \
    --contacts "$CLEAN" \
    --campaign "$CAMPAIGN" \
    --state "$STATE"
  echo
  echo "Validation only. Review $VALIDATION and $STATE/preview.html"
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
  --contacts "$CLEAN" \
  --campaign "$CAMPAIGN" \
  --state "$STATE" \
  --send
