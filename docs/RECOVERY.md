# Vcode server recovery runtime

The server deployment keeps a second Vcode runtime ready for recovery without
running two agents against the same workspace at the same time.

## Layout

```text
primary runtime       /opt/vcode-prod         127.0.0.1:18880
recovery standby      /opt/vcode-recovery     127.0.0.1:18879 (always-on, silent)
recovery active       /opt/vcode-recovery     127.0.0.1:18880 (during failover)
recovery state        /etc/vcode-recovery    (separate sessions/config)
public reverse proxy  v.aimj.xin              / -> 18880, /1/ -> 18879
```

The recovery configuration defaults to the official DeepSeek provider. Its
credential is stored only in `/etc/vcode-recovery/.env` with mode `0600`; it is
not committed, returned to clients, or written into task transcripts.

Both runtimes run as `root` because this private server deployment is intended
to repair the server itself. This is deliberately high-risk: do not expose
the local ports and do not enable both active services simultaneously.

## Operations

Run on the server as `root`:

```sh
/usr/local/sbin/vcode-failover status
/usr/local/sbin/vcode-failover recovery
/usr/local/sbin/vcode-failover primary
```

`recovery` stops the primary, temporarily moves the isolated recovery state to
the fixed public port, and checks `/login` before declaring success. If the
health check fails, it starts the primary again. `primary` performs the inverse
operation and brings the silent `/1/` standby back.

The standby unit runs silently on `/1/` so it can repair the primary runtime:

```sh
systemctl status vcode-recovery.service
systemctl stop vcode-recovery.service
```

It must be stopped before active failover because both recovery modes share the
isolated recovery state directory. The `/1/` web client is path-aware and keeps
its API, SSE, manifest, and favicon requests inside the recovery runtime.

## Upgrade rule

When the primary binary is upgraded, copy the same verified binary to
`/opt/vcode-recovery/bin/vcode` and check both units before changing the public
runtime. Keep the previous primary binary and both configuration backups until
the new primary has passed its HTTP and real-task smoke checks.
