#!/usr/bin/env bash
set -euo pipefail

environment=/etc/vcode-prod/backup.env
test -r "$environment"
# shellcheck disable=SC1090
. "$environment"
: "${RESTIC_REPOSITORY:?RESTIC_REPOSITORY is required}"
: "${RESTIC_PASSWORD_FILE:?RESTIC_PASSWORD_FILE is required}"

export RESTIC_REPOSITORY RESTIC_PASSWORD_FILE
restic backup /etc/vcode-prod /opt/vcode-prod --tag vcode-production
restic forget --keep-daily 14 --keep-weekly 8 --prune
