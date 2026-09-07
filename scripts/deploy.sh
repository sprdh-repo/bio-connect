#!/usr/bin/env bash

set -Eeuo pipefail

BUCKET="${BIOCONNECT_S3_BUCKET:-bioconnect4-zinvos-com-179341475851}"
DISTRIBUTION_ID="${BIOCONNECT_CLOUDFRONT_DISTRIBUTION_ID:-E1PR94ND6CBQQW}"
SITE_URL="${BIOCONNECT_SITE_URL:-https://bioconnect.kerala.gov.in}"
SITE_URL="${SITE_URL%/}"

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

for command in aws curl; do
  if ! command -v "$command" >/dev/null 2>&1; then
    echo "Error: $command is required." >&2
    exit 1
  fi
done

for file in index.html committee.html 404.html styles.css script.js robots.txt sitemap.xml favicon.ico; do
  if [[ ! -f "$file" ]]; then
    echo "Error: required site file '$file' is missing." >&2
    exit 1
  fi
done

if [[ ! -d assets ]]; then
  echo "Error: assets directory is missing." >&2
  exit 1
fi

echo "Checking AWS credentials..."
aws sts get-caller-identity --query 'Account' --output text >/dev/null

echo "Deploying site to s3://$BUCKET..."
aws s3 cp index.html "s3://$BUCKET/index.html" \
  --content-type "text/html; charset=utf-8" \
  --cache-control "no-cache, no-store, must-revalidate" \
  --only-show-errors

aws s3 cp committee.html "s3://$BUCKET/committee.html" \
  --content-type "text/html; charset=utf-8" \
  --cache-control "no-cache, no-store, must-revalidate" \
  --only-show-errors

aws s3 cp 404.html "s3://$BUCKET/404.html" \
  --content-type "text/html; charset=utf-8" \
  --cache-control "no-cache, no-store, must-revalidate" \
  --only-show-errors

# Crawler-facing files: short cache so a sitemap edit reaches Google the same day.
aws s3 cp robots.txt "s3://$BUCKET/robots.txt" \
  --content-type "text/plain; charset=utf-8" \
  --cache-control "public, max-age=300" \
  --only-show-errors

aws s3 cp sitemap.xml "s3://$BUCKET/sitemap.xml" \
  --content-type "application/xml; charset=utf-8" \
  --cache-control "public, max-age=300" \
  --only-show-errors

# Browsers and Google's favicon crawler both probe the site root first.
aws s3 cp favicon.ico "s3://$BUCKET/favicon.ico" \
  --content-type "image/x-icon" \
  --cache-control "public, max-age=86400" \
  --only-show-errors

aws s3 cp styles.css "s3://$BUCKET/styles.css" \
  --content-type "text/css; charset=utf-8" \
  --cache-control "public, max-age=300" \
  --only-show-errors

aws s3 cp script.js "s3://$BUCKET/script.js" \
  --content-type "application/javascript; charset=utf-8" \
  --cache-control "public, max-age=300" \
  --only-show-errors

aws s3 sync assets "s3://$BUCKET/assets/" \
  --cache-control "public, max-age=86400" \
  --only-show-errors

echo "Invalidating CloudFront cache..."
INVALIDATION_ID="$(
  aws cloudfront create-invalidation \
    --distribution-id "$DISTRIBUTION_ID" \
    --paths "/" "/index.html" "/committee.html" "/404.html" "/robots.txt" "/sitemap.xml" "/favicon.ico" \
            "/styles.css" "/script.js" "/assets/*" \
    --query 'Invalidation.Id' \
    --output text
)"

echo "Waiting for invalidation $INVALIDATION_ID..."
aws cloudfront wait invalidation-completed \
  --distribution-id "$DISTRIBUTION_ID" \
  --id "$INVALIDATION_ID"

echo "Verifying $SITE_URL..."
REMOTE_INDEX="$(mktemp)"
trap 'rm -f "$REMOTE_INDEX"' EXIT
curl --fail --silent --show-error --location \
  --retry 3 --retry-delay 2 \
  "$SITE_URL/" > "$REMOTE_INDEX"

if ! cmp -s index.html "$REMOTE_INDEX"; then
  echo "Error: production HTML does not match the deployed index.html." >&2
  exit 1
fi

REMOTE_COMMITTEE="$(mktemp)"
trap 'rm -f "$REMOTE_INDEX" "$REMOTE_COMMITTEE"' EXIT
curl --fail --silent --show-error --location \
  --retry 3 --retry-delay 2 \
  "$SITE_URL/committee.html" > "$REMOTE_COMMITTEE"

if ! cmp -s committee.html "$REMOTE_COMMITTEE"; then
  echo "Error: production HTML does not match the deployed committee.html." >&2
  exit 1
fi

echo "Deployment complete: $SITE_URL"
