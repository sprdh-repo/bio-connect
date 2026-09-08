#!/usr/bin/env bash
# Repoint CloudFront distribution EEDV34VX46GG7 from the placeholder S3 bucket to
# the Lightsail origin (reg-origin.zinvos.com) over HTTPS, forward everything,
# disable caching, and add the shared-secret origin header.
# Idempotent: re-running just re-applies the same config.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="$HERE/secrets/bioconnect-infra.env"
set -a; source "$ENV_FILE"; set +a

DIST="$CLOUDFRONT_DISTRIBUTION_ID"
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT

aws cloudfront get-distribution-config --id "$DIST" > "$tmp/current.json"
ETAG="$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["ETag"])' "$tmp/current.json")"

python3 - "$tmp/current.json" "$tmp/new.json" <<PY
import json, sys
doc = json.load(open(sys.argv[1]))
c = doc["DistributionConfig"]

origin = {
    "Id": "bioconnect-registration-lightsail",
    "DomainName": "${ORIGIN_HOSTNAME}",
    "OriginPath": "",
    "CustomHeaders": {"Quantity": 1, "Items": [
        {"HeaderName": "X-Origin-Verify", "HeaderValue": "${EDGE_AUTH_SECRET}"}
    ]},
    "CustomOriginConfig": {
        "HTTPPort": 80, "HTTPSPort": 443,
        "OriginProtocolPolicy": "https-only",
        "OriginSslProtocols": {"Quantity": 1, "Items": ["TLSv1.2"]},
        "OriginReadTimeout": 30, "OriginKeepaliveTimeout": 5,
    },
    "ConnectionAttempts": 3, "ConnectionTimeout": 10,
    "OriginShield": {"Enabled": False},
}
c["Origins"] = {"Quantity": 1, "Items": [origin]}
c["DefaultRootObject"] = ""  # the placeholder set this to index.json; the app owns "/"

b = c["DefaultCacheBehavior"]
b["TargetOriginId"] = origin["Id"]
b["ViewerProtocolPolicy"] = "redirect-to-https"
b["CachePolicyId"] = "${CLOUDFRONT_CACHE_POLICY_ID}"
b["OriginRequestPolicyId"] = "${CLOUDFRONT_ORIGIN_REQUEST_POLICY_ID}"
b["AllowedMethods"] = {"Quantity": 7,
    "Items": ["GET","HEAD","OPTIONS","PUT","POST","PATCH","DELETE"],
    "CachedMethods": {"Quantity": 2, "Items": ["GET","HEAD"]}}
b["Compress"] = True
for k in ("ForwardedValues", "MinTTL", "DefaultTTL", "MaxTTL"):
    b.pop(k, None)

json.dump(c, open(sys.argv[2], "w"))
PY

aws cloudfront update-distribution --id "$DIST" --if-match "$ETAG" \
  --distribution-config "file://$tmp/new.json" \
  --query 'Distribution.Status' --output text

echo "update submitted; propagation takes a few minutes."
echo "watch:  aws cloudfront get-distribution --id $DIST --query 'Distribution.Status' --output text"
