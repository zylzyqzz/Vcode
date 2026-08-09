#!/usr/bin/env bash
# Install as /opt/vcode-prod/bin/vcode-start.sh for vcode-prod.service.
set -euo pipefail
exec /opt/vcode-prod/current serve --addr 127.0.0.1:18880 --auth password --behind-proxy
