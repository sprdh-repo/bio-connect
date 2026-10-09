#!/usr/bin/env bash
# Known attendees a gate denied on a day who have no check-in for that day,
# from production, read-only. Use it to decide who to check in by hand.
#
#   ops/gate-denials.sh [gate name filter] [YYYY-MM-DD]
#
# The filter is a case-insensitive substring of the gate name (empty matches
# every gate). The day defaults to today in IST. Voided (undone) scans and
# codes that match no pass are left out.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SECRETS="$HERE/secrets"
ENV_FILE="$SECRETS/bioconnect-infra.env"
[ -f "$ENV_FILE" ] || { echo "missing $ENV_FILE" >&2; exit 1; }
set -a; source "$ENV_FILE"; set +a

gate="${1:-}"
day="${2:-$(TZ=Asia/Kolkata date +%F)}"
[ $# -le 2 ] && [[ "$gate" =~ ^[A-Za-z0-9\ _-]*$ ]] && [[ "$day" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]] || { echo "usage: $0 [gate name filter] [YYYY-MM-DD]" >&2; exit 2; }

SSH_KEY="$SECRETS/bioconnect-registration.pem"
SSH_OPTS="-o StrictHostKeyChecking=accept-new -o ConnectTimeout=20 -o ServerAliveInterval=15 -o ServerAliveCountMax=3"

# gate and day are validated above, so they are safe inside the remote command line.
ssh -i "$SSH_KEY" $SSH_OPTS "${SSH_USER}@${SERVER_IP}" \
  "sudo docker compose -f /opt/bioconnect/compose.production.yaml exec -T db psql -U bioconnect -d bioconnect -X -q -v ON_ERROR_STOP=1 -v gate='$gate' -v day='$day'" <<'SQL'
WITH denied AS (
  SELECT DISTINCT ON (s.access_point_id, s.attendee_id)
         s.access_point_id, s.attendee_id, s.reason, s.created_at,
         count(*) OVER (PARTITION BY s.access_point_id, s.attendee_id) AS denials
  FROM access_scans s JOIN access_points g ON g.id = s.access_point_id
  WHERE s.event_day = :'day'::date AND s.decision = 'deny' AND s.voided_at IS NULL
    AND s.attendee_id IS NOT NULL AND g.name ILIKE '%' || :'gate' || '%'
  ORDER BY s.access_point_id, s.attendee_id, s.created_at DESC, s.id DESC
)
SELECT g.name AS gate, p.number AS pass, a.name, c.label AS category,
       d.denials, d.reason AS last_reason,
       to_char(d.created_at AT TIME ZONE 'Asia/Kolkata', 'HH24:MI') AS last_denied_ist
FROM denied d
JOIN access_points g ON g.id = d.access_point_id
JOIN attendees a ON a.id = d.attendee_id
JOIN registrations r ON r.id = a.registration_id
JOIN categories c ON c.id = r.category_id
LEFT JOIN passes p ON p.attendee_id = a.id AND p.revoked_at IS NULL
WHERE NOT EXISTS (SELECT 1 FROM ops_attendance oa WHERE oa.attendee_id = d.attendee_id AND oa.event_day = :'day'::date)
ORDER BY g.name, d.created_at;
SQL
