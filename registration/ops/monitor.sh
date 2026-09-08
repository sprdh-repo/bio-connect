#!/usr/bin/env bash
set -euo pipefail
source /etc/bioconnect/backup.env
: "${COMPOSE_DIR:?}"
cd "$COMPOSE_DIR"
curl --fail --silent --show-error --max-time 15 https://reg.bioconnect.kerala.gov.in/healthz > /dev/null
problem_jobs=$(docker compose -f compose.production.yaml exec -T db psql -U bioconnect -d bioconnect -Atc "SELECT count(*) FROM delivery_jobs WHERE status IN ('failed','uncertain') OR (status IN ('queued','sending') AND updated_at<now()-interval '15 minutes')")
if [[ "$problem_jobs" != 0 ]]; then
  logger -p daemon.err -t bioconnect-monitor "Delivery queue needs staff review"
  exit 1
fi
# No PII or provider payloads in the alert.
