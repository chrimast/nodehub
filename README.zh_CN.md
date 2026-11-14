[English](/README.md) | [فارسی](/README.fa_IR.md) | [العربية](/README.ar_EG.md) | [中文](/README.zh_CN.md) | [Español](/README.es_ES.md) | [Русский](/README.ru_RU.md) | [Türkçe](/README.tr_TR.md)

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./media/3x-ui-dark.png">
    <img alt="3x-ui" src="./media/3x-ui-light.png">
  </picture>
</p>

<p align="center">
  <a href="https://github.com/MHSanaei/3x-ui/releases"><img src="https://img.shields.io/github/v/release/mhsanaei/3x-ui" alt="Release"></a>
  <a href="https://github.com/MHSanaei/3x-ui/actions"><img src="https://img.shields.io/github/actions/workflow/status/mhsanaei/3x-ui/release.yml.svg" alt="Build"></a>
  <a href="#"><img src="https://img.shields.io/github/go-mod/go-version/mhsanaei/3x-ui.svg" alt="GO Version"></a>
  <a href="https://github.com/MHSanaei/3x-ui/releases/latest"><img src="https://img.shields.io/github/downloads/mhsanaei/3x-ui/total.svg" alt="Downloads"></a>
  <a href="https://www.gnu.org/licenses/gpl-3.0.en.html"><img src="https://img.shields.io/badge/license-GPL%20V3-blue.svg?longCache=true" alt="License"></a>
  <a href="https://pkg.go.dev/github.com/mhsanaei/3x-ui/v3"><img src="https://pkg.go.dev/badge/github.com/mhsanaei/3x-ui/v3.svg" alt="Go Reference"></a>
  <a href="https://docs.sanaei.dev"><img src="https://img.shields.io/badge/docs-docs.sanaei.dev-22d3ee" alt="Documentation"></a>
</p>

