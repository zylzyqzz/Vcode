#!/usr/bin/env bash
set -a
. /etc/vcode/vcode.env
set +a
exec /opt/vcode/bin/vcode serve --addr 127.0.0.1:18878 --auth password --behind-proxy
