# Releasing

How Vcode ships, who can ship what, and the canary-before-stable flow.

## Branch model: trunk + tags

- **`master`** is the single development line (the v2 / 1.x trunk). Every PR merges here.
- **Production is a tag, not a branch.** A release is a tagged snapshot of `master`:
  `v1.4.0` (CLI), `npm-v1.4.0` (npm).
- **`v1`** is the archived 1.0/legacy line — maintenance only.
- **Hotfix** an already-released version by branching from its tag, fixing, and tagging again.

There is no separate "production" or "develop" branch by design — the canary channel
provides the pre-release buffer instead of a long-lived branch.

## Channels

| Surface | Stable | Pre-release buffer |
|---|---|---|
| npm | `latest` (0.x), `next` (1.x) | `canary` (`npm i Vcode@canary`) |

A canary build is isolated: it **never** moves `latest` / `next`.
Testers opt in explicitly (npm versions ending in `-canary.N` publish under the
`canary` dist-tag).

## Who can release what

| Action | Who | Mechanism |
|---|---|---|
| **Cut a canary** | any maintainer (write access) | `workflow_dispatch`, runs free (open `canary` environment) |
| **Ship `next` / stable** | **esengine only** | stable publish jobs gate on the `release` environment — esengine must approve before anything goes public |

So a maintainer can dispatch a canary anytime, but a stable release — even one a
maintainer starts by pushing a tag — pauses in the Actions UI until **esengine approves**
the `release` environment deployment.

> Repo settings backing this: Environments → `release` has esengine as a required
> reviewer; `canary` has none. (Optional hardening: a tag ruleset restricting
> `v*`/`npm-v*` creation to esengine, so maintainers can't even start a
> stable release.)

## The release loop

1. **Develop** — PRs land on `master` (branch auto-deletes on merge).
2. **Cut a canary** before the intended release (e.g. heading for `1.4.0`):
   - CLI: Actions → **Release npm** → `base_version: 1.4.0`
   - Publishes `1.4.0-canary.N` to npm `@canary`.
3. **Test** — testers install `Vcode@canary` and report bugs.
4. **Fix** on `master` via PRs; re-cut the canary as needed (`canary.N` bumps).
5. **Ship stable** when the canary is clean — push the two tags:
   ```sh
   git tag v1.4.0     && git push origin v1.4.0      # CLI binaries + Homebrew
   git tag npm-v1.4.0 && git push origin npm-v1.4.0  # npm -> next
   ```
   Each stable run **waits for esengine to approve the `release` environment** before publishing.
6. **Promote to default install** (optional, when 1.x should become the bare `npm i` target):
   ```sh
   npm dist-tag add Vcode@1.4.0 latest
   ```
7. **Next cycle** — the canary rolls on toward `1.5.0`.

## Notes

- Canary version numbers use the workflow `run_number`. Only monotonicity per channel matters.
- A stable `-rc` tag (e.g. `npm-v1.4.0-rc.1`) still ships under `next`, not `canary`.
- Windows and Linux apply downloaded, minisign-verified artifacts in place. macOS
  applies in-app only for Developer ID signed and notarized builds; ad-hoc/local
  builds fall back to the download page.