**3X-UI** 是一个先进的开源 Web 控制面板，用于管理 [Xray-core](https://github.com/XTLS/Xray-core) 服务器。它提供简洁、多语言的界面，用于部署、配置和监控各种代理与 VPN 协议——从单台 VPS 到多节点部署。

3X-UI 作为原始 X-UI 项目的增强分支（fork），增加了更广泛的协议支持、更好的稳定性、按客户端的流量统计以及许多提升使用体验的功能。

> [!IMPORTANT]
> 本项目仅供个人使用。请勿将其用于非法目的，也请勿在生产环境中使用。

## 功能特性

- **多协议入站** — VLESS、VMess、Trojan、Shadowsocks、WireGuard、AmneziaWG、TUIC v5、Hysteria2、MTProto、HTTP、SOCKS (Mixed)、Dokodemo-door / Tunnel 和 TUN。
- **现代传输与安全** — TCP (Raw)、mKCP、WebSocket、gRPC、HTTPUpgrade 和 XHTTP，并通过 TLS、XTLS 和 REALITY 加密。
- **内置 AmneziaWG** — 抗 DPI 的 WireGuard 直接在面板内的用户态网络栈上运行，无需内核模块、DKMS 或额外软件包。
- **内置 TUIC v5** — 基于 QUIC 的高性能代理，支持原生 UDP 中继流量统计、0-RTT 握手和 BBR 拥塞控制。
- **MTProto 代理** — 按客户端配置 FakeTLS 密钥、广告标签和配额，实时生效且不会断开已有连接。
- **回落 (Fallback)** — 通过 Xray 的 fallback 功能在单个端口上提供多种协议（例如在 443 端口上同时使用 VLESS 和 Trojan）。
- **按客户端管理** — 流量配额、到期日期、可豁免受信任地址的 IP 限制、HWID 设备数限制、定时续期周期、实时在线状态，以及一键分享链接、二维码和订阅。
- **流量统计** — 按入站、按客户端、按出站统计，并支持重置控制。
- **多节点支持** — 从单一面板管理并扩展到多台服务器，并可将入站克隆到其他节点。
- **出站与路由** — WARP、NordVPN、PIA、自定义路由规则、支持均衡器间回退的负载均衡器，以及出站代理链。内置的 geosite 与 geoip 分类可直接在规则编辑器中浏览。
- **内置订阅服务器** — 提供 raw、JSON 和 Clash 输出，可依据客户端 User-Agent 自动选择，并支持[自定义页面模板](docs/custom-subscription-templates.md)。
- **Telegram 和 Discord 机器人**，用于远程监控和管理。
- **RESTful API**，支持带作用域、可设置有效期的令牌，并提供面板内置的 API 参考文档。
- **可安装面板 (PWA)** — 将 3X-UI 固定到桌面或手机主屏幕。
- **灵活的存储** — SQLite（默认）或 PostgreSQL。
- **14 种界面语言**，支持深色和浅色主题。
- **Fail2ban 集成**，用于强制执行按客户端的 IP 限制。

## 截图

<details>
<summary>点击展开</summary>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/01-overview-dark.png">
  <img alt="Overview" src="./media/01-overview-light.png">
</picture>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/02-add-inbound-dark.png">
  <img alt="Inbounds" src="./media/02-add-inbound-light.png">
</picture>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/03-add-client-dark.png">
  <img alt="Add client" src="./media/03-add-client-light.png">
</picture>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/05-add-nodes-dark.png">
  <img alt="Configs" src="./media/05-add-nodes-light.png">
</picture>

</details>

## 快速开始

```bash
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh)
```

若要安装特定版本，请在命令后附加对应的标签（例如 `v3.7.0`）：

```bash
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh) v3.7.0
```

若要安装滚动更新的 **dev** 版本（来自 `main` 的最新逐次提交预发布版本，而非稳定版本），请传入 `dev-latest`：

```bash
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh) dev-latest
```

安装过程中会生成随机的用户名、密码和访问路径。安装完成后，运行 `x-ui` 打开管理菜单，您可以在其中启动/停止服务、查看或重置登录凭据、管理 SSL 证书等。

直接启动面板且用户表为空时，默认账号和密码为 `admin` / `admin`。这是弱口令，请在面板对外开放前于面板设置中修改；已有用户不会被覆盖。安装脚本会按上述流程单独设置凭据。

每个发布资源都会在其旁边附带一个 `.sha256` 校验和。`install.sh` 和更新程序都会据此校验压缩包，不匹配时中止。

完整文档（安装、配置、运维以及完整的 API 参考）请访问 **[docs.sanaei.dev](https://docs.sanaei.dev/zh)**。

### 无人值守安装

安装程序也可以**非交互式**运行，适用于 cloud-init。
设置 `XUI_NONINTERACTIVE=1`（或在无 TTY 的情况下通过管道传入），它就会全程
零提示地完成端到端安装，生成随机凭据并写入
`/etc/x-ui/install-result.env`。请参阅 [`deploy/`](deploy/)：

- [Cloud-init user-data](deploy/cloud-init/) — 在任意云平台上无人值守安装（Hetzner/AWS/DO/Vultr/GCP/Azure/Oracle）
- [Hetzner Cloud 说明](deploy/marketplace/hetzner/) — 在 Hetzner 上基于 cloud-init 的部署

## 支持的平台

**操作系统：** Ubuntu、Debian、Armbian、Fedora、CentOS、RHEL、AlmaLinux、Rocky Linux、Oracle Linux、Amazon Linux、Virtuozzo、Arch、Manjaro、Parch、openSUSE (Tumbleweed / Leap)、Alpine 和 Windows。

**架构：** `amd64` · `386` · `arm64` (aarch64) · `armv7` · `armv6` · `armv5` · `s390x`。

## 数据库选项

3X-UI 支持两种后端，可在安装时选择：

- **SQLite**（默认）— 位于 `/etc/x-ui/x-ui.db` 的单个文件。无需配置，适合中小型部署。
- **PostgreSQL** — 推荐用于大量客户端或多节点设置。安装程序可以为您在本地安装 PostgreSQL，或接受指向现有服务器的 DSN。

运行时通过环境变量选择后端（安装程序会为您写入 `/etc/default/x-ui`）：

```
XUI_DB_TYPE=postgres
XUI_DB_DSN=postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable
```

### 将现有的 SQLite 安装迁移到 PostgreSQL

```bash
x-ui migrate-db --dsn "postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable"
# 然后在 /etc/default/x-ui 中设置 XUI_DB_TYPE 和 XUI_DB_DSN 并重启：
systemctl restart x-ui
```

源 SQLite 文件保持不变；在确认新后端正常工作后，请手动删除它。

### Docker

默认的 `docker compose up -d` 仍使用 SQLite。若要使用捆绑的 PostgreSQL 服务运行，请取消注释 `docker-compose.yml` 中的两行 `XUI_DB_*` 环境变量，并使用该 profile 启动：

```bash
docker compose --profile postgres up -d
```

该镜像捆绑了 Fail2ban（默认启用），用于强制执行按客户端的 **IP 限制**。Fail2ban 使用 `iptables` 封禁违规者，这需要 `NET_ADMIN` 权限。`docker-compose.yml` 已通过 `cap_add` 授予该权限；如果您改用 `docker run` 启动容器，请自行添加这些权限，否则封禁只会被记录而永远不会生效：

```bash
docker run -d --cap-add=NET_ADMIN --cap-add=NET_RAW ... ghcr.io/mhsanaei/3x-ui
```

### 远程 nodehub-agent（实验性）

这是独立的 agent，不是远程 3x-ui 面板，也不支持流量配额；不要用于生产环境。面板主动连接 agent，节点上的 Docker 需支持 Linux host 网络。以下操作使用可信的本仓库检出版本，先检查脚本内容；不要直接执行未经审查的远程脚本。

1. 在远端构建镜像，或在构建机上构建后通过可信渠道传输到远端：

   ```sh
   docker build -f cmd/nodehub-agent/Dockerfile -t nodehub-agent:local .
   # 在构建机执行，然后通过 SSH 将镜像导入远端：
   docker save nodehub-agent:local | ssh user@node-host 'docker load'
   ```

   在构建机执行时需另行复制 `cmd/nodehub-agent/install.sh` 到远端并检查。构建依赖 Dockerfile 指定的 Go 基础镜像和 Xray 下载。Compose 默认名 `ghcr.io/chrimast/nodehub-xui-agent:latest` 只是可覆盖的目标名称，**不保证镜像已发布**；请使用自己构建或确认可用的镜像。

2. 在远端以具备 Docker 权限的用户运行（以下以 root 为例）。证书必须匹配面板访问的域名且受面板信任，私钥应保密；`--listen` 使用远端实际可达的地址：

   ```sh
   sh install.sh --image nodehub-agent:local --listen 0.0.0.0:9443 \
     --tls-cert /etc/nodehub-agent/tls/fullchain.pem \
     --tls-key /etc/nodehub-agent/tls/privkey.pem
   docker logs nodehub-agent
   ```

   默认监听 `127.0.0.1:9443`，跨主机面板无法直接访问；将监听地址改为可达地址后，用防火墙仅允许面板访问管理端口。脚本输出的 URL 若含 `0.0.0.0` 只是监听地址，不是面板应填写的地址。脚本在 `/var/lib/nodehub-agent` 保存 token、identity 和 Xray 配置，目录权限为 `0700`，token 为 `0600`；证书和私钥复制到该目录，续期后需更新副本并重启容器。每次成功安装都会输出 token；请保护终端和日志，避免 token 泄露。已有同名容器时脚本拒绝替换；升级前显式移除旧容器并保留状态目录。root 安装会将目录及相关文件所有者设为 UID/GID 10001。

3. 在面板节点页面添加 **agent** 类型，填写 `https://节点证书域名:9443` 和脚本输出的 token，保存并测试连接；不要把 token 当作普通面板 API token。私网地址须按面板的私网地址允许选项显式配置。安装命令成功只表示容器已创建，仍需检查日志和面板探测结果。host 网络也会开放 Xray 动态配置的入站端口，应分别配置防火墙。丢弃所有 capability 的容器不能绑定特权端口（低于 1024），除非主机另有相关配置；不要为此盲目扩大权限。

仅在可信加密隧道或可信网络中才可改用 HTTP：省略证书参数并显式传 `--http`，例如 `sh install.sh --image nodehub-agent:local --listen 0.0.0.0:9443 --http`；面板地址使用 `http://节点地址:9443`。HTTP 会暴露 token 和配置，不能视为加密传输。

也可不用安装脚本、改用 [Compose 示例](cmd/nodehub-agent/docker-compose.yml.example)。以下在远端以 root 初始化全新专用目录；不要覆盖现有 token：

```sh
install -d -m 700 /var/lib/nodehub-agent
test ! -e /var/lib/nodehub-agent/token || { echo 'token already exists' >&2; exit 1; }
umask 077
openssl rand -hex 32 > /var/lib/nodehub-agent/token
install -m 600 /etc/nodehub-agent/tls/fullchain.pem /var/lib/nodehub-agent/tls-cert.pem
install -m 600 /etc/nodehub-agent/tls/privkey.pem /var/lib/nodehub-agent/tls-key.pem
chown -R 10001:10001 /var/lib/nodehub-agent
export NODEHUB_AGENT_IMAGE=nodehub-agent:local
export NODEHUB_AGENT_LISTEN=0.0.0.0:9443
docker compose -f cmd/nodehub-agent/docker-compose.yml.example --profile https up -d
docker compose -f cmd/nodehub-agent/docker-compose.yml.example --profile https logs
```

`NODEHUB_AGENT_DIR` 可覆盖状态目录（使用绝对路径，预先创建并设置权限）；读取 `/var/lib/nodehub-agent/token` 填入面板。HTTP 模式跳过证书准备，使用 `--profile http`，其余操作相同；两种 profile 只能选一种，切换前停止原服务，且不要与脚本安装的容器共用监听地址或状态目录。详见 [agent 说明](cmd/nodehub-agent/README.md)。

## 环境变量

| 变量 | 说明 | 默认值 |
| --- | --- | --- |
| `XUI_DB_TYPE` | 数据库后端：`sqlite` 或 `postgres` | `sqlite` |
| `XUI_DB_DSN` | PostgreSQL 连接字符串（当 `XUI_DB_TYPE=postgres` 时） | — |
| `XUI_DB_FOLDER` | SQLite 数据库文件所在目录 | `/etc/x-ui` |
| `XUI_DB_MAX_OPEN_CONNS` | 最大打开连接数（PostgreSQL 连接池） | — |
| `XUI_DB_MAX_IDLE_CONNS` | 最大空闲连接数（PostgreSQL 连接池） | — |
| `XUI_INIT_WEB_BASE_PATH` | Web 面板的初始 URI 路径 | `/` |
| `XUI_ENABLE_FAIL2BAN` | 启用基于 Fail2ban 的 IP 限制 | `true` |
| `XUI_LOG_LEVEL` | 日志级别（`debug`、`info`、`warning`、`error`） | `info` |
| `XUI_DEBUG` | 启用调试模式 | `false` |
| `XUI_TUNNEL_HEALTH_MONITOR` | 启用隧道健康监控（探测某个 URL，在连续多次失败后重启 xray；重启会断开所有客户端） | `false` |
| `XUI_TUNNEL_HEALTH_PROXY` | 探测请求所经过的代理；将其指向本地 xray 入站，使探测能够测试隧道（例如 `socks5://127.0.0.1:1080`）。留空表示探测仅检查主机连通性 | — |
| `XUI_TUNNEL_HEALTH_URL` | 用于检测隧道健康状况的探测 URL | `https://www.cloudflare.com/cdn-cgi/trace` |
| `XUI_TUNNEL_HEALTH_INTERVAL` | 两次探测之间的间隔 | `30s` |
| `XUI_TUNNEL_HEALTH_TIMEOUT` | 单次探测的超时时间 | `10s` |
| `XUI_TUNNEL_HEALTH_FAILURES` | 触发重启前的连续失败次数 | `3` |
| `XUI_TUNNEL_HEALTH_COOLDOWN` | 两次连续重启之间的最小间隔 | `5m` |
| `NODE_TOKEN_ENCRYPTION` | 节点 API 令牌的静态加密：`off`、`migration` 或 `required`（注意：无 `XUI_` 前缀） | `off` |
| `XUI_NODE_TOKEN_KEY_FILE` | JSON 密钥环（权限 `0600`），包含活动密钥 ID 及其 base64 编码的 32 字节密钥 | `/etc/x-ui/node_token_key.json` |
| `XUI_NODE_TOKEN_KEY` | 单个 base64 编码的 32 字节密钥，仅在无法加载密钥文件时使用 | — |

完整列表请参阅[环境变量参考](https://docs.sanaei.dev/zh/docs/reference/env-vars)。

## 支持的语言

面板界面提供 14 种语言：

English · فارسی · العربية · 中文（简体） · 中文（繁體） · Español · Русский · Українська · Türkçe · Tiếng Việt · 日本語 · Bahasa Indonesia · Português (Brasil) · Slovenčina

## 贡献

欢迎贡献。在提交 issue 或 pull request 之前，请阅读[贡献指南](/CONTRIBUTING.md)。

## 特别感谢

- [alireza0](https://github.com/alireza0/)

## 致谢

- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (许可证: **GPL-3.0**): _增强的 v2ray/xray 和 v2ray/xray-clients 路由规则，内置伊朗域名，专注于安全性和广告拦截。_
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (许可证: **GPL-3.0**): _此仓库包含基于俄罗斯被阻止域名和地址数据自动更新的 V2Ray 路由规则。_

## 社区工具

社区围绕 3x-ui 构建的工具和集成。

- [terraform-provider-3x-ui](https://github.com/batonogov/terraform-provider-threexui) (许可证: **MIT**): _使用 Terraform / OpenTofu 通过代码管理入站、客户端、面板设置和 Xray 配置。_
- [3X-UI Manager](https://github.com/yukh975/3X-UI-Manager) (许可证: **MIT**): _3x-ui 的原生 Android 客户端 — 仪表板、入站、带二维码分享的客户端、节点以及多面板管理。可在 F-Droid 获取。_

## 支持项目

**如果这个项目对您有帮助，您可以给它一个**:star2:

<a href="https://www.buymeacoffee.com/MHSanaei" target="_blank">
<img src="./media/default-yellow.png" alt="Buy Me A Coffee" style="height: 70px !important;width: 277px !important;" >
</a>

</br>
<a href="https://nowpayments.io/donation/hsanaei" target="_blank" rel="noreferrer noopener">
   <img src="./media/donation-button-black.svg" alt="Crypto donation button by NOWPayments">
</a>

## 星标历史

<a href="https://www.star-history.com/?repos=mhsanaei%2F3x-ui&type=date&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=mhsanaei/3x-ui&type=date&theme=dark&legend=top-left" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=mhsanaei/3x-ui&type=date&legend=top-left" />
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=mhsanaei/3x-ui&type=date&legend=top-left" />
 </picture>
</a>

<p align="center">
 <a href="https://www.star-history.com/mhsanaei/3x-ui">
  <picture><source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/badge?repo=MHSanaei/3x-ui&type=rank&theme=dark" /><source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/badge?repo=MHSanaei/3x-ui&type=rank" /><img alt="Star History Rank" src="https://api.star-history.com/badge?repo=MHSanaei/3x-ui&type=rank" /></picture> <picture><source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/badge?repo=MHSanaei/3x-ui&type=trending&theme=dark" /><source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/badge?repo=MHSanaei/3x-ui&type=trending" /><img alt="GitHub Trending Repository of the Day" src="https://api.star-history.com/badge?repo=MHSanaei/3x-ui&type=trending" /></picture>
 </a>
</p>
