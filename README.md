[English](/README.md) | [中文](/README.zh_CN.md)

# Lx-ui

**Lx-ui** is a web control panel for [Xray-core](https://github.com/XTLS/Xray-core)
servers. It is a fork of **[3x-ui](https://github.com/MHSanaei/3x-ui)** by MHSanaei
and contributors, focused on stability fixes, a clean upgrade path and a
reproducible build. All credit for the original work goes to the 3x-ui project;
see [NOTICE](NOTICE) and [docs/LX_CHANGES.md](docs/LX_CHANGES.md).

> [!IMPORTANT]
> Lx-ui is intended for personal use. Do not use it for illegal purposes.

> [!NOTE]
> **Release repository.** Every repository URL in this tree (install/update
> scripts, panel self-update, docs, frontend) points at `jluo73423-ux/Lx-ui`.
> It is defined in exactly one place, [`tools/rebrand.env`](tools/rebrand.env)
> (`LXUI_REPO`). To move the project to another owner:
>
> ```bash
> sed -i 's#^LXUI_REPO=.*#LXUI_REPO="your-org/Lx-ui"#' tools/rebrand.env
> tools/rebrand.sh          # rewrites scripts, self-update URL, docs, frontend
> tools/check-rebrand.sh    # guard: fails on stray upstream download URLs
> ```
>
> Release builds made by CI stamp the repository automatically
> (`-X .../internal/config.releaseRepo=$GITHUB_REPOSITORY`).

## Relation to 3x-ui

- Based on 3x-ui v3.10.2 (upstream commit `a8d0700b`), plus 12 upstream pull
  requests that were not yet merged upstream (list in
  [docs/UPSTREAM_SYNC.md](docs/UPSTREAM_SYNC.md)).
- Feature set, protocols, API and database schema are those of 3x-ui; the
  upstream documentation at [docs.sanaei.dev](https://docs.sanaei.dev) applies
  (replace `x-ui` with `lx-ui` in commands and paths).
- Differences: product name and paths (`lx-ui`), own release/update channel,
  no sponsor feed or donation links, `LXUI_*` environment variables, built-in
  migration from 3x-ui.
- The Go module path stays `github.com/mhsanaei/3x-ui/v3` on purpose, so
  upstream changes can keep being merged with minimal conflicts.

## Install

```bash
bash <(curl -Ls https://raw.githubusercontent.com/jluo73423-ux/Lx-ui/main/install.sh)
```

A specific version: append its tag, e.g. `v1.0.0`. After installation run
`lx-ui` for the management menu. Release archives are verified against their
`.sha256` sums. Non-interactive installs (cloud-init) work as in 3x-ui with
`XUI_NONINTERACTIVE=1`; see [deploy/](deploy/).

Docker image: `ghcr.io/jluo73423-ux/lx-ui` (data volume `/etc/lx-ui`).

| | 3x-ui | Lx-ui |
| --- | --- | --- |
| service / CLI | `x-ui` | `lx-ui` |
| program folder | `/usr/local/x-ui` | `/usr/local/lx-ui` |
| data folder / SQLite | `/etc/x-ui/x-ui.db` | `/etc/lx-ui/lx-ui.db` |
| logs | `/var/log/x-ui` | `/var/log/lx-ui` |
| env file | `/etc/default/x-ui` | `/etc/default/lx-ui` |

## Migrating from 3x-ui

**Recommended (systemd hosts):**

```bash
bash <(curl -Ls https://raw.githubusercontent.com/jluo73423-ux/Lx-ui/main/install.sh) --migrate-from-3x-ui
```

The script stops and disables `x-ui.service`, backs up `/etc/x-ui` to
`/etc/x-ui.bak-<timestamp>`, copies the data to `/etc/lx-ui`
(`x-ui.db` becomes `lx-ui.db`), copies `/etc/default/x-ui` to
`/etc/default/lx-ui`, installs Lx-ui and checks that `lx-ui.service` stays
active. If anything fails it **rolls back automatically**: lx-ui is stopped and
x-ui is re-enabled and restarted.

**Automatic on first start:** when `/etc/lx-ui` has no database and
`/etc/x-ui` exists (and `x-ui.service` is not running), lx-ui copies the
folder itself and writes `/etc/lx-ui/.migrated-from-x-ui`. Docker containers
that still mount the old volume at `/etc/x-ui` use it in place and log a hint
to remount it at `/etc/lx-ui`. PostgreSQL users only need the env file, which
lx-ui also reads from `/etc/default/x-ui` when no `/etc/default/lx-ui` exists.

The database schema is unchanged and the legacy data is never modified, so
**rollback** is always:

```bash
systemctl disable --now lx-ui
systemctl enable --now x-ui
```

## Environment variables

`LXUI_*` is the canonical prefix. Every `XUI_*` variable documented by 3x-ui
keeps working as an alias; when both are set the `LXUI_` value wins. A
one-line note is logged when only legacy names are used.

| Variable | Description | Default |
| --- | --- | --- |
| `LXUI_REPO` | Override the release repository (`owner/name`) for self-update and the install scripts | from `tools/rebrand.env` |
| `LXUI_DB_TYPE` | `sqlite` or `postgres` | `sqlite` |
| `LXUI_DB_DSN` | PostgreSQL DSN | — |
| `LXUI_DB_FOLDER` | SQLite folder | `/etc/lx-ui` |
| `LXUI_LOG_FOLDER` | Log folder | `/var/log/lx-ui` |
| `LXUI_LOG_LEVEL` | `debug`, `info`, `warning`, `error` | `info` |
| `LXUI_ENABLE_FAIL2BAN` | Fail2ban-based IP limits | `true` |
| `LXUI_NODE_TOKEN_KEY_FILE` | Node-token keyring | `/etc/lx-ui/node_token_key.json` |

The full list is the 3x-ui
[environment variables reference](https://docs.sanaei.dev/docs/reference/env-vars)
with the prefix `XUI_` → `LXUI_`.

## Building

```bash
cd frontend && npm ci && npm run build && cd ..
go build -o lx-ui .
go test ./...
tools/check-rebrand.sh
```

## Contributing / upstream sync

See [CONTRIBUTING.md](CONTRIBUTING.md) and [docs/UPSTREAM_SYNC.md](docs/UPSTREAM_SYNC.md)
(branch model: `upstream-main` mirrors 3x-ui, `lx-base` = upstream + merged
PRs, `main` = Lx-ui). Fixes that also apply upstream should be offered to
3x-ui as well.

## License and credits

GPL-3.0, same as 3x-ui — see [LICENSE](LICENSE) and [NOTICE](NOTICE).
Lx-ui exists thanks to [3x-ui](https://github.com/MHSanaei/3x-ui) by MHSanaei
and its contributors, the original [X-UI](https://github.com/vaxilu/x-ui),
[Xray-core](https://github.com/XTLS/Xray-core) and the projects listed in NOTICE.
If you want to support the original author, see the 3x-ui repository.
