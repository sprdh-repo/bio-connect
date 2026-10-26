#!/usr/bin/env bash
set -euo pipefail
# After the event BioConnect only has to stay up: no deliveries are sent, so the
# old delivery-queue check is gone and this alerts only when the app is down.
# The local listener, not the public hostname: this checks this host, whichever
# origin CloudFront currently uses.
curl --fail --silent --show-error --max-time 15 http://127.0.0.1:8090/healthz > /dev/null
