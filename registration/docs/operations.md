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
`OPS_KEY` controls the separate on-site portal at `/ops` and must be a strong shared multi-word passcode that is not used for an admin account.
Rotate it by changing the master environment and deploying; existing 12-hour on-site sessions remain valid until they expire or staff end the shift.

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

## On-site portal

Open `/ops` on each desk or gate device, enter its physical station name, and use the shared `OPS_KEY` passcode.
The session lasts 12 hours and all writes carry CSRF protection.

- Select the event day before scanning. Attendance is unique per attendee per day, while repeated scans are retained in the activity audit.
- A first check-in opens the browser print dialog for a landscape 76.2 × 50.8 mm thermal badge, with attendee details above the QR. Configure the printer driver for that exact label size, landscape orientation, 100% scale, zero margins and no browser headers or footers.
  The desk and the self-service kiosk draw the badge with the same renderer (`web/badge.js`), as a 203 dpi image with whole printer dots per QR module, so a badge looks the same whichever station printed it.
- The badge QR encodes the attendee's public profile URL (`BASE_URL/p/<qr_id>`), so a phone camera opens a Bio Connect page instead of a web search.
  The profile shows only what the badge prints (name, designation, institution, category), is marked `noindex`, and returns 404 once the pass is revoked.
  Every ops scanner (check-in, checkout, undo, gates) accepts the profile URL, the bare QR identifier on PDF passes and in the app, or a typed pass number.
- Checkout requires a check-in for the selected day. Both check-in and checkout can be undone with a required reason.
- Spot registration uses the same server-side category fee and verified bank-reference checks as the staff console, then checks in and prints immediately.
  The pass is also sent at once by email and WhatsApp, since the attendee agrees to delivery in person at the desk; a guest pass is only downloaded.
- Access gates can enforce or only log category, day, prior check-in, capacity and single-entry rules. An attendee already recorded inside is always permitted to exit even if gate rules subsequently change.
- Reports show registered, unique attendance, daily attendance and checkout totals by category. CSV exports neutralise spreadsheet formulas.

Before doors open, run one real pass through check-in, print, gate entry, gate exit and checkout on each device and printer.
Keep a USB scanner available because browser camera scanning depends on `BarcodeDetector` support and camera permission.

### Self-service kiosk

`/kiosk` turns an Android tablet into an unattended self check-in point.
An attendee scans the QR on their pass, confirms their name against a preview of the badge, and prints it; printing is what checks them in for the day.

- Sign the tablet in at `/ops` with a station name such as `Kiosk 1`, open **Kiosk**, and choose **Start kiosk on this device**.
  The staff session on that device is replaced by a kiosk session that can only scan, print and check in.
  It cannot read the roster, reports or gates, and returns no contact details.
- Only the pass QR is accepted (the PDF pass, the app, or a printed badge).
  Typed or scanned pass numbers are refused because they are short and guessable; those attendees go to a staffed desk.
- A pass prints once at a kiosk.
  "Badge didn't print? Try again" reprints from the same kiosk for five minutes, at most three prints in all; later reprints are done at a desk.
  A badge printed earlier at a desk counts too.
- On day two, an attendee who already has a badge is checked in without printing.
- The event day comes from the server clock, not the tablet.
- Prints and check-ins appear in the activity audit under the kiosk's station name with the detail "Self-service kiosk", and opening or closing a kiosk is audited as well.
- Hold the Bio Connect logo for two seconds and enter the ops passcode for the staff menu: printer, resolution, label orientation, camera, a test print, and **Exit kiosk mode**.
  The menu closes itself after 90 seconds. Settings are stored on the tablet.

