#!/usr/bin/env bash
set -euo pipefail
# Run on the dedicated host through the systemd timer.
# set -a so the AWS_* keys in backup.env are exported to the `aws` child process
# (otherwise it falls back to the Lightsail instance role, which has no access here).
set -a; source /etc/bioconnect/backup.env; set +a
: "${BACKUP_BUCKET:?}" "${BACKUP_KMS_KEY_ID:?}" "${COMPOSE_DIR:?}" "${AWS_ACCESS_KEY_ID:?}"
export AWS_DEFAULT_REGION="${AWS_DEFAULT_REGION:-ap-south-1}"
backup_tmp=$(mktemp /var/lib/bioconnect-backup/backup.XXXXXX)
trap 'rm -f "$backup_tmp"' EXIT
chmod 600 "$backup_tmp"
cd "$COMPOSE_DIR"
docker compose -f compose.production.yaml exec -T db pg_dump -U bioconnect -d bioconnect -Fc > "$backup_tmp"
# Validate the archive before uploading it. Bucket policy also requires SSE-KMS.
docker compose -f compose.production.yaml exec -T db pg_restore --list < "$backup_tmp" > /dev/null
backup_stamp=$(date -u +%Y-%m-%dT%H-%M-%SZ)
aws s3 cp "$backup_tmp" "s3://$BACKUP_BUCKET/postgres/$backup_stamp.dump" --sse aws:kms --sse-kms-key-id "$BACKUP_KMS_KEY_ID" --only-show-errors
logger -t bioconnect-backup "Encrypted PostgreSQL backup completed"
