#!/usr/bin/env bash
set -eu

PRIMARY_UNIT="vcode-prod.service"
RECOVERY_STANDBY_UNIT="vcode-recovery.service"
RECOVERY_ACTIVE_UNIT="vcode-recovery-active.service"
ACTIVE_PORT="18880"
NGINX_CONF="/www/server/panel/vhost/nginx/v.aimj.xin.conf"

die() { echo "vcode-failover: $*" >&2; exit 1; }

require_root() {
  [ "$(id -u)" -eq 0 ] || die "must run as root"
  [ -f "$NGINX_CONF" ] || die "missing $NGINX_CONF"
  command -v systemctl >/dev/null || die "systemctl is required"
  command -v curl >/dev/null || die "curl is required"
  grep -q "proxy_pass http://127.0.0.1:${ACTIVE_PORT};" "$NGINX_CONF" ||
    die "public Nginx route is not the fixed Vcode port ${ACTIVE_PORT}"
}

wait_http() {
  i=0
  while [ "$i" -lt 30 ]; do
    if curl -fsS --max-time 3 "http://127.0.0.1:${ACTIVE_PORT}/login" >/dev/null 2>&1; then
      return 0
    fi
    i=$((i + 1))
    sleep 1
  done
  return 1
}

stop_clean() {
  unit="$1"
  systemctl stop "$unit" || true
  systemctl reset-failed "$unit" || true
}

status() {
  echo "primary:          $(systemctl is-active "$PRIMARY_UNIT" 2>/dev/null || true)"
  echo "recovery-standby:  $(systemctl is-active "$RECOVERY_STANDBY_UNIT" 2>/dev/null || true)"
  echo "recovery-active:   $(systemctl is-active "$RECOVERY_ACTIVE_UNIT" 2>/dev/null || true)"
  echo "public-upstream:   127.0.0.1:${ACTIVE_PORT} (fixed)"
}

switch_recovery() {
  stop_clean "$PRIMARY_UNIT"
  stop_clean "$RECOVERY_STANDBY_UNIT"
  systemctl start "$RECOVERY_ACTIVE_UNIT"
  if ! wait_http; then
    stop_clean "$RECOVERY_ACTIVE_UNIT"
    systemctl start "$PRIMARY_UNIT" || true
    systemctl start "$RECOVERY_STANDBY_UNIT" || true
    die "recovery runtime did not become healthy; primary was restarted"
  fi
  echo "recovery runtime is now active on ${ACTIVE_PORT}"
}

switch_primary() {
  stop_clean "$RECOVERY_ACTIVE_UNIT"
  systemctl start "$PRIMARY_UNIT"
  if ! wait_http; then
    systemctl start "$RECOVERY_ACTIVE_UNIT" || true
    die "primary runtime did not become healthy; recovery was restarted"
  fi
  systemctl start "$RECOVERY_STANDBY_UNIT"
  echo "primary runtime is now active on ${ACTIVE_PORT}"
}

require_root
case "${1:-status}" in
  status) status ;;
  recovery|standby) switch_recovery ;;
  primary|restore) switch_primary ;;
  *) echo "usage: $0 status|recovery|primary" >&2; exit 2 ;;
esac
