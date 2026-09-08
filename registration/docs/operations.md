# Operations

The registration app runs on a dedicated AWS Lightsail instance in Mumbai (`ap-south-1`), behind the existing `reg.bioconnect.kerala.gov.in` CloudFront distribution, separate from the marketing site.
Go, PostgreSQL, and Caddy run under Docker Compose.
PostgreSQL is on an internal Docker network and is never published.
The concrete resource names, the topology diagram, and the provisioning scripts are in [`infrastructure.md`](infrastructure.md).

Nothing here is executed by CI.
Deployment and any live message test are gated on readiness (see the checklist at the end) and explicit authorisation.

## Host layout

```
/opt/bioconnect/                 COMPOSE_DIR
  compose.production.yaml
  Caddyfile
  ops/                           backup.sh, monitor.sh, alert.sh
/etc/bioconnect/app.env          app environment, APP_ENV=production
/etc/bioconnect/postgres.env     POSTGRES_USER=bioconnect, POSTGRES_PASSWORD=..., POSTGRES_DB=bioconnect
/etc/bioconnect/caddy.env        ORIGIN_HOSTNAME, EDGE_AUTH_SECRET, ACME_EMAIL
/etc/bioconnect/backup.env       BACKUP_BUCKET, BACKUP_KMS_KEY_ID, COMPOSE_DIR, ALERT_SNS_TOPIC_ARN, backup IAM keys
/var/lib/bioconnect-backup/      scratch space for pg_dump (mode 700)
```

All of `/etc/bioconnect/*.env` is rendered from `registration/ops/secrets/bioconnect-infra.env` by `ops/deploy.sh`.
That master file also holds the SSH key path and every resource id; safe-keep it privately.
Losing `ENCRYPTION_KEY` makes stored TOTP secrets and pass tokens unrecoverable.

`REGISTRATION_ENABLED` stays `false` until launch.
`LIVE_DELIVERY` stays `false` until the Zinvos senders, webhook credentials, and WhatsApp template are confirmed; the app refuses to start with `LIVE_DELIVERY=true` unless every provider and webhook value is set, and refuses live registration unless the SBI URL is set too.

## Deploy

From a workstation with the repo, AWS credentials, and `registration/ops/secrets/` populated:

```sh
cd registration
ops/deploy.sh          # build image, ship to the host, render env files, compose up, health-check
```

`deploy.sh` builds `bioconnect-registration:<git-sha>` locally, `docker save`s it to the host (no registry), pushes `/etc/bioconnect/*.env`, syncs the compose file and ops scripts, runs `docker compose up -d`, enables the backup/monitor timers, and checks `/healthz` at the origin and through CloudFront.
Migrations are additive and run once under a Postgres advisory lock, so re-running is safe.
First-time only, after `deploy.sh`: `ops/cloudfront-origin.sh` repoints CloudFront from the placeholder bucket to the box.

### Rollback

Application only, schema unchanged:

```sh
export APP_IMAGE=<previous good tag>
docker compose -f compose.production.yaml up -d app
curl -fsS https://reg.bioconnect.kerala.gov.in/healthz
```

Keep the previous image tag recorded with each deploy.
Because migrations are forward-only, a rollback that must also undo a schema change is a restore (below) to the pre-deploy hourly backup, then redeploy the previous image.
Take a manual backup (`/opt/bioconnect/ops/backup.sh`) immediately before any deploy that includes a new migration.

## Backups and snapshots

