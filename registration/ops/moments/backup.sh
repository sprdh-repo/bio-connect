#!/usr/bin/env bash
set -euo pipefail
# Hourly, from bioconnect-backup.timer. Same bucket, prefix and KMS key as the
# Lightsail host. The AWS CLI runs from a pinned container because CentOS 7
# has no supported AWS CLI v2; the IAM user can only write postgres/*.
set -a; source /etc/bioconnect/backup.env; set +a
: "${BACKUP_BUCKET:?}" "${BACKUP_KMS_KEY_ID:?}" "${AWS_ACCESS_KEY_ID:?}" "${AWS_CLI_IMAGE:?}" "${POSTGRES_PASSWORD:?}"
backup_tmp=$(mktemp /var/lib/bioconnect-backup/backup.XXXXXX)
trap 'rm -f "$backup_tmp"' EXIT
chmod 600 "$backup_tmp"
docker exec -e PGPASSWORD="$POSTGRES_PASSWORD" bioconnect-db pg_dump -U bioconnect -d bioconnect -Fc > "$backup_tmp"
# Validate the archive before uploading it. Bucket policy also requires SSE-KMS.
docker exec -i bioconnect-db pg_restore --list < "$backup_tmp" > /dev/null
backup_stamp=$(date -u +%Y-%m-%dT%H-%M-%SZ)
docker run --rm --network host -e AWS_ACCESS_KEY_ID -e AWS_SECRET_ACCESS_KEY -e AWS_DEFAULT_REGION \
  -v "$backup_tmp:/backup.dump:ro" "$AWS_CLI_IMAGE" \
  s3 cp /backup.dump "s3://$BACKUP_BUCKET/postgres/$backup_stamp.dump" --sse aws:kms --sse-kms-key-id "$BACKUP_KMS_KEY_ID" --only-show-errors
logger -t bioconnect-backup "Encrypted PostgreSQL backup completed"
