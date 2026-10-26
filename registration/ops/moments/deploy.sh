#!/usr/bin/env bash
# Build the registration backend and run it on the Moments host
# (moments.mshiyaf.com, CentOS 7 + DirectAdmin, shared with other sites and mail).
# Safe to re-run.
#
#   registration/ops/moments/deploy.sh            # binary, env, units, timers
#   registration/ops/moments/deploy.sh --apache   # also the origin vhost + certificate
#
# Layout on the host:
#   /srv/bioconnect          root:bioconnect 0750  the app's home; the user cannot write here
#   /srv/bioconnect/bin      root:bioconnect 0750  the binary
#   /srv/bioconnect/files    bioconnect      0700  STORAGE_DIR (unused while S3_BUCKET is set)
#   /srv/bioconnect/pgdata   999 (postgres)  0700  PostgreSQL 17 data, container bioconnect-db
#   /srv/bioconnect/pgsock   999:bioconnect  0750  the only way in to PostgreSQL
#   /opt/bioconnect-ops      root            0755  backup / monitor / alert scripts run by root
#   /etc/bioconnect          root            0700  env files
#
# Nothing here touches CSF, mail, DNS on the host, or other Apache vhosts. The
# Apache step only appends one Include to httpd-includes.conf and refuses to
# reload unless `httpd -t` passes and the default vhost of every IP is unchanged.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
SECRETS="$HERE/../secrets"
set -a; source "$SECRETS/bioconnect-infra.env"; set +a

HOST_ENV="$SECRETS/moments-host.env"
if [ ! -f "$HOST_ENV" ]; then
  umask 077
  cat > "$HOST_ENV" <<EOF
# Moments host (registration/ops/moments/deploy.sh). Never committed.
MOMENTS_HOST=moments@moments.mshiyaf.com
MOMENTS_ORIGIN_IP=185.62.36.59
MOMENTS_ORIGIN_HOSTNAME=bioconnect-origin.zinvos.com
MOMENTS_POSTGRES_PASSWORD=$(openssl rand -hex 32)
MOMENTS_BACKUP_AWS_ACCESS_KEY_ID=PENDING
MOMENTS_BACKUP_AWS_SECRET_ACCESS_KEY=PENDING
AWS_CLI_IMAGE=amazon/aws-cli:2.22.0
POSTGRES_IMAGE=postgres:17
EOF
  echo "created $HOST_ENV"
fi
set -a; source "$HOST_ENV"; set +a

SSH="ssh -o ConnectTimeout=20 -o ServerAliveInterval=15 -o ServerAliveCountMax=3 $MOMENTS_HOST"
backup_ready=false
[ "$MOMENTS_BACKUP_AWS_ACCESS_KEY_ID" != PENDING ] && backup_ready=true

tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT

echo "== build (linux/amd64, static) =="
(cd "$ROOT" && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o "$tmp/bioconnect" ./cmd/bioconnect)

echo "== host: user, directories, packages, images =="
$SSH "sudo bash -s" <<EOF
set -euo pipefail
getent group bioconnect >/dev/null || groupadd --system bioconnect
getent passwd bioconnect >/dev/null || useradd --system --gid bioconnect --home-dir /srv/bioconnect --no-create-home --shell /sbin/nologin bioconnect
install -d -m 750 -o root -g bioconnect /srv/bioconnect /srv/bioconnect/bin
install -d -m 700 -o bioconnect -g bioconnect /srv/bioconnect/files
install -d -m 700 -o 999 -g 999 /srv/bioconnect/pgdata
install -d -m 750 -o 999 -g bioconnect /srv/bioconnect/pgsock
install -d -m 755 -o root -g root /opt/bioconnect-ops /var/www/bioconnect-acme
install -d -m 700 -o root -g root /etc/bioconnect /var/lib/bioconnect-backup
rpm -q poppler-utils >/dev/null || yum -y -q install poppler-utils
docker image inspect $POSTGRES_IMAGE >/dev/null 2>&1 || docker pull -q $POSTGRES_IMAGE
docker image inspect $AWS_CLI_IMAGE >/dev/null 2>&1 || docker pull -q $AWS_CLI_IMAGE
EOF

