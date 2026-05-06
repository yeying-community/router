# Router 部署文档

## 部署对象

- Linux 主机上的 Router 发布目录
- 通过 `scripts/package.sh` 生成的发布包
- 使用 `scripts/starter.sh` 管理单实例进程

## 安装包信息

- 打包脚本：`./scripts/package.sh`
- 默认输出目录：`./output`
- 产物命名：`router-<tag>-<commit>.tar.gz`
- 产物内容：
  - `build/router`
  - `config.yaml.template`
  - `scripts/starter.sh`
  - `web/dist`

### 打包命令

基于已有 tag 重新打包：

```bash
./scripts/package.sh v0.0.1
```

自动选择下一个 patch 版本并打包：

```bash
./scripts/package.sh
```

说明：

- 不带参数时，脚本会基于 `origin/main` 打包。
- 不带参数时，脚本会在本地创建新 tag，并向 `${PACKAGE_REMOTE:-origin}` 推送该 tag。
- `AUTO_BUILD=false` 时，会跳过自动构建，要求产物里已存在 `build/router` 和 `web/dist`。

## 目标环境准备

- 目标机器需提供 `bash`、`tar` 和可执行权限。
- 目标机器需能访问 PostgreSQL。
- 部署目录需允许创建 `logs/` 与 `run/` 目录。
- 如果使用打包脚本构建安装包，构建机还需要 Go、Node.js 和 npm。
- 使用预打包产物部署到目标机时，不需要在目标机额外安装 Go、Node.js 或 npm。

## 部署步骤

### 1. 生成安装包

```bash
./scripts/package.sh v0.0.1
```

执行完成后，安装包会出现在 `output/` 目录。

### 2. 拷贝安装包到目标机器

```bash
PACKAGE=router-v0.0.1-abcdef0.tar.gz
scp "output/${PACKAGE}" deploy@your-host:/opt/router/
```

请将 `router-v0.0.1-abcdef0.tar.gz` 替换为实际产物名。

### 3. 解压安装包

```bash
PACKAGE=router-v0.0.1-abcdef0
mkdir -p /opt/router/releases
tar -xzf "/opt/router/${PACKAGE}.tar.gz" -C /opt/router/releases
cd "/opt/router/releases/${PACKAGE}"
```

### 4. 准备配置文件

```bash
cp config.yaml.template config.yaml
```

至少确认以下配置项：

- `database.sql_dsn`：必填，且只支持 PostgreSQL DSN。
- `auth.cookie_secret`：必须替换模板示例值。
- `auth.jwt_secret`：如需钱包登录与 refresh token，必须配置。
- `server.address`：如需密码重置链接、支付回调或跳转链接，必须配置为对外可访问地址。
- `ucan.aud`：公网部署或域名/端口非默认值时，建议显式配置。
- `bootstrap.root_wallet_address`：按需配置系统级用户管理钱包地址。

注意：

- `scripts/starter.sh` 启动时始终传入 `--port` 和 `--log-dir`。
- 因此，通过 `starter.sh` 启动时，实际监听端口和日志目录由 `ROUTER_PORT`、`ROUTER_LOG_DIR` 或脚本默认值控制，而不是 `config.yaml` 中的 `server.port`、`server.log_dir`。

### 5. 启动服务

使用默认端口 `3011` 和默认日志目录 `./logs`：

```bash
./scripts/starter.sh start
```

如需覆盖端口或日志目录：

```bash
ROUTER_PORT=3011 ROUTER_LOG_DIR=/opt/router/logs ./scripts/starter.sh start
```

常用命令：

```bash
./scripts/starter.sh stop
./scripts/starter.sh restart
```

## 验证方式

1. 检查启动输出是否包含 `Router started`。
2. 检查 PID 文件是否生成：

```bash
cat run/router.pid
```

3. 检查状态接口：

```bash
curl http://127.0.0.1:3011/api/v1/public/status
```

如使用了 `ROUTER_PORT`，请替换为实际端口。

4. 检查日志：

```bash
tail -n 50 logs/starter.log
tail -n 50 logs/error.log
```

如使用了 `ROUTER_LOG_DIR`，请替换为实际日志目录。

## 回滚方式

部署新版本前，请保留上一个已验证可用的解压目录。

回滚步骤：

1. 进入当前运行版本目录并停止服务：

```bash
cd /opt/router/releases/router-v0.0.2-deadbee
./scripts/starter.sh stop
```

2. 进入上一个版本目录，确认该目录下的 `config.yaml` 可用后重新启动：

```bash
cd /opt/router/releases/router-v0.0.1-abcdef0
./scripts/starter.sh start
```

说明：

- `starter.sh` 的 PID 文件位于各自版本目录下的 `run/router.pid`。
- 回滚前必须先停止当前版本，否则可能因端口占用导致旧版本启动失败。

## 常见问题

- 执行 `./scripts/package.sh` 后出现新 tag：这是脚本默认行为；不带参数时会自动创建并推送下一个 patch tag。
- 修改了 `config.yaml` 中的 `server.port`，但启动端口没变：`starter.sh` 会用 `ROUTER_PORT` 或默认值 `3011` 覆盖该配置。
- 修改了 `config.yaml` 中的 `server.log_dir`，但日志目录没变：`starter.sh` 会用 `ROUTER_LOG_DIR` 或默认值 `./logs` 覆盖该配置。
- `starter.sh` 提示 `Binary not found or not executable`：检查发布目录下是否存在 `build/router`，以及打包过程是否成功。
