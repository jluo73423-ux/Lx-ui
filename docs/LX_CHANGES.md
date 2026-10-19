# Lx-ui changes relative to upstream 3x-ui

This file is the human-readable modification statement required by GPL-3.0
section 5a. Dates are in UTC+8. The authoritative record is the git history.

Fork point: upstream `a8d0700b` (3x-ui v3.10.2 + 2 commits), 2026-10-18.

## 2026-10-19 — Milestone 1

- Phase 0: repository set up, NOTICE and this file added. LICENSE (GPL-3.0) unchanged.
- Phase 2: merged 12 upstream pull requests into `lx-base` (list in docs/UPSTREAM_SYNC.md).
- Removed the upstream AI issue-analyst / PR-review workflows (`claude-*`) and
  their tests.
- Phase 1 — rebrand to Lx-ui, version **1.0.0**:
  - `tools/rebrand.env` is the single source of the identity
    (`LXUI_REPO="<ORG>/Lx-ui"` placeholder); `tools/rebrand.sh` applies it
    reproducibly (the mechanical rebrand is its own commit);
    `tools/check-rebrand.sh` (`make rebrand-check`, part of `make verify`)
    fails on stray upstream download/update/image URLs.
  - Names and paths: `x-ui` → `lx-ui` (binary, CLI, `lx-ui.service`,
    `/usr/local/lx-ui`, `/etc/lx-ui/lx-ui.db`, `/var/log/lx-ui`,
    `/etc/default/lx-ui`), release assets `lx-ui-linux-<arch>.tar.gz`,
    GHCR image `ghcr.io/<owner>/lx-ui` (Docker Hub publishing removed).
  - Install/update/menu scripts download from `LXUI_REPO`; the panel's
    self-update and release lookup use `config.GetReleaseRepo()` and are
    disabled while the placeholder is in place.
  - `LXUI_*` environment variables, with the `XUI_*` names kept as aliases.
  - Migration from 3x-ui: automatic copy of `/etc/x-ui` on first start
    (never modifies the legacy data) and `install.sh --migrate-from-3x-ui`
    with automatic rollback.
  - Removed: sponsor feed/page/API (`/sponsors`), donation links, upstream
    READMEs in languages other than English/Chinese.
  - Frontend brand from `frontend/src/lib/brand.ts`; README in English and
    Chinese crediting 3x-ui.
  - Unchanged on purpose: Go module path `github.com/mhsanaei/3x-ui/v3`,
    database schema, API, internal `3xipl` fail2ban jail names.
  - Known leftovers: logo/icon images are still the upstream artwork (renamed
    files); the paused docs site (`docs/content`) still describes 3x-ui
    paths in places and mentions `/sponsors`.

## 2026-10-19 — Milestone 2 (Phase 3–5)

Status legend: **confirmed** = root cause shown by a test that fails on the
old code; **hypothesis** = plausible cause, not reproduced; **mitigation** =
change that limits damage regardless of the cause.

### Fixes (Phase 3)

- **#6874 TUIC per-user routing: confirmed.** The TUIC bridge fed Xray
  through a no-auth SOCKS relay, so Xray never knew the user and `user`
  routing rules could not match. The relay SOCKS inbound now carries one
  account per TUIC client email (the AmneziaWG approach; disabled clients
  keep their account so toggling does not restart Xray), and a restart is
  only flagged when an email is added or renamed. Proven with an in-process
  xray-core test (`internal/tuic/xray_user_routing_test.go`). Not verified:
  per-user matching for UDP ASSOCIATE traffic.