echo "== render + push /etc/bioconnect/*.env =="
database_url="postgres://bioconnect:${MOMENTS_POSTGRES_PASSWORD}@/bioconnect?host=/srv/bioconnect/pgsock"
cat > "$tmp/app.env" <<EOF
DATABASE_URL=${database_url}
ENCRYPTION_KEY=${ENCRYPTION_KEY}
BASE_URL=${BASE_URL}
REGISTRATION_ENABLED=${REGISTRATION_ENABLED}
REGISTRATION_CLOSED=${REGISTRATION_CLOSED:-false}
LIVE_DELIVERY=${LIVE_DELIVERY}
AWS_REGION=${AWS_REGION}
S3_BUCKET=${S3_BUCKET}
AWS_ACCESS_KEY_ID=${APP_AWS_ACCESS_KEY_ID}
AWS_SECRET_ACCESS_KEY=${APP_AWS_SECRET_ACCESS_KEY}
SBI_COLLECT_URL=${SBI_COLLECT_URL}
OPS_KEY=${OPS_KEY:-}
STORE_REVIEW_CODE=${STORE_REVIEW_CODE:-}
POSTMARK_SERVER_TOKEN=${POSTMARK_SERVER_TOKEN}
POSTMARK_FROM_ADDRESS=${POSTMARK_FROM_ADDRESS}
POSTMARK_FROM_NAME=${POSTMARK_FROM_NAME}
POSTMARK_STREAM=${POSTMARK_STREAM}
POSTMARK_WEBHOOK_USER=${POSTMARK_WEBHOOK_USER}
POSTMARK_WEBHOOK_PASSWORD=${POSTMARK_WEBHOOK_PASSWORD}
META_ACCESS_TOKEN=${META_ACCESS_TOKEN}
META_PHONE_NUMBER_ID=${META_PHONE_NUMBER_ID}
META_APP_SECRET=${META_APP_SECRET}
META_VERIFY_TOKEN=${META_VERIFY_TOKEN}
META_API_VERSION=${META_API_VERSION}
META_TEMPLATE=${META_TEMPLATE}
META_TEMPLATE_LANGUAGE=${META_TEMPLATE_LANGUAGE}
MOMENTS_API_BASE=${MOMENTS_API_BASE}
MOMENTS_API_TOKEN=${MOMENTS_API_TOKEN}
EOF
cat > "$tmp/postgres.env" <<EOF
POSTGRES_USER=bioconnect
POSTGRES_PASSWORD=${MOMENTS_POSTGRES_PASSWORD}
POSTGRES_DB=bioconnect
POSTGRES_INITDB_ARGS=--auth-local=scram-sha-256 --auth-host=scram-sha-256
EOF
cat > "$tmp/backup.env" <<EOF
BACKUP_BUCKET=${BACKUP_BUCKET}
BACKUP_KMS_KEY_ID=${BACKUP_KMS_KEY_ID}
ALERT_SNS_TOPIC_ARN=${ALERT_SNS_TOPIC_ARN}
AWS_DEFAULT_REGION=${AWS_REGION}
AWS_ACCESS_KEY_ID=${MOMENTS_BACKUP_AWS_ACCESS_KEY_ID}
AWS_SECRET_ACCESS_KEY=${MOMENTS_BACKUP_AWS_SECRET_ACCESS_KEY}
AWS_CLI_IMAGE=${AWS_CLI_IMAGE}
POSTGRES_PASSWORD=${MOMENTS_POSTGRES_PASSWORD}
EOF
for f in app postgres backup; do
  $SSH "sudo install -m 600 -o root -g root /dev/stdin /etc/bioconnect/${f}.env" < "$tmp/${f}.env"
done

echo "== ship binary, scripts and units =="
$SSH "sudo install -m 750 -o root -g bioconnect /dev/stdin /srv/bioconnect/bin/bioconnect.new" < "$tmp/bioconnect"
for f in backup.sh monitor.sh alert.sh cloudfront-ips.sh; do
  $SSH "sudo install -m 755 -o root -g root /dev/stdin /opt/bioconnect-ops/$f" < "$HERE/$f"
done
for f in "$HERE"/bioconnect*.service "$HERE"/bioconnect*.timer; do
  $SSH "sudo install -m 644 -o root -g root /dev/stdin /etc/systemd/system/$(basename "$f")" < "$f"
done

echo "== start =="
$SSH "sudo bash -s" <<EOF
set -euo pipefail
systemctl daemon-reload
systemctl enable --now bioconnect-db.service
for i in \$(seq 1 30); do
  docker exec bioconnect-db pg_isready -q -U bioconnect -d bioconnect && break
  [ "\$i" = 30 ] && { echo "PostgreSQL never became ready" >&2; exit 1; }
  sleep 2
done
mv -f /srv/bioconnect/bin/bioconnect.new /srv/bioconnect/bin/bioconnect
systemctl enable bioconnect.service
systemctl restart bioconnect.service
for i in \$(seq 1 30); do
  curl -fsS -o /dev/null --max-time 5 http://127.0.0.1:8090/healthz && { echo "app healthz OK"; break; }
  [ "\$i" = 30 ] && { echo "app never became healthy" >&2; journalctl -u bioconnect --no-pager -n 30 >&2; exit 1; }
  sleep 2
