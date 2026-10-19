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
