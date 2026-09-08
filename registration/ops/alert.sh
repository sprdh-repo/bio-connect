#!/usr/bin/env bash
set -euo pipefail
set -a; source /etc/bioconnect/backup.env; set +a
: "${ALERT_SNS_TOPIC_ARN:?}" "${AWS_ACCESS_KEY_ID:?}"
aws sns publish --region ap-south-1 --topic-arn "$ALERT_SNS_TOPIC_ARN" --subject 'Bio Connect 4.0 operations alert' --message 'An application, delivery queue or backup check failed. Inspect the dedicated host and staff delivery history.' > /dev/null
