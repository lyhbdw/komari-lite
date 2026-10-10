# Komari Lite

Komari Lite 是一个面向个人服务器和小型基础设施的自托管监控面板与轻量探针单体仓库（Monorepo）。它基于 Komari 的监控能力维护，专注于节点状态、历史指标、Ping 质量和通知，不包含任何远程控制类危险功能。

本仓库整合了：
- **服务端面板（Server）**：轻量级 Web 面板、REST/RPC 接口、SQLite 降采样存储与通知告警。
- **客户端探针（Agent，位于 `agent/` 目录）**：纯 Go 静态编译、无 CGO 依赖的极轻量系统监控采集器。

当前首个独立版本：`1.0.0`

## 项目定位

这个版本适合只需要“看数据、收告警、查历史”的部署场景：

- 轻量级 Agent 上报节点运行指标
- Web 面板查看节点状态和实时数据
- 查询 CPU、内存、磁盘、网络等历史指标
- 配置 Ping 任务和延迟历史
- 接收节点离线、流量等通知
- 管理消息渠道
- 备份与恢复已有数据
- 支持本地账户登录和双因素认证（2FA）
- 支持默认主题和数据目录中已安装主题的切换与配置

## 与原版相比的优化

本项目以 `komari-monitor/komari` 为基础，针对单纯监控场景进行了精简和加固：

### 1. 移除不使用的控制功能

关闭远程任务、脚本执行、网页终端、远程文件管理、文件传输和剪贴板，降低误操作风险和服务端暴露面。

### 2. 移除插件和远程主题市场

关闭插件安装、插件市场、主题市场、主题导入、在线更新和远程主题下载。保留默认主题，以及数据目录中已安装主题的切换和配置能力，兼容 Emerald 等已有主题。

### 3. 关闭 OAuth/OIDC 和非核心告警

保留本地账户登录和 2FA，关闭 OAuth/OIDC 登录、负载告警和 pprof 性能分析接口。监控重点集中在节点指标、历史数据、Ping 和离线/流量通知。

### 4. 后端和前端同步精简

精简不是只隐藏菜单。Lite 模式同时处理：

- 前端页面路由和管理菜单
- HTTP 路由
- JSON-RPC 方法注册
- 插件、OAuth/OIDC 等运行时初始化
- 远程任务、终端和文件传输相关模块

访问已关闭接口会返回 `404` 或权限拒绝，避免仅隐藏页面后仍可通过接口调用。

### 5. 保留数据库兼容性

不删除原有数据库表，不执行破坏性删表。已有节点、历史指标、Ping 任务、通知设置、本地账户和 2FA 数据可以继续使用，便于从原版迁移和回滚。

### 6. 独立版本和更新检查

Lite 版本使用独立版本序列，当前首版为 `1.0.0`。管理界面的更新检查指向 Lite 自己的 GitHub 仓库，不再把原版仓库的新版本误报为 Lite 版本更新。

## 保留功能

- 节点在线状态和实时指标
- CPU、内存、磁盘、网络等历史数据
- Ping 任务和延迟历史
- 节点离线通知
- 流量通知
- 消息渠道管理
- 数据库备份与恢复
- 本地账户登录
- 双因素认证（2FA）
- 本地已安装主题的切换与配置

## 已关闭功能

- 远程任务、脚本执行
- 网页终端
- 远程文件管理和文件传输
- 剪贴板
- 负载告警
- 插件安装与插件市场
- 主题市场、主题导入、在线更新和远程主题下载
- OAuth/OIDC 登录
- pprof 性能分析接口

## 数据兼容

本项目本身就是监控专用 Lite 版本。关闭的控制面功能已经从路由、RPC 注册和运行时中移除，访问相关接口会返回 `404` 或权限拒绝。

Lite 版本不会删除已有数据库表，也不会执行破坏性删表，已有历史数据和迁移兼容性会保留。升级前请先备份数据目录。

## 快速开始

### 从源码构建

环境要求：

- Go 1.25 或兼容版本
- Node.js 23 或兼容版本
- npm
- CGO 编译环境
- zstd（重新打包前端资源时需要）

前端资源已包含在仓库中。重新构建前端时可直接执行：

```bash
# 构建访客端主题 (Vue 3 + Vite)
./scripts/build-theme.sh

# 构建管理后台 (React + Vite)
./scripts/build-admin.sh
```

构建后端：

```bash
CGO_ENABLED=1 go build -tags nomsgpack,sqlite_omit_load_extension \
  -ldflags "-s -w -X github.com/lyhbdw/komari-lite/utils.CurrentVersion=1.0.0" \
  -o komari .
```

启动服务：

```bash
./komari server
```

默认监听地址为 `0.0.0.0:25774`，数据目录为当前目录下的 `data/`。生产环境建议通过反向代理提供 HTTPS，并限制管理入口访问范围。

### 轻量时序存储配置（可选）

如需进一步减少 SQLite 时序数据存储占用，可在数据库配置中开启轻量保留档位（`metric_retention_profile = "lightweight"`），将保留周期切换为：
- 1 分钟精度：3 小时
- 5 分钟精度：24 小时
- 1 小时精度：7 天
- 1 天精度与指标总历史：30 天

未显式配置时默认使用兼容模式（指标保留 90 天），不会自动缩短或删除已有历史数据。

### Docker 构建

```bash
# 先按上面的步骤生成 Linux amd64 可执行文件
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -tags nomsgpack,sqlite_omit_load_extension \
  -ldflags "-s -w -X github.com/lyhbdw/komari-lite/utils.CurrentVersion=1.0.0" \
  -o komari-linux-amd64 .

docker build \
  --build-arg TARGETOS=linux \
  --build-arg TARGETARCH=amd64 \
  --build-arg OCI_REVISION=1.0.0 \
  --build-arg OCI_VERSION=1.0.0 \
  -t komari-lite:1.0.0 .

docker run -d \
  --name komari \
  -p 25774:25774 \
  -v "$(pwd)/data:/app/data" \
  --restart unless-stopped \
  komari-lite:1.0.0
```

## 部署建议

- 生产升级前备份整个 `data/` 目录。
- 不要把数据库、备份、环境文件、凭据或构建产物提交到 Git。
- 本项目不提供远程执行能力；如需服务器运维，请使用独立且受控的管理工具。
- 只在你拥有或获授权管理的服务器上部署和使用本项目。

## 与原项目的关系

本项目源自 Komari：

<https://github.com/lyhbdw/komari-lite>

Lite 版本使用独立版本号和独立仓库维护，首个版本为 `1.0.0`。上游项目的版本更新不会自动合并到本项目；如需同步更新，应在测试、数据库备份和功能回归后进行。

## 许可证

许可证和原始版权声明请参阅仓库中的 `LICENSE` 与 `NOTICE` 文件。
