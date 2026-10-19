[English](/README.md) | [中文](/README.zh_CN.md)

# Lx-ui

**Lx-ui** 是一个 [Xray-core](https://github.com/XTLS/Xray-core) 服务器 Web 管理面板，
派生（fork）自 MHSanaei 及贡献者开发的 **[3x-ui](https://github.com/MHSanaei/3x-ui)**，
侧重稳定性修复、平滑的升级/迁移路径和可复现的构建。原始工作的全部功劳归于 3x-ui 项目，
详见 [NOTICE](NOTICE) 与 [docs/LX_CHANGES.md](docs/LX_CHANGES.md)。

> [!IMPORTANT]
> 本项目仅供个人使用，请勿用于非法用途。

> [!NOTE]
> **发布仓库。** 本仓库中所有仓库 URL（安装/更新脚本、面板自更新、文档、前端）都指向 `jluo73423-ux/Lx-ui`，
> 它只在一个地方定义：[`tools/rebrand.env`](tools/rebrand.env) 中的 `LXUI_REPO`。如需迁移到其他账号：
>
> ```bash
> sed -i 's#^LXUI_REPO=.*#LXUI_REPO="your-org/Lx-ui"#' tools/rebrand.env
> tools/rebrand.sh          # 重写脚本、自更新地址、文档、前端
> tools/check-rebrand.sh    # 守卫：发现残留的上游下载地址即失败
> ```
>
> CI 构建的发布版会自动写入仓库名（`-X .../internal/config.releaseRepo=$GITHUB_REPOSITORY`）。

## 与 3x-ui 的关系

- 基于 3x-ui v3.10.2（上游提交 `a8d0700b`），并合入了 12 个上游尚未合并的 PR
  （列表见 [docs/UPSTREAM_SYNC.md](docs/UPSTREAM_SYNC.md)）。
- 功能、协议、API 和数据库结构与 3x-ui 相同；上游文档 [docs.sanaei.dev](https://docs.sanaei.dev/zh)
  仍然适用（命令和路径中的 `x-ui` 换成 `lx-ui`）。
- 区别：产品名与路径（`lx-ui`）、独立的发布/更新渠道、移除了赞助商信息流与捐赠链接、
  `LXUI_*` 环境变量、内置从 3x-ui 迁移。
- Go 模块路径刻意保持为 `github.com/mhsanaei/3x-ui/v3`，以便持续低冲突地同步上游。

## 安装

```bash
bash <(curl -Ls https://raw.githubusercontent.com/jluo73423-ux/Lx-ui/main/install.sh)
```

安装指定版本：在末尾追加标签，例如 `v1.0.0`。安装完成后运行 `lx-ui` 打开管理菜单。
发布包会用 `.sha256` 校验。无人值守安装（cloud-init）与 3x-ui 相同，设置 `XUI_NONINTERACTIVE=1`，
见 [deploy/](deploy/)。

Docker 镜像：`ghcr.io/jluo73423-ux/lx-ui`（数据卷 `/etc/lx-ui`）。

| | 3x-ui | Lx-ui |
| --- | --- | --- |
| 服务 / 命令 | `x-ui` | `lx-ui` |
| 程序目录 | `/usr/local/x-ui` | `/usr/local/lx-ui` |
| 数据目录 / SQLite | `/etc/x-ui/x-ui.db` | `/etc/lx-ui/lx-ui.db` |
| 日志 | `/var/log/x-ui` | `/var/log/lx-ui` |
| 环境变量文件 | `/etc/default/x-ui` | `/etc/default/lx-ui` |

## 从 3x-ui 迁移

**推荐方式（systemd 主机）：**

```bash
bash <(curl -Ls https://raw.githubusercontent.com/jluo73423-ux/Lx-ui/main/install.sh) --migrate-from-3x-ui
```

脚本会停止并禁用 `x-ui.service`，把 `/etc/x-ui` 备份为 `/etc/x-ui.bak-<时间戳>`，
将数据复制到 `/etc/lx-ui`（`x-ui.db` 改名为 `lx-ui.db`），把 `/etc/default/x-ui`
复制为 `/etc/default/lx-ui`，然后安装 Lx-ui 并检查 `lx-ui.service` 是否持续运行。
任何一步失败都会**自动回滚**：停止 lx-ui，重新启用并启动 x-ui。

**首次启动自动迁移：** 如果 `/etc/lx-ui` 中没有数据库而 `/etc/x-ui` 存在（且 `x-ui.service`
未在运行），lx-ui 会自行复制该目录并写入 `/etc/lx-ui/.migrated-from-x-ui`。仍把旧数据卷挂载在
`/etc/x-ui` 的 Docker 容器会直接就地使用旧卷，并在日志中提示改为挂载到 `/etc/lx-ui`。
PostgreSQL 用户只需要环境变量文件；当 `/etc/default/lx-ui` 不存在时 lx-ui 也会读取
`/etc/default/x-ui`。

数据库结构未改变，旧数据从不被修改，因此**回滚**始终是：

```bash
systemctl disable --now lx-ui
systemctl enable --now x-ui
```

## 环境变量

`LXUI_*` 是规范前缀。3x-ui 文档中的所有 `XUI_*` 变量继续作为别名有效；两者同时设置时以
`LXUI_` 为准。只使用旧变量名时会记录一行提示。

| 变量 | 说明 | 默认值 |
| --- | --- | --- |
| `LXUI_REPO` | 覆盖自更新和安装脚本使用的发布仓库（`owner/name`） | 来自 `tools/rebrand.env` |
| `LXUI_DB_TYPE` | `sqlite` 或 `postgres` | `sqlite` |
| `LXUI_DB_DSN` | PostgreSQL 连接串 | — |
| `LXUI_DB_FOLDER` | SQLite 目录 | `/etc/lx-ui` |
| `LXUI_LOG_FOLDER` | 日志目录 | `/var/log/lx-ui` |
| `LXUI_LOG_LEVEL` | `debug`、`info`、`warning`、`error` | `info` |
| `LXUI_ENABLE_FAIL2BAN` | 基于 Fail2ban 的 IP 限制 | `true` |
| `LXUI_NODE_TOKEN_KEY_FILE` | 节点令牌密钥文件 | `/etc/lx-ui/node_token_key.json` |

完整列表见 3x-ui 的[环境变量参考](https://docs.sanaei.dev/zh/docs/reference/env-vars)，
把前缀 `XUI_` 换成 `LXUI_` 即可。

## 构建

```bash
cd frontend && npm ci && npm run build && cd ..
go build -o lx-ui .
go test ./...
tools/check-rebrand.sh
```

## 贡献 / 上游同步

见 [CONTRIBUTING.md](CONTRIBUTING.md) 与 [docs/UPSTREAM_SYNC.md](docs/UPSTREAM_SYNC.md)
（分支模型：`upstream-main` 镜像 3x-ui，`lx-base` = 上游 + 合入的 PR，`main` = Lx-ui）。
同样适用于上游的修复也应回馈给 3x-ui。

## 许可证与致谢

与 3x-ui 相同，采用 GPL-3.0，见 [LICENSE](LICENSE) 和 [NOTICE](NOTICE)。
感谢 MHSanaei 及贡献者的 [3x-ui](https://github.com/MHSanaei/3x-ui)、最初的
[X-UI](https://github.com/vaxilu/x-ui)、[Xray-core](https://github.com/XTLS/Xray-core)
以及 NOTICE 中列出的项目。如需支持原作者，请前往 3x-ui 仓库。