- **#6870 multi-node client desync: confirmed (logic) / hypothesis (trigger).**
  The master latched a node-reported `enable=false` whenever node and master
  limits matched. It now accepts a node disable only when its own accounting
  (with this tick's deltas) or the node's reported counters prove quota
  exhaustion, or the client is expired; otherwise the master row stays
  enabled, the lifecycle lift marks the node dirty and the client is
  re-pushed. Same rule in Go (`nodeDisableIsStale`) and SQL
  (`ClientTrafficEnableMergeExpr`, SQLite + Postgres). Regression tests in
  `internal/web/service/node_disable_desync_test.go` fail on the old code.
  The real-world trigger (a node acting on a stale `client_global_traffics`
  row after a master reset, or a lagging reconcile) is a hypothesis; the
  node-side window (up to one global push, ~30 s) is not changed. The
  Postgres SQL path was not run locally.
- **#6541 node sync load: mitigation.** New settings
  `nodeHeartbeatInterval` / `nodeTrafficInterval` (5–300 s, default 5 = old
  behaviour) and `nodeSyncJitter` (0–60 s, default 0), applied at panel
  start (Settings → General). The 30 s global-usage push skips a node whose
  payload digest is unchanged (forced refresh every 5 min, full push after
  the node was offline or a push failed). Not done (needs a node protocol
  change): incremental snapshots, merged heartbeat+traffic request, idle
  back-off. Node-down alerts count failed probes, so a longer heartbeat
  interval delays them.
- **#6871 Xray update under the systemd sandbox: mitigation; root cause NOT
  reproduced (hypothesis: a sandbox restriction).** Panel Xray updates now
  keep Xray running during download, stage and validate the new binary in the
  panel's own environment (`xray version` must match, `xray run -test -c
  <generated config>` must pass — `-test` runs the core-creation step that
  fails in the issue), keep the old binary as `<bin>.bak`, and roll back
  automatically if the new core is not running 3 s after start. EPERM errors
  carry a sandbox hint. Reproduction attempt (no systemd on the build box):
  the sandbox was emulated with seccomp `@system-service` lists from systemd
  252 (Debian 12) and 257, the unit's capability bounding set,
  NoNewPrivileges, address-family/namespace restrictions and a read-only
  `/proc/sys` except `/proc/sys/net`; Xray v26.10.10 and v26.10.17 both pass
  `-test` and start with REALITY + Shadowsocks + WireGuard kernel-TUN + TUN
  configs. Denying `ioctl` reproduces the exact error text with a WireGuard
  kernel-TUN outbound, and `-test` catches it. `lx-ui.service` is therefore
  unchanged; `docs/systemd/90-lxui-relaxed.conf.example` is an opt-in escape
  hatch.
- **Xray version policy (new).** `internal/xray/compat.go`: *recommended*
  v26.10.10 (bundled; matches the go.mod xray-core module), *verified*
  v26.9.30 / v26.7.28 / v26.7.11 / v26.6.27 (pins upstream shipped since the
  v26.6.27 minimum; not re-tested against this build), *experimental*
  everything else ≥ v26.6.27. The picker shows tiers (`/getXrayVersionTiers`),
  always offers recommended + verified (even when GitHub is unreachable) and
  lists/installs experimental versions only with `xrayAllowExperimental`
  (default off). A test fails if release.yml / DockerInit.sh drift from the
  recommended version.

### CI / release (Phase 4)

- `release.yml`: single `XRAY_VERSION`, Xray archives verified against the
  XTLS `.dgst` SHA2-256, artifacts `lx-ui-linux-<arch>.tar.gz` (+ `.sha256`)
  for amd64/arm64/armv7/armv6/386/armv5/s390x and `lx-ui-windows-amd64.zip`,
  `config.releaseRepo` injected from `GITHUB_REPOSITORY`.
- `Dockerfile` takes `LXUI_RELEASE_REPO` (docker.yml passes the repository);
  `DockerInit.sh` verifies the Xray checksum; image `ghcr.io/<owner>/lx-ui`.
- `ci.yml`: `rebrand-check` job.
- Validated with actionlint 1.7.12 + shellcheck 0.10.0 (only pre-existing
  shellcheck notes remain). **Never run on GitHub.**

### Verification (Phase 5)

- `make verify` passes locally (codegen freshness, rebrand guard,
  golangci-lint 2.15, oxlint, oxfmt, tsc, Go + vitest suites, frontend build,
  Storybook build). Lint findings fixed on the way.
- New tests: real-SQLite legacy migration (`legacy_migrate_db_test.go`),
  brand/rebrand consistency and guard under `go test`
  (`internal/config/rebrand_consistency_test.go`).
- End-to-end on the build box: `make node-e2e` (real master + node panel
  processes, 36 sub-tests) and a smoke script (build, temp DB, login/CSRF,
  API token, inbound add with Xray binding the port, version tiers, live
  switch to a verified Xray and back with `.bak`, legacy DB migration with the
  real binary), plus a local `docker build` (amd64) of the image, which
  starts Xray 26.10.10 and serves the login page. The docker build exposed a
  milestone-1 regression (`go build main.go` no longer compiled after the
  migration helper moved to its own file), fixed by building the package.
- Not testable here: GitHub Actions, real systemd units / install.sh /
  update.sh on a VM, Postgres, multi-arch (QEMU) images, Windows build,
  panel self-update against a real release repository.
