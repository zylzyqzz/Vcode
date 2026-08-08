#!/usr/bin/env bash
set -euo pipefail

commit=${1:?commit is required}
artifact=${2:?artifact path is required}
checksum=${3:?checksum path is required}
root=/opt/vcode-prod
releases="$root/releases"
current="$root/current"

test "$(id -u)" -eq 0
test "$commit" != "master"
test -f "$artifact"
test -f "$checksum"

expected=$(awk '{print $1}' "$checksum")
actual=$(sha256sum "$artifact" | awk '{print $1}')
test "$expected" = "$actual"

install -d -m 0700 "$releases" "$root/staging"
release="$releases/$commit"
install -m 0755 "$artifact" "$release"
file "$release" | grep -q 'ELF 64-bit.*x86-64'

previous=""
if test -L "$current"; then
  previous=$(readlink -f "$current")
fi

ln -sfn "$release" "$root/current.next"
mv -Tf "$root/current.next" "$current"

if systemctl restart vcode-prod.service && /usr/local/sbin/vcode-prod-healthcheck; then
  find "$releases" -maxdepth 1 -type f -printf '%T@ %p\n' | sort -nr | awk 'NR > 3 {print $2}' | xargs -r rm -f
  exit 0
fi

if test -n "$previous" && test -x "$previous"; then
  ln -sfn "$previous" "$root/current.rollback"
  mv -Tf "$root/current.rollback" "$current"
  systemctl restart vcode-prod.service || true
fi
exit 1
