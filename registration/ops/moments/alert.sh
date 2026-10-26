#!/usr/bin/env bash
set -euo pipefail
set -a; source /etc/bioconnect/backup.env; set +a
: "${ALERT_SNS_TOPIC_ARN:?}" "${AWS_ACCESS_KEY_ID:?}" "${AWS_CLI_IMAGE:?}"
# No PII or provider payloads in the alert.
docker run --rm --network host -e AWS_ACCESS_KEY_ID -e AWS_SECRET_ACCESS_KEY "$AWS_CLI_IMAGE" \
  sns publish --region ap-south-1 --topic-arn "$ALERT_SNS_TOPIC_ARN" --subject 'Bio Connect 4.0 operations alert' \
  --message 'An application, delivery queue or backup check failed on the Moments host. Inspect it and staff delivery history.' > /dev/null
