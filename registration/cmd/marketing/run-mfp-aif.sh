#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
# Reuse the latest campaign state as a second idempotency barrier during sending.
STATE="$ROOT/var/marketing/first-two-batches"
CAMPAIGN="invitation-september-2026-first-two-batches"
MFP_CONTACTS="${MFP_CONTACTS:-/home/mshiyaf/Downloads/MFP mail id 19926.xlsx}"
AIF_CONTACTS="${AIF_CONTACTS:-/home/mshiyaf/Downloads/AIF- Beneficiary wise data as on 22-07-2026.xlsx}"
CLEAN="$STATE/mfp-aif-validated-contacts.csv"
VALIDATION="$STATE/mfp-aif-validation.csv"

if [[ ${1:-} != "" && ${1:-} != "--send" ]]; then
  echo "Usage: $0 [--send]" >&2
  exit 2
fi
for file in "$MFP_CONTACTS" "$AIF_CONTACTS"; do
  if [[ ! -f $file ]]; then
    echo "Contact file not found: $file" >&2
    exit 1
  fi
done

mkdir -p "$STATE"
cd "$ROOT"

# Exclude every address with a prior attempted outcome. Reports are included as
# a cross-check against the authoritative JSONL logs.
shopt -s nullglob
history_args=()
for file in \
  "$ROOT"/var/marketing/sends.jsonl \
  "$ROOT"/var/marketing/report-*.csv \
  "$ROOT"/var/marketing/combined-report-*.csv \
  "$STATE"/sends.jsonl \
  "$STATE"/report-*.csv; do
  [[ -f $file ]] && history_args+=(--exclude-history "$file")
done

go run ./cmd/marketing-prepare \
  --output "$CLEAN" \
  --report "$VALIDATION" \
  "${history_args[@]}" \
  --correct-domain gmai.com=gmail.com \
  --correct-domain gamil.com=gmail.com \
  --correct-domain gmil.com=gmail.com \
  --correct-domain gmal.com=gmail.com \
  --correct-domain gmaill.com=gmail.com \
  --correct-domain gamail.com=gmail.com \
  --correct-domain gmail.co=gmail.com \
  --correct-domain gmail.coms=gmail.com \
  --correct-domain 2gmail.com=gmail.com \
  --correct-domain gmalim.com=gmail.com \
  --correct-domain gnail.com=gmail.com \
  --correct-domain gmail.com.in=gmail.com \
  "$MFP_CONTACTS" "$AIF_CONTACTS"

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
