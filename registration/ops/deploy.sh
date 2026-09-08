#!/usr/bin/env bash
# Build the registration image, ship it to the Lightsail host, render the
# server-side env files, and bring the stack up. Safe to re-run.
#
#   registration/ops/deploy.sh
#
# Requires: docker, ssh, rsync locally; ops/secrets/bioconnect-infra.env filled
# in (run provision-finish.sh first so the IAM keys are no longer PENDING).
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/.." && pwd)"
SECRETS="$HERE/secrets"
ENV_FILE="$SECRETS/bioconnect-infra.env"
[ -f "$ENV_FILE" ] || { echo "missing $ENV_FILE" >&2; exit 1; }
set -a; source "$ENV_FILE"; set +a

fail=0
for k in APP_AWS_ACCESS_KEY_ID APP_AWS_SECRET_ACCESS_KEY ENCRYPTION_KEY POSTGRES_PASSWORD EDGE_AUTH_SECRET; do
  v="${!k:-}"
  [ -n "$v" ] && [ "$v" != PENDING ] || { echo "!! $k is unset/PENDING - run provision-finish.sh" >&2; fail=1; }
done
[[ "${ACME_EMAIL:-}" == CHANGEME* || -z "${ACME_EMAIL:-}" ]] && { echo "!! set ACME_EMAIL in $ENV_FILE" >&2; fail=1; }
[ "$fail" = 0 ] || exit 1

if [ "${LIVE_DELIVERY}" = "true" ]; then
  for k in POSTMARK_SERVER_TOKEN POSTMARK_FROM_ADDRESS POSTMARK_FROM_NAME META_ACCESS_TOKEN \
           META_PHONE_NUMBER_ID META_APP_SECRET META_API_VERSION META_TEMPLATE SBI_COLLECT_URL; do
    [ -n "${!k:-}" ] || { echo "!! LIVE_DELIVERY=true but $k is empty" >&2; exit 1; }
  done
fi

SSH_KEY="$SECRETS/bioconnect-registration.pem"
SSH="ssh -i $SSH_KEY -o StrictHostKeyChecking=accept-new -o ConnectTimeout=20 ${SSH_USER}@${SERVER_IP}"
TAG="$(cd "$ROOT" && git rev-parse --short HEAD 2>/dev/null || date +%s)"
IMAGE="bioconnect-registration:${TAG}"

echo "== wait for cloud-init on ${SERVER_IP} =="
for i in $(seq 1 40); do
  if $SSH 'test -f /opt/bioconnect/.provisioned' 2>/dev/null; then echo "host ready"; break; fi
  [ "$i" = 40 ] && { echo "host never finished cloud-init" >&2; exit 1; }
  sleep 15
done

echo "== build $IMAGE (linux/amd64, single manifest) =="
docker build --platform linux/amd64 --provenance=false --sbom=false -t "$IMAGE" "$ROOT"

echo "== ship image (~$(docker image inspect "$IMAGE" --format '{{.Size}}' | numfmt --to=iec)) =="
docker save "$IMAGE" | gzip | $SSH 'gunzip | docker load'

