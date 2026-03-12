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

### 启动后端
```bash
git clone https://github.com/yeying-community/router.git
cd router

cp config.yaml.template config.yaml
# 按需编辑 config.yaml：
# - 仅支持 PostgreSQL，默认已填本地开发 DSN
# - 如数据库不在本机，请修改 database.sql_dsn

go mod download
go run ./cmd/router --config ./config.yaml --log-dir ./logs
```
访问 http://localhost:3011。

当前版本不再自动创建默认 `root / 123456` 管理员账户。

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

### 编译后端
```bash
# 如需把前端打包进后端二进制，先构建前端
# npm run build --prefix web

mkdir -p build
go build -o build/router ./cmd/router
```

### 启动前端热更新（可选）
```bash
npm install --prefix web
npm start --prefix web   # 自动代理到 http://localhost:3011
```
如需把前端打包进后端：`npm run build --prefix web` 后重启后端。

## 配置文件与启动（必读）
Router 默认读取当前目录 `config.yaml`，也可通过 `--config` 指定路径。

**生产环境建议至少配置：**
- `database.sql_dsn: postgres://...`（必须为 PostgreSQL）
- `ucan.aud: did:web:<公网域名>`
- `auth.auto_register_enabled: true`（按需开启钱包自动注册）
- `bootstrap.root_wallet_address: "0xabc...,0xdef..."`（按需开启“管理员管理管理员”的额外权限）

若服务端口不是默认 `3011`，请显式设置 `ucan.aud`，否则可能出现 `UCAN audience mismatch`。

## 生产部署速览（公网）
### 1) 非侵入预检查
```bash
ss -lntp
systemctl list-units --type=service --no-pager
```

### 2) 准备配置
```bash
cp config.yaml.template config.yaml
# 编辑 config.yaml，至少设置：
# - database.sql_dsn（PostgreSQL）
# - ucan.aud（did:web:<你的域名>）
```

### 3) 构建并启动
```bash
npm install --prefix web
npm run build --prefix web

mkdir -p build
go build -o build/router ./cmd/router
./build/router --config ./config.yaml --port 13011 --log-dir ./logs
```

### 4) systemd（示例）
```ini
[Service]
WorkingDirectory=/root/code/router
ExecStart=/root/code/router/build/router --config /root/code/router/config.yaml --port 13011 --log-dir ./logs
Restart=on-failure
```

### 5) Nginx 反代（示例）
```nginx
server {
    server_name router.yeying.pub;
    location / {
        proxy_pass http://127.0.0.1:13011;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
    listen 443 ssl;
}
```

### 6) 验证
```bash
journalctl -u router --since 'today' --no-pager | rg "using PostgreSQL as database" -S
curl -s http://127.0.0.1:13011/api/v1/public/status
curl -I https://router.yeying.pub
```
出现 `using PostgreSQL as database` 才算数据库配置正确。

### 7) 常见问题
- 启动报数据库连接错误：检查 `database.sql_dsn`、`--config` 路径与 `WorkingDirectory`。
- 构建报 `web/dist/*` 不存在：先执行 `npm run build --prefix web`。
- `502 Bad Gateway`：检查 Nginx `proxy_pass` 端口和 Router `--port` 是否一致。
- `UCAN audience mismatch`：设置 `ucan.aud: did:web:<公网域名>`。

## 配置与文档
- 配置文件说明与示例：`config.yaml.template`
- API 文档：`docs/接口文档.md`

## 生成 OpenAPI 文档（/api/v1）
```bash
go install github.com/swaggo/swag/cmd/swag@latest
./scripts/gen-openapi.sh
# 输出：docs/swagger/openapi.json 与 docs/swagger/swagger.yaml
```

## 许可证
MIT License
