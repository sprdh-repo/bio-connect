#!/usr/bin/env bash
# Replace the Moments host's database with a dump from the backups bucket.
# DESTRUCTIVE on the host: drops and recreates the bioconnect database.
#
#   registration/ops/moments/restore.sh postgres/2026-10-26T13-01-53Z.dump
#
# The dump streams from S3 to the host through a 10-minute presigned URL made
# with your local AWS credentials; it never lands on this machine. The URL is
# handed to curl in a root-only file, not argv, because other accounts on the
# host can read the process list.
set -euo pipefail
key="${1:?usage: restore.sh <key in the backups bucket>}"
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SECRETS="$HERE/../secrets"
set -a; source "$SECRETS/bioconnect-infra.env"; source "$SECRETS/moments-host.env"; set +a
SSH="ssh -o ConnectTimeout=20 -o ServerAliveInterval=15 -o ServerAliveCountMax=3 $MOMENTS_HOST"

url="$(aws s3 presign "s3://${BACKUP_BUCKET}/${key}" --expires-in 600 --region "$AWS_REGION")"
printf 'url = "%s"\n' "$url" | $SSH "sudo install -m 600 -o root -g root /dev/stdin /root/bioconnect-restore.curl"

$SSH "sudo bash -s" <<'EOF'
set -euo pipefail
trap 'rm -f /root/bioconnect-restore.curl /var/lib/bioconnect-backup/restore.dump' EXIT
source /etc/bioconnect/backup.env
curl -fsS --config /root/bioconnect-restore.curl -o /var/lib/bioconnect-backup/restore.dump
docker exec -i bioconnect-db pg_restore --list < /var/lib/bioconnect-backup/restore.dump > /dev/null
systemctl stop bioconnect.service
# No -i here: this script itself arrives on stdin, and docker exec -i would swallow the rest of it.
psql() { docker exec -e PGPASSWORD="$POSTGRES_PASSWORD" bioconnect-db psql -v ON_ERROR_STOP=1 -U bioconnect "$@" < /dev/null; }
psql -d postgres -qc 'DROP DATABASE IF EXISTS bioconnect WITH (FORCE)'
psql -d postgres -qc 'CREATE DATABASE bioconnect OWNER bioconnect'
docker exec -i -e PGPASSWORD="$POSTGRES_PASSWORD" bioconnect-db pg_restore -U bioconnect -d bioconnect --exit-on-error --no-owner < /var/lib/bioconnect-backup/restore.dump
systemctl start bioconnect.service
for i in $(seq 1 30); do
  curl -fsS -o /dev/null --max-time 5 http://127.0.0.1:8090/healthz && { echo "restored; app healthz OK"; exit 0; }
  sleep 2
done
echo "app never became healthy after the restore" >&2; journalctl -u bioconnect --no-pager -n 30 >&2; exit 1
EOF
