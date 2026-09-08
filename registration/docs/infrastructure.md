# Infrastructure

AWS account `179341475851`, region `ap-south-1` (Mumbai). No secrets in this file; secret values live in `registration/ops/secrets/bioconnect-infra.env` (git-ignored).

## Topology

```
viewer ─HTTPS→ CloudFront  EEDV34VX46GG7   (reg.bioconnect.kerala.gov.in)
                 │  ACM cert already covers the name
                 │  cache: Managed-CachingDisabled
                 │  origin request: Managed-AllViewerExceptHostHeader (forwards headers, cookies, query)
                 │  origin custom header  X-Origin-Verify: <EDGE_AUTH_SECRET>
                 ▼  HTTPS, https-only
        reg-origin.zinvos.com   (Route53 zone Z04419011U2NLCZ532RX5, A → 3.6.222.177)
                 ▼
        Lightsail  bioconnect-registration  (small_3_1, Ubuntu 24.04, ap-south-1a, 2 GB swap, AWS CLI v2)
        static IP  bioconnect-registration-ip  = 3.6.222.177
        firewall   22/tcp ← 0.0.0.0/0 (key-only auth; TIGHTEN to your IP) · 80,443/tcp ← anywhere
        daily auto-snapshot at 19:00 UTC
          docker compose  (/opt/bioconnect/compose.production.yaml):
            caddy   :80/:443  LE cert for reg-origin.zinvos.com; 403s requests without X-Origin-Verify
            app     :8080     Go server + PostgreSQL-backed delivery worker
            db      postgres:17-alpine, private "backend" network, volume "db"
          uploads / passes  → s3://bioconnect4-registration-uploads-179341475851  (private, SSE-S3, versioned)
          hourly pg_dump    → s3://bioconnect4-registration-backups-179341475851  (private, SSE-KMS, 30-day expiry)
```

Why CloudFront stays in front: the `reg.` DNS record points at CloudFront today and changing records on `bioconnect.kerala.gov.in` needs a multi-step external approval. This design needs no `kerala.gov.in` DNS change. If that record can later become an `A` record to `3.6.222.177`, drop CloudFront and let Caddy serve `reg.bioconnect.kerala.gov.in` directly.

## Resources

| Kind | Name / ID |
|---|---|
| Lightsail instance | `bioconnect-registration` (small_3_1, ubuntu_24_04, ap-south-1a) |
| Lightsail static IP | `bioconnect-registration-ip` = `3.6.222.177` |
| Lightsail key pair | `bioconnect-registration-key` (private key in `ops/secrets/bioconnect-registration.pem`) |
| S3 uploads | `bioconnect4-registration-uploads-179341475851` — SSE-S3, versioned, block-public, 30-day noncurrent expiry |
| S3 backups | `bioconnect4-registration-backups-179341475851` — SSE-KMS, versioned, block-public, TLS-only + correct-key bucket policy, 30-day `postgres/` expiry |
| KMS key | `alias/bioconnect-registration-backups` (rotation enabled) |
| SNS topic | `bioconnect-registration-alerts` |
| CloudFront | `EEDV34VX46GG7` (alias `reg.bioconnect.kerala.gov.in`, cert `arn:aws:acm:us-east-1:179341475851:certificate/4070ad91-...`) |
| Route53 record | `reg-origin.zinvos.com` A → `3.6.222.177` (zone `Z04419011U2NLCZ532RX5`) |
| IAM user (app) | `bioconnect-registration-app` — `s3:PutObject/GetObject` on the uploads bucket only |
| IAM user (backup) | `bioconnect-registration-backup` — put/get on `backups/postgres/*`, `kms:*DataKey/Encrypt/Decrypt` on the key, `sns:Publish` on the topic |

## Status

Deployed 2026-09-07. `https://reg.bioconnect.kerala.gov.in` serves the app through CloudFront; `registration_enabled=false`, `live_delivery=false`. The hourly backup and 5-minute monitor timers are enabled and a test backup is in S3. The placeholder S3 bucket is no longer referenced by CloudFront (its `DefaultRootObject` was cleared).

## Provision and deploy

Three scripts under `registration/ops/`, run from a workstation with AWS credentials for the account and the files in `ops/secrets/`:

```sh
cd registration

# 1. IAM users + keys, the Route53 origin record, SNS email subscription.
#    Writes the IAM keys back into ops/secrets/bioconnect-infra.env.
#    Set ALERT_EMAIL / ACME_EMAIL in that file first.
ops/provision-finish.sh

# 2. Build the image, ship it, render /etc/bioconnect/*.env, compose up, health-check.
#    Re-run for every code change.
ops/deploy.sh

# 3. One-time: point CloudFront at the Lightsail origin (replaces the placeholder bucket).
ops/cloudfront-origin.sh
```

`deploy.sh` refuses to run while the Zinvos values are blank *and* `LIVE_DELIVERY=true`. Keep `REGISTRATION_ENABLED=false` and `LIVE_DELIVERY=false` in the env file until launch; flip them and re-run `deploy.sh`.

The KMS key, SNS topic, both S3 buckets, the Lightsail instance + static IP + key pair, and the firewall are already created. `provision-finish.sh` covers the four resources that could not be created non-interactively (2 IAM users, the DNS record, the SNS subscription).

## SSH

```sh
ssh -i registration/ops/secrets/bioconnect-registration.pem ubuntu@3.6.222.177
```

SSH is currently open to `0.0.0.0/0` (key-only, password auth disabled). **Restrict it to your workstation/VPN IP:**

```sh
aws lightsail put-instance-public-ports --region ap-south-1 --instance-name bioconnect-registration \
  --port-infos fromPort=22,toPort=22,protocol=TCP,cidrs=<yourip>/32 \
              fromPort=80,toPort=80,protocol=TCP,cidrs=0.0.0.0/0 \
              fromPort=443,toPort=443,protocol=TCP,cidrs=0.0.0.0/0
```

Create the first staff accounts:

```sh
ssh -i registration/ops/secrets/bioconnect-registration.pem ubuntu@3.6.222.177
echo '{"Email":"you@example.com","Password":"a-long-passphrase","Role":"manager"}' \
  | sudo docker compose -f /opt/bioconnect/compose.production.yaml exec -T app bioconnect staff-create
# prints an otpauth:// URI - add it to an authenticator app
```

## Backups, restore, rollback, monitoring

See [`operations.md`](operations.md). The backup/monitor systemd units are installed and enabled by `deploy.sh`. Enable Lightsail automatic snapshots on the instance in the console (one-time).

## Known limitations to revisit

- CloudFront → origin depends on `reg-origin.zinvos.com`; keep that Route53 record as long as CloudFront is in front.
- The `X-Origin-Verify` shared header is the only thing stopping direct traffic to `reg-origin.zinvos.com`. Rotate it by editing `ops/secrets/bioconnect-infra.env` and re-running `deploy.sh` then `cloudfront-origin.sh`. Optionally also restrict the Lightsail firewall 443 to the CloudFront origin-facing prefix.
- AWS root access keys are in use for provisioning. Create an admin IAM user and disable the root keys after launch.
