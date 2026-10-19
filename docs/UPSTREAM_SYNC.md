# Upstream synchronisation log

Upstream: https://github.com/MHSanaei/3x-ui (git remote `upstream`, push disabled).

## Branch model

| Branch | Purpose |
|---|---|
| `upstream-main` | Read-only mirror of `upstream/main`. Never commit here. Refresh with `git fetch upstream && git branch -f upstream-main upstream/main`. |
| `lx-base` | Upstream code **before** the rebrand + upstream PRs merged by Lx-ui. Upstream fixes are cherry-picked here first because they apply without conflicts. |
| `main` | Lx-ui (lx-base + rebrand + Lx-ui's own work). |
| `release/x.y` | Maintenance branches, fixes only. |
| `feat/*`, `fix/*` | Topic branches merged into `main` via review. |
| `sync/upstream-YYYYMMDD` | Temporary branch used while syncing upstream. |

## Sync procedure

1. `git fetch upstream`
2. `git log --oneline <last-synced>..upstream/main` and classify each commit.
3. Cherry-pick fixes onto `lx-base` (`git cherry-pick -x <sha>`), then merge
   `lx-base` into `main`. When a commit conflicts with the rebrand on `main`,
   re-run `tools/rebrand.sh` after the merge (it is idempotent).
4. Record the new sync point below.

## Sync points

| Date (UTC+8) | Upstream commit | Notes |
|---|---|---|
| 2026-10-19 | `a8d0700b` | Fork point (v3.10.2 + #6859, #6666). |

## Upstream pull requests merged into lx-base

(filled in by Milestone 1, see below)
