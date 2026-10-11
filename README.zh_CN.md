[English](/README.md) | [فارسی](/README.fa_IR.md) | [العربية](/README.ar_EG.md) | [中文](/README.zh_CN.md) | [Español](/README.es_ES.md) | [Русский](/README.ru_RU.md) | [Türkçe](/README.tr_TR.md)

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./media/3x-ui-dark.png">
    <img alt="3x-ui" src="./media/3x-ui-light.png">
  </picture>
</p>

<p align="center">
  <a href="https://github.com/zenvor/3x-ui/releases"><img src="https://img.shields.io/github/v/release/zenvor/3x-ui" alt="Release"></a>
  <a href="https://github.com/zenvor/3x-ui/actions"><img src="https://img.shields.io/github/actions/workflow/status/zenvor/3x-ui/release.yml.svg" alt="Build"></a>
  <a href="#"><img src="https://img.shields.io/github/go-mod/go-version/zenvor/3x-ui.svg" alt="GO Version"></a>
  <a href="https://github.com/zenvor/3x-ui/releases/latest"><img src="https://img.shields.io/github/downloads/zenvor/3x-ui/total.svg" alt="Downloads"></a>
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
- **订阅转换** — 本 Fork 的 Mihomo 订阅转换，支持入站/客户端选择、IP 限制、User-Agent 过滤、访问日志、独立 SQLite 存储，以及可远程刷新的 YAML 模板。支持 **VLESS + TCP + REALITY**、**VLESS + XHTTP + REALITY**、**VLESS + XHTTP + TLS**、**VLESS + XHTTP** 和 **VLESS + XHTTP + CDN TLS**。
- **Telegram 和 Discord 机器人**，用于远程监控和管理。
- **RESTful API**，支持带作用域、可设置有效期的令牌，并提供面板内置的 API 参考文档。
- **可安装面板 (PWA)** — 将 3X-UI 固定到桌面或手机主屏幕。
- **灵活的存储** — SQLite（默认）或 PostgreSQL。
- **13 种界面语言**，支持深色和浅色主题。
- **Fail2ban 集成**，用于强制执行按客户端的 IP 限制。

## Mihomo 模板管理

在 **订阅转换 → 设置 → Mihomo 配置 → 当前生效模板** 选择内置的 **通用版（默认）**、**Aggressive 版** 或自定义模板，然后点击 **保存**。这是订阅转换的全局设置，对所有 `/feed/:token` Mihomo 主订阅生效；节点列表和其他订阅格式不变。

- [通用版](https://raw.githubusercontent.com/zenvor/mihomo-config/main/mihomo.yaml)：未知域名使用阿里 DoH 直连查询。
- [Aggressive 版](https://raw.githubusercontent.com/zenvor/mihomo-config/main/mihomo-aggressive.yaml)：未知域名使用 Cloudflare DoH 经 `PROXY` 查询。

两版的已知代理域名均使用 Cloudflare，国内直连域名均使用阿里，路由规则等其余设置相同。现有配置默认继续使用通用版。切换保存时先下载并校验目标模板，成功后将选择保存到 `subconverter.db`，重启后保留；下载或校验失败则保存失败，原版本与有效缓存继续使用。界面显示当前生效版本对应的 URL 和缓存更新时间；尚未保存的新选择不会改变这些信息，也不能执行“立即刷新”。

当前生效模板的“立即刷新”和每 10 分钟的后台刷新使用已保存的选择；模板列表也支持分别手动刷新其他模板。每个模板的缓存文件、ETag 和检查记录独立管理，刷新失败保留该模板的有效缓存，不借用其他模板缓存；缺少有效缓存时主订阅返回 503。`__API_DOMAIN__` 与 `__TOKEN__` 仍在生成订阅时替换。

“最后成功检查时间”在每次成功检查（包括远端返回 304）后更新；“缓存更新时间”只在模板内容重新写入缓存时更新。即使收到 200，内容完全相同时也保留原缓存时间，并显示“模板未变化”。失败不会更新成功检查记录。检查记录按模板独立保存，重启后保留；升级前的缓存显示“暂无记录”，直到下一次成功检查。

在 **模板列表 → 新增模板** 填写配置名称和 HTTPS 链接，再点击 **校验并保存**。名称最多 100 个字符；远端内容须为不超过 1 MiB 的有效 YAML，并在至少一个 `proxy-providers` 的 `url` 中同时包含 `__API_DOMAIN__` 和 `__TOKEN__`。新增和编辑会立即保存模板列表；选择它作为当前生效模板后，还需点击设置的 **保存**。关闭设置不会撤销已保存的模板列表操作。

内置模板不可编辑或删除。自定义模板可改名、修改链接或删除；改名保留原缓存与检查时间，修改链接先校验新来源，失败保留原来源和当前订阅。当前生效模板须先切换后才能删除。模板由固定 ID 标识，相同名称或相同链接的不同模板也拥有独立缓存。为保证并发中的订阅读取，删除或修改来源不会立即移除旧缓存文件；旧 ID 不会复用。已有通用版/Aggressive 选择和缓存兼容保留。

自定义来源只接受公网 HTTPS 地址；连接时检查所有 DNS 解析结果，并重新校验重定向，拒绝内网、回环及保留地址。该下载通道直接连接，不使用环境代理。数据模型预留 `upload` 来源，本次未提供文件上传入口或接口。

**保存或刷新后，需要在客户端更新订阅并加载新配置，才能应用变更。** 此功能不会操作客户端正在运行的代理开关。

新增提示提供简体中文、繁体中文及英文；其他界面语言的新增提示暂使用英文。

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
bash <(curl -Ls https://raw.githubusercontent.com/zenvor/3x-ui/main/install.sh)
```

若要安装特定版本，请在命令后附加对应的标签（例如 `v3.8.5`）：

```bash
bash <(curl -Ls https://raw.githubusercontent.com/zenvor/3x-ui/main/install.sh) v3.8.5
```

若要安装滚动更新的 **dev** 版本（来自 `main` 的最新逐次提交预发布版本，而非稳定版本），请传入 `dev-latest`：

```bash
bash <(curl -Ls https://raw.githubusercontent.com/zenvor/3x-ui/main/install.sh) dev-latest
```

安装过程中会生成随机的用户名、密码和访问路径。安装完成后，运行 `x-ui` 打开管理菜单，您可以在其中启动/停止服务、查看或重置登录凭据、管理 SSL 证书等。

每个发布资源都会在其旁边附带一个 `.sha256` 校验和。`install.sh` 和更新程序都会据此校验压缩包，不匹配时中止。

上游的安装、配置、运维与 API 文档请访问 **[docs.sanaei.dev](https://docs.sanaei.dev/zh)**；本 Fork 专有功能请参阅 [项目 Wiki](https://github.com/zenvor/3x-ui/wiki)。

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
docker run -d --cap-add=NET_ADMIN --cap-add=NET_RAW ... zenvorhub/3x-ui
```

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

面板界面提供 13 种语言：

English · فارسی · العربية · 中文（简体） · 中文（繁體） · Español · Русский · Українська · Türkçe · Tiếng Việt · 日本語 · Bahasa Indonesia · Português (Brasil)

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

## 随时间变化的星标数

[![Stargazers over time](https://starchart.cc/zenvor/3x-ui.svg?variant=adaptive)](https://starchart.cc/zenvor/3x-ui)
