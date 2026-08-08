#!/usr/bin/env bash
set -a
. /etc/vcode-recovery/vcode.env
set +a
exec /opt/vcode-recovery/bin/vcode serve --addr 127.0.0.1:18879 --auth password --behind-proxy
