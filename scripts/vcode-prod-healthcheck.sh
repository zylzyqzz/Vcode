#!/usr/bin/env bash
set -euo pipefail

test "$(systemctl is-active vcode-prod.service)" = active
nginx -t >/dev/null
test "$(curl --silent --show-error --output /dev/null --write-out '%{http_code}' http://127.0.0.1:18880/status)" = 401
test "$(curl --silent --show-error --output /dev/null --write-out '%{http_code}' https://v.aimj.xin/status)" = 401

use=$(df --output=pcent / | tail -1 | tr -dc '0-9')
if test "$use" -ge 75; then
  logger -t vcode-prod-healthcheck "warning: root filesystem usage is ${use}%"
fi
test "$use" -lt 85

end=$(openssl s_client -connect v.aimj.xin:443 -servername v.aimj.xin </dev/null 2>/dev/null | openssl x509 -noout -enddate | cut -d= -f2)
test "$(date -d "$end" +%s)" -gt "$(date -d '+14 days' +%s)"