- **Hourly encrypted database backup**: `ops/bioconnect-backup.timer` runs `ops/backup.sh`, which `pg_dump -Fc`, verifies the archive with `pg_restore --list`, and uploads to `s3://$BACKUP_BUCKET/postgres/<ISO8601>.dump` with SSE-KMS.
- **Retention**: an S3 lifecycle rule on `$BACKUP_BUCKET` expires `postgres/` objects after 30 days. The bucket is private with a policy that denies non-KMS puts and public access.
- **Daily instance snapshot**: enable automatic snapshots on the Lightsail instance (retained by Lightsail's default window). These capture uploaded files only if `STORAGE_DIR` is on the instance; in production uploads and passes live in a private, versioned S3 bucket instead, so the snapshot is for the OS and compose state.
- **Uploads/passes bucket**: enable versioning and a 30-day noncurrent-version expiry.

`deploy.sh` installs and enables the timers. `ops/bioconnect-backup.service` and `bioconnect-monitor.service` call `ops/alert.sh` on failure, which publishes to `$ALERT_SNS_TOPIC_ARN` with no PII or provider payloads in the message.
Enable Lightsail automatic snapshots on the instance once, in the console.

## Restore rehearsal and recovery

Rehearse this on a scratch instance or a throwaway compose project at least once before launch and after any migration change.

```sh
# 1. Pick a backup.
aws s3 ls s3://$BACKUP_BUCKET/postgres/
aws s3 cp s3://$BACKUP_BUCKET/postgres/<stamp>.dump /var/lib/bioconnect-backup/restore.dump

# 2. Stop the app so nothing writes during the restore.
docker compose -f compose.production.yaml stop app

# 3. Recreate the database and load the dump.
docker compose -f compose.production.yaml exec -T db \
  psql -U bioconnect -d postgres -c \
  "DROP DATABASE IF EXISTS bioconnect WITH (FORCE); CREATE DATABASE bioconnect OWNER bioconnect;"
docker compose -f compose.production.yaml exec -T db \
  pg_restore -U bioconnect -d bioconnect --no-owner --clean --if-exists < /var/lib/bioconnect-backup/restore.dump

# 4. Bring the app back and verify.
docker compose -f compose.production.yaml up -d app
curl -fsS https://reg.bioconnect.kerala.gov.in/healthz
docker compose -f compose.production.yaml exec -T db \
  psql -U bioconnect -d bioconnect -Atc \
  "SELECT (SELECT count(*) FROM registrations), (SELECT count(*) FROM passes), (SELECT max(created_at) FROM audit_events)"
```

Restore uses the same `ENCRYPTION_KEY` as the backup, otherwise TOTP secrets and pass tokens will not decrypt.
Passes are regenerated on demand from the database, so pass PDFs do not need separate backup; the private uploads bucket holds receipts and logos.

## Monitoring

- `ops/bioconnect-monitor.timer` runs every 5 minutes: it fails if `/healthz` is down or if any delivery job is `failed`/`uncertain` or has been `queued`/`sending` for over 15 minutes. Failures raise the SNS alert.
- Staff review `failed` and `uncertain` deliveries in the console (registration detail, Delivery history) and use **Retry**. `uncertain` requires checking the provider first and acknowledging duplicate-send risk.
- Container logs go to the local json-file driver (10 MB x 5). Logs never contain message bodies, tokens, or receipts. Caddy access logs are disabled because pass URLs carry bearer tokens.

## Launch checklist

Before setting `REGISTRATION_ENABLED=true` and `LIVE_DELIVERY=true`:

- [ ] `ops/deploy.sh` and `ops/cloudfront-origin.sh` have run; `https://reg.bioconnect.kerala.gov.in/healthz` returns 200 through CloudFront (not the placeholder JSON).
- [ ] The real SBI Collect link (or merchant setup) is confirmed and set as `SBI_COLLECT_URL`; staff have access to the SBI reconciliation report.
- [x] Zinvos Postmark: verified sender `bioconnect@zinvos.com` / "Bio Connect 4.0", server token set. `POSTMARK_WEBHOOK_USER`/`PASSWORD` configured on the Postmark webhook, pointed at `/api/v1/webhooks/postmark`. Live send to a real inbox delivered and the webhook flipped the job to `delivered`.
- [x] Zinvos Meta: access token, phone number id, app secret, API version set. `META_VERIFY_TOKEN` entered in the Meta webhook config against `/api/v1/webhooks/meta`; the approved pass template with a DOCUMENT header is live (`META_TEMPLATE=bioconnect_pass_delivery_doc`). The worker uploads the pass PDF to the WhatsApp media store and sends it as the header, keeping the pass link in the body as a fallback; the older text-only `bioconnect_pass_delivery` stays approved as a backup. Live template send (PDF + link) to a real handset was accepted by Meta.
- [ ] `ops/secrets/bioconnect-infra.env` (holds `ENCRYPTION_KEY`) backed up to a private vault.
- [ ] Initial staff identities created (`ssh` to the box, `sudo docker compose -f /opt/bioconnect/compose.production.yaml exec app bioconnect staff-create` with `{"Email":...,"Password":...,"Role":"manager"|"reviewer"}` on stdin), each enrolled in an authenticator, at least one `manager` and one `reviewer`.
- [ ] Hourly backup timer green; one restore rehearsal completed; Lightsail automatic snapshots enabled in the console.
- [ ] Published fees confirmed against SBI as the payable amounts.
- [ ] The two registration links in the marketing site's `index.html` are deployed.
- [ ] A single end-to-end live message test (one delegate and one exhibitor) authorised and passed.

Out of scope: tax invoices, automated refunds, bank-report imports, attendance scanning, automated stall allocation, sponsorship registration.
