# Router

Router 是一个多模型路由服务，提供 OpenAI 兼容接口、管理后台，以及可随二进制一起发布的前端静态资源。

## 项目简介

- 提供统一的 OpenAI API 兼容入口，接入多家大模型服务
- 提供后台管理、用户与计费相关能力
- 前端位于 `web/`，构建后可随服务端一起发布

## 目录说明

- `cmd/router`：程序入口
- `internal`：后端业务实现与 HTTP 路由
- `web`：管理后台前端
- `scripts`：打包与启动脚本
- `docs`：接口、路由、运营等专题文档

## 本地环境要求

- Go 1.22+
- Node.js 与 npm
- PostgreSQL（`database.sql_dsn` 仅支持 PostgreSQL DSN）

## 本地快速开始

1. 准备本地配置文件：

```bash
cp config.yaml.template config.yaml
```

2. 至少补齐以下配置：

```yaml
database:
  sql_dsn: "postgres://user:password@127.0.0.1:5432/router?sslmode=disable"

auth:
  cookie_secret: "replace-with-random-string"
  jwt_secret: "replace-with-another-random-string"
```

3. 启动后端：

```bash
go mod download
go run ./cmd/router --config ./config.yaml --log-dir ./logs
```

4. 启动前端开发服务器：

```bash
cd web
npm install
VITE_SERVER=http://localhost:3011 npm run dev
```

5. 打开 `http://localhost:5181`。

## 配置说明

- `database.sql_dsn`：必填，且只支持 PostgreSQL DSN。
- `auth.cookie_secret`：不要保留模板中的示例值 `random_string`。
- `auth.jwt_secret`：钱包登录的 access/refresh token 签发与校验依赖该字段；留空会导致相关流程不可用。
- `server.address`：用于密码重置链接、支付回调与跳转 URL 组装；对外部署时应填写可访问地址。
- `ucan.aud`：对外部署或服务端口不是默认 `3011` 时，建议显式设置为 `did:web:<公网域名>`。
- `bootstrap.root_wallet_address`：可选；用于开启“管理员管理管理员”的额外权限，多个地址使用英文逗号分隔。

## 本地验证方式

- 后端启动后访问状态接口：

```bash
curl http://127.0.0.1:3011/api/v1/public/status
```

- 前端启动后访问 `http://localhost:5181`，确认页面能正常加载并请求后端。

## 常见问题

- `config file "./config.yaml" not found`：先执行 `cp config.yaml.template config.yaml`。
- `database.sql_dsn is required and only PostgreSQL is supported`：检查 `database.sql_dsn` 是否已配置为 PostgreSQL DSN。
- 钱包登录或 refresh token 流程失败：检查 `auth.jwt_secret` 是否为空。

## 相关文档

- [生产部署文档](./DEPLOYMENT.md)
- [文档索引](./docs/README.md)
- [接口参考](./API_reference.md)
- [UCAN 能力定义](./docs/UCAN能力定义.md)
- [问题排查](./docs/问题排查.md)
