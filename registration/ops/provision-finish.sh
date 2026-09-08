#!/usr/bin/env bash
# Finishes provisioning the pieces the assistant could not create under sandbox:
# two scoped IAM users + keys, the Route53 origin record, the SNS email subscription.
# Idempotent. Run from a shell with AWS credentials for account 179341475851.
#
#   registration/ops/provision-finish.sh
#
# Reads and updates ops/secrets/bioconnect-infra.env in place.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="$HERE/secrets/bioconnect-infra.env"
[ -f "$ENV_FILE" ] || { echo "missing $ENV_FILE" >&2; exit 1; }
set -a; source "$ENV_FILE"; set +a

U="$S3_BUCKET"
B="$BACKUP_BUCKET"
KEY_ARN="$BACKUP_KMS_KEY_ID"
TOPIC_ARN="$ALERT_SNS_TOPIC_ARN"

set_kv() { # set_kv KEY VALUE  -> updates the env file in place
  local k="$1" v="$2"
  if grep -q "^${k}=" "$ENV_FILE"; then
    python3 - "$ENV_FILE" "$k" "$v" <<'PY'
import sys
path, key, val = sys.argv[1], sys.argv[2], sys.argv[3]
lines = open(path).read().splitlines()
out = [f"{key}={val}" if l.startswith(key + "=") else l for l in lines]
open(path, "w").write("\n".join(out) + "\n")
PY
  else
    printf '%s=%s\n' "$k" "$v" >> "$ENV_FILE"
  fi
}

make_user() { # make_user NAME POLICY_NAME POLICY_JSON KEYID_VAR SECRET_VAR
  local name="$1" pol="$2" doc="$3" kid_var="$4" sec_var="$5"
  aws iam get-user --user-name "$name" >/dev/null 2>&1 || \
    aws iam create-user --user-name "$name" --tags Key=project,Value=bioconnect-registration >/dev/null
  aws iam put-user-policy --user-name "$name" --policy-name "$pol" --policy-document "$doc"
  # Only mint a key if the env file still says PENDING (avoid key sprawl).
  if [ "${!kid_var:-PENDING}" = "PENDING" ]; then
    local out; out="$(aws iam create-access-key --user-name "$name" --query 'AccessKey.[AccessKeyId,SecretAccessKey]' --output text)"
    set_kv "$kid_var" "$(echo "$out" | cut -f1)"
    set_kv "$sec_var" "$(echo "$out" | cut -f2)"
    echo "created access key for $name"
  else
    echo "$name already has a key recorded; skipping create-access-key"
  fi
}

echo "== IAM: app runtime user =="
make_user bioconnect-registration-app s3-uploads "{
  \"Version\":\"2012-10-17\",
  \"Statement\":[
    {\"Effect\":\"Allow\",\"Action\":[\"s3:PutObject\",\"s3:GetObject\"],\"Resource\":\"arn:aws:s3:::$U/*\"},
    {\"Effect\":\"Allow\",\"Action\":[\"s3:ListBucket\"],\"Resource\":\"arn:aws:s3:::$U\"}
  ]}" APP_AWS_ACCESS_KEY_ID APP_AWS_SECRET_ACCESS_KEY

echo "== IAM: backup user =="
make_user bioconnect-registration-backup backup-and-alert "{
  \"Version\":\"2012-10-17\",
  \"Statement\":[
    {\"Effect\":\"Allow\",\"Action\":[\"s3:PutObject\",\"s3:GetObject\"],\"Resource\":\"arn:aws:s3:::$B/postgres/*\"},
    {\"Effect\":\"Allow\",\"Action\":[\"s3:ListBucket\"],\"Resource\":\"arn:aws:s3:::$B\",\"Condition\":{\"StringLike\":{\"s3:prefix\":\"postgres/*\"}}},
    {\"Effect\":\"Allow\",\"Action\":[\"kms:GenerateDataKey\",\"kms:Encrypt\",\"kms:Decrypt\",\"kms:DescribeKey\"],\"Resource\":\"$KEY_ARN\"},
    {\"Effect\":\"Allow\",\"Action\":[\"sns:Publish\"],\"Resource\":\"$TOPIC_ARN\"}
  ]}" BACKUP_AWS_ACCESS_KEY_ID BACKUP_AWS_SECRET_ACCESS_KEY

echo "== Route53: $ORIGIN_HOSTNAME A -> $SERVER_IP =="
aws route53 change-resource-record-sets --hosted-zone-id "$ROUTE53_ZONE_ID" --change-batch "{
  \"Comment\":\"Bio Connect 4.0 registration origin for CloudFront $CLOUDFRONT_DISTRIBUTION_ID\",
  \"Changes\":[{\"Action\":\"UPSERT\",\"ResourceRecordSet\":{
    \"Name\":\"$ORIGIN_HOSTNAME\",\"Type\":\"A\",\"TTL\":300,
    \"ResourceRecords\":[{\"Value\":\"$SERVER_IP\"}]}}]}" --query 'ChangeInfo.Status' --output text

if [ "${ALERT_EMAIL:-}" != "" ] && [[ "$ALERT_EMAIL" != CHANGEME* ]]; then
  echo "== SNS: subscribe $ALERT_EMAIL (confirm the email AWS sends) =="
  aws sns subscribe --topic-arn "$TOPIC_ARN" --protocol email --notification-endpoint "$ALERT_EMAIL" --query 'SubscriptionArn' --output text
else
  echo "!! set ALERT_EMAIL in $ENV_FILE, then re-run for the SNS subscription"
fi

echo
echo "done. Next: registration/ops/deploy.sh"
