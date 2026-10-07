#!/usr/bin/env bash
# Let a pass print at the kiosk again by deleting its badge_print audit rows.
# For testing: a printed pass otherwise only checks in (see kioskPrint).
# Check-ins are left alone; undo those from the ops roster if needed.
#
#   ops/kiosk-reset.sh <pass number | QR id>
#
# Shows the attendee and their print history, then asks before deleting.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SECRETS="$HERE/secrets"
ENV_FILE="$SECRETS/bioconnect-infra.env"
[ -f "$ENV_FILE" ] || { echo "missing $ENV_FILE" >&2; exit 1; }
set -a; source "$ENV_FILE"; set +a

code="${1:-}"
[ $# = 1 ] && [[ "$code" =~ ^[A-Za-z0-9_-]+$ ]] || { echo "usage: $0 <pass number | QR id>" >&2; exit 2; }

SSH_KEY="$SECRETS/bioconnect-registration.pem"
SSH_OPTS="-o StrictHostKeyChecking=accept-new -o ConnectTimeout=20 -o ServerAliveInterval=15 -o ServerAliveCountMax=3"
# code is validated above, so it is safe inside the remote command line.
psql() {
  ssh -i "$SSH_KEY" $SSH_OPTS "${SSH_USER}@${SERVER_IP}" \
    "sudo docker compose -f /opt/bioconnect/compose.production.yaml exec -T db psql -U bioconnect -d bioconnect -X -q -v ON_ERROR_STOP=1 -v code=$code $*"
}

MATCH="FROM passes p JOIN attendees a ON a.id=p.attendee_id
 WHERE p.revoked_at IS NULL AND (p.number=:'code' OR p.qr_id=:'code')"

echo "== pass $code =="
psql <<SQL
SELECT a.id AS attendee, p.number, a.name, a.email $MATCH;
SELECT o.created_at AT TIME ZONE 'Asia/Kolkata' AS printed_at, o.station, o.detail
 FROM ops_activity o WHERE o.kind='badge_print' AND o.attendee_id IN (SELECT a.id $MATCH)
 ORDER BY o.created_at;
SQL

count=$(psql -At <<SQL
SELECT count(*) FROM ops_activity WHERE kind='badge_print' AND attendee_id IN (SELECT a.id $MATCH);
SQL
)
count="${count//[^0-9]/}"
[ "${count:-0}" -gt 0 ] || { echo "no badge prints recorded for $code; nothing to do"; exit 0; }

read -r -p "Delete these $count badge_print row(s) in PRODUCTION? Type 'yes': " answer
[ "$answer" = yes ] || { echo "aborted"; exit 1; }

psql <<SQL
BEGIN;
DELETE FROM ops_activity WHERE kind='badge_print' AND attendee_id IN (SELECT a.id $MATCH);
COMMIT;
SQL
echo "done: $code can print at the kiosk again"
