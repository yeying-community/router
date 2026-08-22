# Router

Router 是面向运营场景的多模型路由服务，提供 OpenAI 兼容接口、管理后台、模型渠道治理、用户权益和财务计费能力。

## 核心能力

1. 统一 OpenAI 兼容入口，接入多家模型供应商和渠道。
2. 按供应商、渠道、分组、模型和端点治理路由能力。
3. 支持请求内 fallback、运行时禁用、恢复探测和路由日志解释。
4. 支持套餐、余额、请求扣费、采购成本和毛利分析。
5. 前端位于 `web/`，构建后可随服务端一起发布。

## 目录说明

1. `cmd/router`：程序入口。
2. `internal`：后端业务实现与 HTTP 路由。
3. `web`：管理后台前端。
4. `scripts`：打包、启动和健康检查脚本。
5. `docs`：接口、架构、运营、计费和部署文档。

## 本地环境

本地开发需要：

1. Go 1.22+
2. Node.js 和 npm
3. PostgreSQL

`database.sql_dsn` 只支持 PostgreSQL DSN。

## 快速开始

准备配置文件：

```bash
cp config.yaml.template config.yaml
```

至少补齐：

```yaml
database:
  sql_dsn: "postgres://user:password@127.0.0.1:5432/router?sslmode=disable"

auth:
  cookie_secret: "replace-with-random-string"
  jwt_secret: "replace-with-another-random-string"
```

启动后端：

```bash
go mod download
go run ./cmd/router --config ./config.yaml --log-dir ./logs
```

启动前端开发服务器：

```bash
cd web
npm install
VITE_SERVER=http://localhost:3011 npm run dev
```

访问前端：

```text
http://localhost:5181
```

检查后端状态：

```bash
curl http://127.0.0.1:3011/api/v1/public/status
```

## 关键配置

1. `database.sql_dsn`：必填，只支持 PostgreSQL DSN。
2. `auth.cookie_secret`：必须替换模板示例值。
3. `auth.jwt_secret`：钱包登录和 refresh token 依赖该字段。
4. `server.public_url`：密码重置、支付回调和跳转链接需要对外可访问 URL。
5. `cache.type`：只支持 `local` 或 `redis`。
6. `redis.conn_string`：`cache.type: redis` 时必填。
7. `billing_service.base_url`：渠道账务刷新调用独立 Billing 服务。
8. `ucan.aud`：公网部署或域名/端口非默认值时建议显式配置。
9. `ucan.trusted_issuer_dids`：使用 Node 中心化 TOTP/UCAN 登录时必须配置 Node 当前 issuer DID。
10. `bootstrap.root_wallet_address`：按需配置系统级用户管理钱包地址。
11. `identity.node_url`、`identity.app_id`：启用无钱包插件的钱包身份授权码登录；`identity.callback_url` 可选，用于 Router 对外回调地址与 `server.public_url` 不同时。留空时使用 `/api/v1/public/oauth/identity/callback`。该回调地址必须与 Node 应用的 `redirectUris` 精确一致。契约不使用 Passport assertion 或 `subjectId`。

## 文档入口

1. [文档索引](./docs/README.md)
2. [部署手册](./docs/部署手册.md)
3. [OpenAPI](./docs/openapi/router.openapi.yaml)
4. [路由架构 V1](./docs/路由架构V1.md)
5. [路由架构 V2](./docs/路由架构V2.md)
6. [路由架构 V3](./docs/路由架构V3.md)
7. [问题排查](./docs/问题排查.md)
8. [钱包身份登录](./docs/钱包身份登录.md)

## 验证

后端测试：

```bash
go test ./...
```

前端检查：

```bash
npm --prefix web run check
```
