# Upstream synchronisation log

Upstream: https://github.com/MHSanaei/3x-ui (git remote `upstream`, push disabled).

## Branch model

| Branch                   | Purpose                                                                                                                                                |
| ------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `upstream-main`          | Read-only mirror of `upstream/main`. Never commit here. Refresh with `git fetch upstream && git branch -f upstream-main upstream/main`.                |
| `lx-base`                | Upstream code **before** the rebrand + upstream PRs merged by Lx-ui. Upstream fixes are cherry-picked here first because they apply without conflicts. |
| `main`                   | Lx-ui (lx-base + rebrand + Lx-ui's own work).                                                                                                          |
| `release/x.y`            | Maintenance branches, fixes only.                                                                                                                      |
| `feat/*`, `fix/*`        | Topic branches merged into `main` via review.                                                                                                          |
| `sync/upstream-YYYYMMDD` | Temporary branch used while syncing upstream.                                                                                                          |

## Sync procedure

1. `git fetch upstream`
2. `git log --oneline <last-synced>..upstream/main` and classify each commit.
3. Cherry-pick fixes onto `lx-base` (`git cherry-pick -x <sha>`), then merge
   `lx-base` into `main`. When a commit conflicts with the rebrand on `main`,
   re-run `tools/rebrand.sh` after the merge (it is idempotent).
4. Record the new sync point below.

## Sync points

| Date (UTC+8) | Upstream commit | Notes                                |
| ------------ | --------------- | ------------------------------------ |
| 2026-10-19   | `a8d0700b`      | Fork point (v3.10.2 + #6859, #6666). |

## Upstream pull requests merged into lx-base

Merged 2026-10-19 with `git merge --no-ff refs/pull/<N>/head` (one merge commit
each, message carries `Upstream-PR: MHSanaei/3x-ui#<N>`). All 12 merged without
textual conflicts; `go build`, `go vet`, `go test ./...` (5081 pass / 0 fail),
`npm run gen` (no drift), lint, typecheck and vitest (1852 pass) were green afterwards.

| Order | PR    | Title                                                                                  |
| ----- | ----- | -------------------------------------------------------------------------------------- |
| 1     | #6837 | chore(deps): bump i18next 26.4.3 -> 26.4.4 (ReDoS fix)                                 |
| 2     | #6862 | fix(sub): send no-store on every subscription body                                     |
| 3     | #6850 | fix(outbound): correctly dial IPv6 literals in TCP probes                              |
| 4     | #6857 | fix(inbounds): preserve formatting in Advanced JSON editor (#6854)                     |
| 5     | #6843 | fix(sub): pass numeric XMUX counts to Clash reuse-settings (#6841)                     |
| 6     | #6838 | feat(sub): map shadowsocks tcp http header to mihomo obfs plugin (#6735)               |
| 7     | #6825 | feat(sub): export tls cipher suites as cs query param in share links                   |
| 8     | #6831 | fix(hosts): carry an inbound update's externalProxy change onto its imported hosts     |
| 9     | #6851 | fix(clients): keep a re-created client from being orphaned by a stale delete tombstone |
| 10    | #6833 | fix(traffic): re-enable quota-disabled clients on inbound and reset-all resets         |
| 11    | #6830 | fix(iplimit): keep recording client IPs with fail2ban disabled (#6722)                 |
| 12    | #6768 | feat(notifications): failed-check threshold for node.down alerts                       |

Deliberately not merged: #6866 (empty description, rewrites install.sh, -1951
lines), #6829 (superseded upstream by #6859), #6799 (conflicts), #6845
(sponsor module is removed entirely in Lx-ui), #6847 (overlaps the planned
Xray version pinning), plus large feature/draft PRs. See
`/docs/LX_CHANGES.md`.
