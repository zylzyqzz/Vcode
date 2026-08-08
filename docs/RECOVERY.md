# Vcode server recovery runtime

The server deployment keeps a second Vcode runtime ready for recovery without
running two agents against the same workspace at the same time.

## Layout

```text
primary runtime       /opt/vcode              127.0.0.1:18878
recovery standby      /opt/vcode-recovery     127.0.0.1:18879 (normally stopped)
recovery active       /opt/vcode-recovery     127.0.0.1:18878 (during failover)
recovery state        /etc/vcode-recovery    (separate sessions/config)
public reverse proxy  v.aimj.xin              always points to 18878
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

`recovery` stops the primary, starts the isolated recovery state on the fixed
public port, and checks `/login` before declaring success. If the health check
fails, it starts the primary again. `primary` performs the inverse operation.

The standby unit is intentionally disabled and stopped by default:

```sh
systemctl start vcode-recovery.service
systemctl stop vcode-recovery.service
```

That unit is useful for a local health check on port `18879`; it must be
stopped before active failover because both recovery modes share the isolated
recovery state directory.

## Upgrade rule

When the primary binary is upgraded, copy the same verified binary to
`/opt/vcode-recovery/bin/vcode` and check both units before changing the public
runtime. Keep the previous primary binary and both configuration backups until
the new primary has passed its HTTP and real-task smoke checks.