Printing goes through [RawBT](https://play.google.com/store/apps/details?id=ru.a402d.rawbtprinter) so no Android print dialog appears.
The kiosk draws the 76.2 × 50.8 mm badge as a PNG at the printer's resolution, with whole printer dots per QR module, and hands it to RawBT through an Android intent after the server has recorded the print.
Set up each tablet before doors open:

1. Install RawBT, pair the badge printer, choose its driver, set the paper to the 76.2 mm label width, and print RawBT's own test page.
2. In Chrome, open `/kiosk` once and allow the camera. Optionally use **Add to Home screen**: the kiosk opens full screen from there.
3. Keep the tablet on a kiosk launcher that allows RawBT to open, or simply in the full-screen kiosk app.
   Do not use Android screen pinning: it blocks Chrome from opening RawBT, so nothing prints.
4. From the staff menu, run **Print a test badge** and check the margins, orientation (switch to "Rotated 90°" if the printer feeds labels portrait) and that a phone can scan the QR.
5. Run one real pass through scan, print, gate entry and checkout.

The front camera is the default, since it faces the attendee; the preview is mirrored so it behaves like a mirror.
A USB or Bluetooth scanner in keyboard mode also works with no field focused.

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

### Exhibitor pass upgrade (migration 009)

Exhibitor allowances rose from 3 / 2 / 2 to 5 / 3 / 2 passes (premium, standard, table) at no extra fee.
The migration runs on deploy and raises every live premium and standard registration to the new allowance; passes already issued are not touched or re-sent.
Right after that deploy, tell the affected exhibitors they have passes to assign:

```sh
sudo docker compose -f /opt/bioconnect/compose.production.yaml exec app bioconnect roster-notice
```

It emails each exhibitor contact with unassigned passes a single-use link to their registration, valid for 7 days, and prints one line per registration.
Re-running it only reaches registrations that have never had the notice; staff can send a single one again from the console (at most once per 24 hours).

Do not roll back to an image from before 009 without also restoring the pre-deploy backup.
Older code cannot approve a registration with unassigned passes, and would send a queued notice as a generic pass email.

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

- [x] `ops/deploy.sh` and `ops/cloudfront-origin.sh` have run; `https://reg.bioconnect.kerala.gov.in/healthz` returns 200 through CloudFront (not the placeholder JSON). `/delegates` and `/exhibitors` render the disabled-submissions preview.
- [ ] The real SBI Collect link (or merchant setup) is confirmed and set as `SBI_COLLECT_URL`; staff have access to the SBI reconciliation report.
- [x] Zinvos Postmark: verified sender `bioconnect@zinvos.com` / "Bio Connect 4.0", server token set. `POSTMARK_WEBHOOK_USER`/`PASSWORD` configured on the Postmark webhook, pointed at `/api/v1/webhooks/postmark`. Live send to a real inbox delivered and the webhook flipped the job to `delivered`.
- [x] Zinvos Meta: access token, phone number id, app secret, API version set. `META_VERIFY_TOKEN` entered in the Meta webhook config against `/api/v1/webhooks/meta`; the approved pass template with a DOCUMENT header is live (`META_TEMPLATE=bioconnect_pass_delivery_doc`). The worker uploads the pass PDF to the WhatsApp media store and sends it as the header, keeping the pass link in the body as a fallback; the older text-only `bioconnect_pass_delivery` stays approved as a backup. Live template send (PDF + link) to a real handset was accepted by Meta.
- [ ] `ops/secrets/bioconnect-infra.env` (holds `ENCRYPTION_KEY`) backed up to a private vault.
- [ ] Initial staff identities created (`ssh` to the box, `sudo docker compose -f /opt/bioconnect/compose.production.yaml exec app bioconnect staff-create` with `{"Email":...,"Password":...,"Role":"manager"|"reviewer"}` on stdin), each enrolled in an authenticator, at least one `manager` and one `reviewer`.
- [ ] Hourly backup timer green; one restore rehearsal completed; Lightsail automatic snapshots enabled in the console.
- [ ] Published fees confirmed against SBI as the payable amounts.
- [x] The delegate and exhibitor links in the marketing site are deployed (the `data-registration="soon"` panels link to preview the forms; flip to `open` at launch).
- [ ] A single end-to-end live message test (one delegate and one exhibitor) authorised and passed. (Provider-level sends are already proven: branded email delivered to a real inbox with the pass PDF attached, and the WhatsApp template accepted to a real handset with the PDF as its document header. The full create -> approve -> deliver path still needs one run with registration enabled.)

Out of scope: tax invoices, automated refunds, bank-report imports, attendance scanning, automated stall allocation, sponsorship registration.