echo "== render + push /etc/bioconnect/*.env =="
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
cat > "$tmp/app.env" <<EOF
DATABASE_URL=${DATABASE_URL}
ENCRYPTION_KEY=${ENCRYPTION_KEY}
BASE_URL=${BASE_URL}
APP_ENV=production
REGISTRATION_ENABLED=${REGISTRATION_ENABLED}
LIVE_DELIVERY=${LIVE_DELIVERY}
AWS_REGION=${AWS_REGION}
S3_BUCKET=${S3_BUCKET}
AWS_ACCESS_KEY_ID=${APP_AWS_ACCESS_KEY_ID}
AWS_SECRET_ACCESS_KEY=${APP_AWS_SECRET_ACCESS_KEY}
SBI_COLLECT_URL=${SBI_COLLECT_URL}
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
EOF
cat > "$tmp/postgres.env" <<EOF
POSTGRES_USER=bioconnect
POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
POSTGRES_DB=bioconnect
EOF
cat > "$tmp/caddy.env" <<EOF
ORIGIN_HOSTNAME=${ORIGIN_HOSTNAME}
EDGE_AUTH_SECRET=${EDGE_AUTH_SECRET}
ACME_EMAIL=${ACME_EMAIL}
EOF
cat > "$tmp/backup.env" <<EOF
BACKUP_BUCKET=${BACKUP_BUCKET}
BACKUP_KMS_KEY_ID=${BACKUP_KMS_KEY_ID}
COMPOSE_DIR=${COMPOSE_DIR}
ALERT_SNS_TOPIC_ARN=${ALERT_SNS_TOPIC_ARN}
AWS_DEFAULT_REGION=${AWS_REGION}
AWS_ACCESS_KEY_ID=${BACKUP_AWS_ACCESS_KEY_ID}
AWS_SECRET_ACCESS_KEY=${BACKUP_AWS_SECRET_ACCESS_KEY}
EOF
for f in app postgres caddy backup; do
  $SSH "sudo install -m 600 -o root -g root /dev/stdin /etc/bioconnect/${f}.env" < "$tmp/${f}.env"
done

echo "== sync compose + Caddyfile + ops scripts to ${COMPOSE_DIR} =="
rsync -e "ssh -i $SSH_KEY -o StrictHostKeyChecking=accept-new" -az --delete \
  "$HERE/compose.production.yaml" "$HERE/Caddyfile" "$HERE/backup.sh" "$HERE/monitor.sh" "$HERE/alert.sh" \
  "$HERE"/bioconnect-*.service "$HERE"/bioconnect-*.timer \
  "${SSH_USER}@${SERVER_IP}:/tmp/bioconnect-ops/"
$SSH "sudo mkdir -p ${COMPOSE_DIR}/ops && sudo cp /tmp/bioconnect-ops/compose.production.yaml /tmp/bioconnect-ops/Caddyfile ${COMPOSE_DIR}/ && sudo cp /tmp/bioconnect-ops/*.sh ${COMPOSE_DIR}/ops/ && sudo chmod +x ${COMPOSE_DIR}/ops/*.sh && sudo cp /tmp/bioconnect-ops/bioconnect-*.service /tmp/bioconnect-ops/bioconnect-*.timer /etc/systemd/system/ && rm -rf /tmp/bioconnect-ops"

# docker compose auto-loads COMPOSE_DIR/.env, so backup.sh / monitor.sh can run
# `docker compose ... exec` without needing APP_IMAGE in their own environment.
$SSH "printf 'APP_IMAGE=%s\n' '${IMAGE}' | sudo install -m 640 -o root -g root /dev/stdin ${COMPOSE_DIR}/.env"

echo "== compose up =="
$SSH "cd ${COMPOSE_DIR} && sudo docker compose -f compose.production.yaml up -d --remove-orphans"
$SSH "sudo systemctl daemon-reload && sudo systemctl enable --now bioconnect-backup.timer bioconnect-monitor.timer && systemctl is-active bioconnect-backup.timer bioconnect-monitor.timer"

echo "== wait for Caddy cert + health =="
for i in $(seq 1 20); do
  code="$($SSH "curl -s -o /dev/null -w '%{http_code}' -H 'X-Origin-Verify: ${EDGE_AUTH_SECRET}' https://${ORIGIN_HOSTNAME}/healthz" || true)"
  [ "$code" = 200 ] && { echo "origin healthz OK"; break; }
  echo "  healthz=$code (try $i)"; sleep 15
done

echo "== through CloudFront =="
curl -s -o /dev/null -w '%{url_effective} -> %{http_code}\n' "https://${PUBLIC_HOSTNAME}/healthz" || true
echo "(if this is 403/placeholder, run: registration/ops/cloudfront-origin.sh)"
