<p align="right">
  <strong>中文</strong> | <a href="./README.en.md">English</a> | <a href="./README.ja.md">日本語</a>
</p>

<p align="center">
  <a href="https://github.com/yeying-community/router"><img src="https://raw.githubusercontent.com/yeying-community/router/main/web/default/public/logo.png" width="150" height="150" alt="router logo"></a>
</p>

<div align="center">

# Router

_通过标准的 OpenAI API 格式访问所有的大模型，开箱即用_

</div>

> [!NOTE]
> 请在遵循 OpenAI 使用条款及当地法律法规的前提下使用本项目；根据《生成式人工智能服务管理暂行办法》，请勿对中国地区公众提供未经备案的生成式 AI 服务。

## 目录
- [项目名称](#项目名称)
- [项目简介](#项目简介)
- [功能特性](#功能特性)
- [快速开始](#快速开始)
  - [环境要求](#环境要求)
  - [安装步骤](#安装步骤)
  - [配置说明](#配置说明)
- [本地开发](#本地开发)
  - [开发环境搭建](#开发环境搭建)
  - [运行项目](#运行项目)
  - [调试方法](#调试方法)
- [生产部署](#生产部署)
  - [部署前准备](#部署前准备)
  - [部署步骤](#部署步骤)
  - [环境变量配置](#环境变量配置)
  - [健康检查](#健康检查)
- [API 文档](#api文档)
- [测试](#测试)
- [贡献指南](#贡献指南)
- [许可证](#许可证)

## 项目名称
Router

## 项目简介
Router 是一个用 Go 构建的多模型 API 中继网关，前端采用 React。它将主流大模型服务统一为 OpenAI API 兼容接口，提供鉴权、额度、渠道、负载均衡等完整管理能力，适合个人和团队快速搭建稳定的模型接入层。

**技术栈**
- 后端：Go 1.22、Gin、GORM、Redis/SQLite/MySQL/PostgreSQL
- 前端：React 18、react-scripts、Semantic UI；静态资源通过 `go:embed` 内置
- 部署：Docker / Docker Compose / 二进制；系统日志可落地到本地目录

## 功能特性
- 支持 OpenAI、Azure OpenAI、Anthropic、Gemini、Mistral、Claude、Moonshot、DeepSeek、xAI、Ollama 等数十种上游模型渠道，统一 OpenAI API 调用方式。
- 渠道、模型、用户分组与倍率管理；支持失败自动重试、负载均衡、渠道白名单、模型映射。
- 令牌与兑换码管理，额度统计与明细；可按分组设置额度倍率与并发限制。
- 支持 Stream 响应、绘图接口、Cloudflare AI Gateway、Cloudflare Turnstile 校验。
- 多机部署与缓存：Redis/内存缓存、配置定时同步、主从节点模式、前端跳转。
- 可自定义系统名称、Logo、主题与首页/About 内容；内置多语言（中文/英文/日文）。
- 管理 API 暴露，便于无二开扩展功能（详见 `docs/API.md`）。
- 登录方式丰富：邮箱注册/重置、GitHub、飞书、微信公众号、钱包登录（可选 Root 白名单）。

## 快速开始
### 环境要求
| 依赖 | 版本要求 | 用途 |
|------|----------|------|
| Go | >= 1.22 | 后端编译/运行 |
| Node.js | >= 16 | 前端构建/开发 |
| npm / yarn | >= 8 / >= 1.22 | 前端依赖管理 |
| Docker | >= 20 (可选) | 一键部署 |
| Docker Compose | >= 2 (可选) | 编排部署 |
| MySQL | >= 8 (可选) | 生产数据库，推荐高并发场景 |
| Redis | >= 6 (可选) | 缓存与会话，同步配置 |
| SQLite | 内置 | 默认开发/低并发使用 |

### 安装步骤
#### 方式一：Docker（推荐）
- SQLite 快速试用（数据存储在宿主机 `/home/ubuntu/data/router`）：
```bash
docker run --name router -d --restart always -p 3000:3000 \
  -e TZ=Asia/Shanghai \
  -v /home/ubuntu/data/router:/data \
  yeying-community/router:latest
```
- 使用 MySQL（示例：`SQL_DSN` 指向宿主 MySQL）：
```bash
docker run --name router -d --restart always -p 3000:3000 \
  -e SQL_DSN="root:123456@tcp(host.docker.internal:3306)/router" \
  -e TZ=Asia/Shanghai \
  -v /home/ubuntu/data/router:/data \
  yeying-community/router:latest
```
如镜像无法直接拉取，可将 `yeying-community/router` 换为 `ghcr.io/yeying-community/router`。

#### 方式二：Docker Compose
1. 打开 `docker-compose.yml`，根据需要修改 `SQL_DSN`、`REDIS_CONN_STRING`、`SESSION_SECRET` 等环境变量。
2. 运行：
```bash
docker compose up -d
```
3. 查看状态：`docker compose ps`

#### 方式三：源码/二进制部署
```bash
# 1) 克隆代码
git clone https://github.com/yeying-community/router.git
cd router

# 2) （可选）构建前端静态资源
npm install --prefix web/default
npm run build --prefix web/default

# 3) 构建后端二进制
go mod download
go build -ldflags "-s -w" -o router

# 4) 运行
./router --port 3000 --log-dir ./logs
```
访问 http://localhost:3000 登录，默认管理员：用户名 `root`，密码 `123456`。

### 配置说明
- 复制模板：`cp .env.template .env`（按需填写）。
- 常用环境变量示例：
```env
PORT=3000
SESSION_SECRET=replace_with_random_string
SQL_DSN=root:123456@tcp(localhost:3306)/router   # 留空则使用 SQLite
REDIS_CONN_STRING=redis://default:password@localhost:6379
FRONTEND_BASE_URL=https://openai.example.com      # 从节点可设置
THEME=default
INITIAL_ROOT_TOKEN=sk-your-root-token             # 可选：首启自动创建 root 令牌
INITIAL_ROOT_ACCESS_TOKEN=sk-your-admin-token     # 可选：首启自动创建系统管理令牌
TZ=Asia/Shanghai
```
- 命令行参数：`--port <port>`，`--log-dir <dir>`，`--version`，`--help`。

## 本地开发
### 开发环境搭建
1. 安装 Go 1.22+、Node.js 16+、npm/yarn。
2. 克隆仓库：`git clone https://github.com/yeying-community/router.git && cd router`。
3. 复制环境文件：`cp .env.template .env`，按需补充 `SQL_DSN`、`SESSION_SECRET` 等。
4. 后端依赖：`go mod download`；前端依赖：`npm install --prefix web/default`。

### 运行项目
- 后端（使用内置静态资源）：
```bash
PORT=3000 go run ./main.go --log-dir ./logs
```
- 前端热更新开发：保持后端运行，另开终端执行
```bash
npm start --prefix web/default   # 自动代理到 http://localhost:3000
```
- 如修改前端并需要嵌入 Go 二进制，请重新执行 `npm run build --prefix web/default`。

### 调试方法
- 设置 `GIN_MODE=debug` 或在 `.env` 中 `DEBUG=true` 以开启更详细日志。
- 查看日志：`tail -f logs/*.log`（使用 `--log-dir` 指定目录）。
- 推荐 VS Code `Go` 插件或 `dlv debug` 进行断点调试；前端可使用 React DevTools。

## 生产部署
### 部署前准备
- 必须更换 `SESSION_SECRET` 为随机字符串。
- 高并发场景务必改用 MySQL/PostgreSQL（设置 `SQL_DSN`），并建议接入 Redis。
- 配置时区、域名、SSL；确保数据目录持久化（`/data` 挂载）。
- 部署前执行 `go test ./...` 通过所有用例。

### 部署步骤
1) Docker 单机：参见“快速开始”中的 Docker 命令，可选添加 `--privileged=true` 解决少数宿主机限制。
2) Docker Compose：修改 `docker-compose.yml` 后 `docker compose up -d`；更新使用 `docker compose pull && docker compose up -d`。
3) 二进制 + Systemd（示例见 `router.service`）：复制到 `/etc/systemd/system/router.service`，修改路径与端口后执行 `systemctl enable --now router`。

### 环境变量配置
核心变量速查：
| 变量 | 说明 | 示例 |
|------|------|------|
| PORT | 服务监听端口 | 3000 |
| SESSION_SECRET | 会话加密密钥，必须自定义 | 随机字符串 |
| SQL_DSN | 使用 MySQL/PostgreSQL 时的连接串 | `user:pass@tcp(db:3306)/router` |
| SQLITE_PATH | 指定 SQLite 文件路径（默认 `/data/one-api.db`） | `/data/router.db` |
| REDIS_CONN_STRING | 启用 Redis 缓存/会话 | `redis://:pass@redis:6379` |
| NODE_TYPE | 节点角色 `master` / `slave` | slave |
| SYNC_FREQUENCY | 从数据库/Redis 同步配置间隔（秒） | 60 |
| FRONTEND_BASE_URL | 从节点前端跳转地址 | https://openai.example.com |
| RELAY_PROXY | 上游请求代理 | http://127.0.0.1:7890 |
| RELAY_TIMEOUT | 上游请求超时（秒） | 120 |
| THEME | 前端主题 | default |
| INITIAL_ROOT_TOKEN | 首启自动生成的 root 用户令牌 | sk-xxx |
| INITIAL_ROOT_ACCESS_TOKEN | 首启自动生成的系统管理令牌 | sk-xxx |

更多可选项（缓存、指标、钱包登录、请求限流等）见源码 `common/config` 与现有注释，保持未设置即为默认值。

### 健康检查
- API 状态：`curl http://localhost:3000/api/status`
- 预期响应包含 `"success": true`。`docker-compose.yml` 已内置该检查，可直接复用。

## API 文档
- 管理与扩展 API：详见 `docs/API.md`。
- 中继接口遵循 OpenAI API 格式，设置 `Authorization: Bearer <令牌>`，`API Base = https://<host>:<port>/v1`。

## 测试
- 后端：`go test ./...`
- 前端（可选）：`npm test --prefix web/default`

## 贡献指南
- 欢迎 Issue 与 PR，提交前请确保 `gofmt`、`go test ./...`、（如改动前端）`npm test` 通过。
- 复用现有 PR 模板 `pull_request_template.md`，描述变更与验证步骤。
- 新增功能请同时更新文档/配置样例；添加与渠道/鉴权相关的改动建议补充最小可复现步骤。

## 许可证
项目采用 MIT License。基于本项目的衍生版本需在页面底部保留署名及指向本项目的链接；如需去除署名请先获得授权。
