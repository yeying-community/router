<p align="right">
  <strong>中文</strong> | <a href="./README.en.md">English</a> | <a href="./README.ja.md">日本語</a>
</p>

<p align="center">
  <a href="https://github.com/yeying-community/router"><img src="https://raw.githubusercontent.com/yeying-community/router/main/web/public/logo.png" width="150" height="150" alt="router logo"></a>
</p>

<div align="center">

# Router

_通过标准的 OpenAI API 格式访问多家大模型服务，带管理后台，前端可内置到二进制中。_

</div>

## 快速开始（开发）

### 环境要求
- Go 1.22+
- Node.js 16+
- npm/yarn
- PostgreSQL 14+（必需，当前仅支持 PostgreSQL）

### 准备数据库
请先准备一个可访问的 PostgreSQL 实例，并在 `config.yaml` 中配置：

```yaml
database:
  sql_dsn: "postgres://user:password@127.0.0.1:5432/router?sslmode=disable"
```

说明：
- `database.sql_dsn` 必须是 PostgreSQL DSN。
- 程序启动时会自动执行当前基线 migration。

### 启动后端
```bash
git clone https://github.com/yeying-community/router.git
cd router

cp config.yaml.template config.yaml
# 按需编辑 config.yaml：

# 启动后端
go mod download
go run ./cmd/router --config ./config.yaml --log-dir ./logs

# 启动前端
cd web
npm run dev

```
访问 http://localhost:5181/

如需开启“管理员管理管理员”的额外权限，请在 `config.yaml` 中配置：

```yaml
bootstrap:
  root_wallet_address: "0xabc...,0xdef..."
```
说明：
- 支持多个钱包地址，使用英文逗号分隔。
- 所有管理员都能看到“用户”页面。
- 只有这些钱包地址对应的登录用户，才拥有“处理管理员账户”的额外能力，例如删除其他管理员、修改其他管理员角色。
- 如果该配置留空，则普通管理员仍可管理普通用户，但不能处理管理员账户。

- `database.sql_dsn: postgres://...`（必须为 PostgreSQL）
- `ucan.aud: did:web:<公网域名>`
- `auth.auto_register_enabled: true`（按需开启钱包自动注册）
- `bootstrap.root_wallet_address: "0xabc...,0xdef..."`（按需开启“管理员管理管理员”的额外权限）
- UCAN 能力定义基线见 [`docs/UCAN能力定义.md`](./docs/UCAN能力定义.md)

若服务端口不是默认 `3011`，请显式设置 `ucan.aud`，否则可能出现 `UCAN audience mismatch`。

## 打包发布

仓库内提供了打包脚本 [`scripts/package.sh`](./scripts/package.sh)，用于生成带前端静态资源的发布包。

### 前置条件
- 本地已安装 Go、Node.js、npm
- 本地可正常执行 `go build` 和 `npm run build --prefix web`
- 仓库内存在 `config.yaml.template`
- 已配置好打包用 remote，默认 `origin`

### 基本用法

自动选择下一个 patch 版本并打包：

```bash
./scripts/package.sh
```

默认行为：
- 先从 remote 拉取最新 refs 和 tags
- 默认基于 `origin/main` 的最新 commit 打包
- 不直接改动当前工作区，而是使用临时 worktree 构建产物

基于已有 tag 重新打包：

```bash
./scripts/package.sh v0.0.1
```

### 脚本行为
- 默认会自动构建前端和后端：`AUTO_BUILD=true`
- 默认先执行 `git fetch <remote> --prune --tags`
- 前端产物来自 `web/dist`
- 后端二进制输出到 `build/router`
- 输出压缩包目录为 `output/`
- 构建过程基于临时 worktree，避免当前工作区未提交改动影响打包结果
- 产物中会包含：
  - `build/router`
  - `config.yaml.template`
  - `scripts/starter.sh`
  - `web/dist`

### 解压后使用

```bash
tar -xzf output/router-v0.0.1-<commit>.tar.gz
cd router-v0.0.1-<commit>
cp config.yaml.template config.yaml
# 按需编辑 config.yaml
./scripts/starter.sh start
```

`starter.sh` 支持以下命令：
- `./scripts/starter.sh start`
- `./scripts/starter.sh stop`
- `./scripts/starter.sh restart`

### 可选环境变量

```bash
PACKAGE_REMOTE=origin ./scripts/package.sh
AUTO_BUILD=false ./scripts/package.sh v0.0.1
KEEP_STAGE=1 ./scripts/package.sh
PROJECT_NAME=router ./scripts/package.sh
```

说明：
- `PACKAGE_REMOTE`：推送 tag 使用的远端名称，默认 `origin`
- `AUTO_BUILD=false`：跳过自动构建，要求事先准备好 `build/router` 和 `web/dist`
- `KEEP_STAGE=1`：保留解包后的临时目录，便于检查产物
- `PROJECT_NAME`：覆盖输出包名前缀
