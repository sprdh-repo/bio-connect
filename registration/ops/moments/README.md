# Registration backend on the Moments host

After the event the registration backend moved off the Lightsail instance onto the Moments host (`moments.mshiyaf.com`, 185.62.36.59).
That host is CentOS 7 with DirectAdmin and also serves other sites and mail, so everything here is additive and isolated.
CloudFront `EEDV34VX46GG7` still answers on `reg.bioconnect.kerala.gov.in`; only its origin changed to `bioconnect-origin.zinvos.com` (Route53 A record to 185.62.36.59).
The resources are listed in [`../../docs/infrastructure.md`](../../docs/infrastructure.md).

## Pieces

- `bioconnect-db.service` runs PostgreSQL 17 in Docker with `--network none`; the app reaches it only through the socket in `/srv/bioconnect/pgsock`.
  It uses the Debian `postgres:17` image: on this 3.10 kernel the Alpine image fails `initdb` with `EPERM`.
- `bioconnect.service` runs the static Go binary as the `bioconnect` system user (no shell, no sudo, not in `docker`) on `127.0.0.1:8090`.
- `bioconnect.conf` is the Apache vhost, included at the end of DirectAdmin's `httpd-includes.conf`.
  A request must come from a CloudFront origin-facing range and carry `X-Origin-Verify`; it has no access log because paths carry pass tokens.
  `cloudfront-ips.sh` refreshes the ranges daily and reloads Apache only after `httpd -t` passes.
  The certificate is a certbot webroot certificate, renewed by the host's snap certbot timer.
- `backup.sh` dumps hourly to the same `postgres/` prefix and KMS key as before, through a pinned `amazon/aws-cli` container.
  The IAM user `bioconnect-registration-backup-moments` can only write there (`backup-iam-policy.json`); it cannot read dumps back.
- `monitor.sh` and `alert.sh` are the five-minute health check and its SNS alert.
  Nothing is delivered after the event, so it no longer watches the delivery queue.

Secrets live in `registration/ops/secrets/moments-host.env` and `bioconnect-infra.env`, never in git.

## Operations

```sh
registration/ops/moments/deploy.sh            # build, ship, render env, restart; safe to re-run
registration/ops/moments/deploy.sh --apache   # also the vhost and certificate
registration/ops/moments/restore.sh postgres/<stamp>.dump   # DESTRUCTIVE: replaces the host database
```

`deploy.sh --apache` restores the previous Apache config by itself if `httpd -t` fails or the default vhost of any IP changes.
The Lightsail instance is deleted, so there is no host to roll back to.
The cut-over dump stays in the backups bucket at `final/2026-10-26T14-23-12Z.dump` (no expiry); `restore.sh final/2026-10-26T14-23-12Z.dump` rebuilds from it.
