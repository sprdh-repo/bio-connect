#!/usr/bin/env bash
# Per-gate scan totals from production, read-only.
#
#   ops/gate-counts.sh [YYYY-MM-DD]
#
# Without a day, shows every event day plus an all-days total per gate.
# Voided (undone) scans are excluded. "inside" matches the ops occupancy count:
# attendees whose latest allowed scan at that gate is an entry.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SECRETS="$HERE/secrets"
ENV_FILE="$SECRETS/bioconnect-infra.env"
[ -f "$ENV_FILE" ] || { echo "missing $ENV_FILE" >&2; exit 1; }
set -a; source "$ENV_FILE"; set +a

day="${1:-}"
[ $# -le 1 ] && [[ -z "$day" || "$day" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]] || { echo "usage: $0 [YYYY-MM-DD]" >&2; exit 2; }

SSH_KEY="$SECRETS/bioconnect-registration.pem"
SSH_OPTS="-o StrictHostKeyChecking=accept-new -o ConnectTimeout=20 -o ServerAliveInterval=15 -o ServerAliveCountMax=3"

# day is validated above, so it is safe inside the remote command line.
ssh -i "$SSH_KEY" $SSH_OPTS "${SSH_USER}@${SERVER_IP}" \
  "sudo docker compose -f /opt/bioconnect/compose.production.yaml exec -T db psql -U bioconnect -d bioconnect -X -q -v ON_ERROR_STOP=1 -v day='$day'" <<'SQL'
WITH s AS (
  SELECT * FROM access_scans
  WHERE voided_at IS NULL AND (:'day' = '' OR event_day = NULLIF(:'day','')::date)
), latest AS (
  SELECT DISTINCT ON (access_point_id, event_day, attendee_id) access_point_id, event_day, direction
  FROM s WHERE decision='allow' AND attendee_id IS NOT NULL
  ORDER BY access_point_id, event_day, attendee_id, created_at DESC, id DESC
)
SELECT p.name AS gate,
       COALESCE(s.event_day::text, 'ALL DAYS') AS day,
       count(*) FILTER (WHERE s.decision='allow' AND s.direction='entry') AS entries,
       count(DISTINCT s.attendee_id) FILTER (WHERE s.decision='allow' AND s.direction='entry') AS unique_people,
       count(*) FILTER (WHERE s.decision='allow' AND s.direction='exit') AS exits,
       count(*) FILTER (WHERE s.decision='deny') AS denied,
       count(*) FILTER (WHERE s.would_deny<>'') AS would_deny,
       CASE WHEN s.event_day IS NULL THEN NULL ELSE
         (SELECT count(*) FROM latest l WHERE l.access_point_id=p.id AND l.event_day=s.event_day AND l.direction='entry')
       END AS inside
FROM access_points p
JOIN s ON s.access_point_id = p.id
GROUP BY GROUPING SETS ((p.id, p.name, s.event_day), (p.id, p.name))
ORDER BY p.name, s.event_day NULLS LAST;
SQL