done
systemctl enable --now bioconnect-monitor.timer
if $backup_ready; then
  systemctl enable --now bioconnect-backup.timer
else
  echo "!! backup IAM keys are PENDING in moments-host.env: bioconnect-backup.timer not enabled" >&2
fi
EOF

[ "${1:-}" = --apache ] || exit 0

echo "== Apache origin vhost for ${MOMENTS_ORIGIN_HOSTNAME} =="
resolved="$(dig +short "$MOMENTS_ORIGIN_HOSTNAME" | tail -1)"
[ "$resolved" = "$MOMENTS_ORIGIN_IP" ] || { echo "!! $MOMENTS_ORIGIN_HOSTNAME resolves to '${resolved}', not $MOMENTS_ORIGIN_IP" >&2; exit 1; }
render() { # $1 = with-tls | http-only
  sed -e "s|@ORIGIN_HOSTNAME@|${MOMENTS_ORIGIN_HOSTNAME}|g" -e "s|@EDGE_AUTH_SECRET@|${EDGE_AUTH_SECRET}|g" "$HERE/bioconnect.conf" |
    if [ "$1" = with-tls ]; then sed -e '/@TLS_BEGIN@/d' -e '/@TLS_END@/d'; else sed -e '/@TLS_BEGIN@/,/@TLS_END@/d'; fi
}
# Install a vhost file, check the config and the default vhosts, then reload
# gracefully; put everything back if any check fails.
apply_vhost() {
  $SSH "sudo bash -s" <<EOF
set -euo pipefail
conf=/etc/httpd/conf/extra/bioconnect.conf
includes=/etc/httpd/conf/extra/httpd-includes.conf
stamp=\$(date +%s)
defaults() { /usr/sbin/httpd -S 2>/dev/null | grep -A1 'is a NameVirtualHost' | grep 'default server' || true; }
before=\$(defaults)
[ -f \$conf ] && cp -p \$conf /root/bioconnect.conf.\$stamp
cp -p \$includes /root/httpd-includes.conf.\$stamp
install -m 600 -o root -g root /tmp/bioconnect.conf.new \$conf && rm -f /tmp/bioconnect.conf.new
grep -qx 'Include /etc/httpd/conf/extra/bioconnect.conf' \$includes || printf '\n# Bio Connect registration origin (registration/ops/moments/deploy.sh)\nInclude /etc/httpd/conf/extra/bioconnect.conf\n' >> \$includes
[ -f /etc/httpd/conf/extra/bioconnect-cloudfront-ips.conf ] || printf '<RequireAny>\n    Require all denied\n</RequireAny>\n' > /etc/httpd/conf/extra/bioconnect-cloudfront-ips.conf
restore() {
  echo "!! \$1; restoring the previous Apache config" >&2
  cp -p /root/httpd-includes.conf.\$stamp \$includes
  if [ -f /root/bioconnect.conf.\$stamp ]; then cp -p /root/bioconnect.conf.\$stamp \$conf; else rm -f \$conf; fi
  /usr/sbin/httpd -t && exit 1
  echo "!! restored config still fails httpd -t; investigate before touching Apache" >&2; exit 1
}
/usr/sbin/httpd -t || restore "httpd -t failed"
[ "\$(defaults)" = "\$before" ] || restore "a default vhost changed"
/usr/sbin/httpd -k graceful
echo "Apache reloaded gracefully"
EOF
}
if ! $SSH "sudo test -f /etc/letsencrypt/live/${MOMENTS_ORIGIN_HOSTNAME}/fullchain.pem"; then
  render http-only | $SSH "sudo install -m 600 -o root -g root /dev/stdin /tmp/bioconnect.conf.new"
  apply_vhost
  $SSH "sudo /opt/bioconnect-ops/cloudfront-ips.sh"
  $SSH "sudo /var/lib/snapd/snap/bin/certbot certonly --webroot -w /var/www/bioconnect-acme -d ${MOMENTS_ORIGIN_HOSTNAME} --non-interactive --agree-tos -m ${ACME_EMAIL} --deploy-hook '/usr/sbin/httpd -k graceful'"
fi
render with-tls | $SSH "sudo install -m 600 -o root -g root /dev/stdin /tmp/bioconnect.conf.new"
apply_vhost
$SSH "sudo /opt/bioconnect-ops/cloudfront-ips.sh && sudo systemctl enable --now bioconnect-cloudfront-ips.timer"

echo "== origin checks =="
code_without="$(curl -s -o /dev/null -w '%{http_code}' "https://${MOMENTS_ORIGIN_HOSTNAME}/healthz" || true)"
echo "direct request without CloudFront: ${code_without} (expect 403)"
