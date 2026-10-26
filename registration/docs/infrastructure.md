# Infrastructure

AWS account `179341475851`, region `ap-south-1` (Mumbai). No secrets in this file; secret values live in `registration/ops/secrets/` (git-ignored).

After the event the backend only has to stay up for the mobile app and the site, so on 2026-10-26 it moved off its Lightsail instance onto the Moments host.
The Lightsail instance, its static IP, its snapshots and the `reg-origin.zinvos.com` record are deleted.
Operating details for the current host are in [`ops/moments/README.md`](../ops/moments/README.md).

## Topology

```
viewer ─HTTPS→ CloudFront  EEDV34VX46GG7   (reg.bioconnect.kerala.gov.in; the mobile app hard-codes this name)
                 │  cache: Managed-CachingDisabled
                 │  origin request: Managed-AllViewerExceptHostHeader (forwards headers, cookies, query)
                 │  origin custom header  X-Origin-Verify: <EDGE_AUTH_SECRET>
                 ▼  HTTPS, https-only
        bioconnect-origin.zinvos.com   (Route53 zone Z04419011U2NLCZ532RX5, A → 185.62.36.59)
                 ▼
        Moments host  moments.mshiyaf.com  (CentOS 7, DirectAdmin; shared with other sites and mail)
          Apache vhost   CloudFront origin-facing ranges + X-Origin-Verify only; LE webroot cert; no access log
          bioconnect.service      Go binary as user bioconnect on 127.0.0.1:8090
          bioconnect-db.service   postgres:17 in Docker, --network none, socket in /srv/bioconnect/pgsock
          uploads / passes  → s3://bioconnect4-registration-uploads-179341475851  (private, SSE-S3, versioned)
          hourly pg_dump    → s3://bioconnect4-registration-backups-179341475851/postgres/  (SSE-KMS, 30-day expiry)
```

## Resources

| Kind | Name / ID |
|---|---|
| Host | `moments.mshiyaf.com` (185.62.36.59), SSH as `moments` |
| S3 uploads | `bioconnect4-registration-uploads-179341475851` - SSE-S3, versioned, block-public, 30-day noncurrent expiry |
| S3 backups | `bioconnect4-registration-backups-179341475851` - SSE-KMS, versioned, block-public, TLS-only + correct-key bucket policy, 30-day `postgres/` expiry; `final/` holds the cut-over dumps and never expires |
| KMS key | `alias/bioconnect-registration-backups` (rotation enabled) |
| SNS topic | `bioconnect-registration-alerts` |
| CloudFront | `EEDV34VX46GG7` (alias `reg.bioconnect.kerala.gov.in`, cert `arn:aws:acm:us-east-1:179341475851:certificate/4070ad91-...`) |
| Route53 record | `bioconnect-origin.zinvos.com` A → `185.62.36.59` (zone `Z04419011U2NLCZ532RX5`) |
| IAM user (app) | `bioconnect-registration-app` - `s3:PutObject/GetObject` on the uploads bucket only |
| IAM user (backup) | `bioconnect-registration-backup-moments` - write-only `postgres/*`, the backup key via S3 only, `sns:Publish` on the topic |
| IAM user (retired) | `bioconnect-registration-backup` - the Lightsail backup user; its key is deactivated |

## Deploy

```sh
registration/ops/moments/deploy.sh            # build, ship, render env, restart; safe to re-run
registration/ops/moments/deploy.sh --apache   # also the origin vhost and certificate
```

The other scripts in `registration/ops/` (`deploy.sh`, `provision-finish.sh`, `cloudfront-origin.sh`, `cloud-init.sh`, the compose file and Caddyfile) built the retired Lightsail host and are kept only for reference.

## Known limitations

- CentOS 7 is end of life; the host is accepted as is, with the BioConnect pieces isolated from the rest of it.
- CloudFront → origin depends on `bioconnect-origin.zinvos.com`; keep that record while CloudFront is in front.
- AWS root access keys are still in use for administration. Create an admin IAM user and disable the root keys.
