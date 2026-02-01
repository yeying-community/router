# Router 运行/启动/公网/PG 交接说明

## 目的
给后续的 Codex/同事一个“免探查仓库”的运行说明，快速理解如何启动、如何公网访问、以及如何连 PG。

## API 开发必读
凡涉及 **API 新增/修改/迁移**，请先阅读项目根目录 `API_reference.md`，按公司 Interface 规范完成分层（public/admin/internal）与兼容性要求，并同步更新文档。

---

## CodexDev 目录（仓库内）
- 本仓库已自带 `CodexDev/` 用于放置各类 Codex 执行提示与临时说明。
- 请在仓库根目录下使用：`/root/code/router/router_new/CodexDev/`  
  例如：`CodexDev/UI`、`CodexDev/API`。
- **不要在仓库外另建同名目录**，以免误用路径。

---

## 当前实际运行方式
- **systemd 服务**：`/etc/systemd/system/router.service`
- **启动命令**：`/root/code/router/router_new/build/router --port 3011 --log-dir ./logs`
- **工作目录**：`/root/code/router/router_new`  
  这是关键点（保证 `.env` 会被自动加载）
- **监听端口**：本机 `3011`

---

## 公网访问链路
- **Nginx 配置**：`/etc/nginx/conf.d/router.conf`
- **HTTP → HTTPS**：80 端口强制跳 https
- **HTTPS 终止**：证书在 `/etc/letsencrypt/live/router.yeying.pub/`
- **反代目标**：`http://127.0.0.1:3011`

结论：公网访问只依赖 **Nginx 正常 + 本机 3011 服务可用**，程序本身不需要知道域名。

---

## PG 连接方式
- **`.env`** 里配置 `SQL_DSN=postgres://...`
- 入口 `internal/app/app.go` 使用 `godotenv/autoload` 自动加载 `.env`
- DB 类型由 `internal/admin/model/main.go` 判定：  
  - `postgres://` → PostgreSQL  
  - 为空 → SQLite  
  - 其他 → MySQL

---

## 为什么现在是 PG
关键链路：  
`systemd WorkingDirectory` 指向仓库根目录 → `.env` 正确加载 → `SQL_DSN` 是 postgres → 使用 PG

启动日志会打印：  
`[openPostgreSQL] using PostgreSQL as database`

---

## 更新代码是否影响 PG
一般不会，前提：
- `.env` 不被覆盖
- systemd 工作目录仍是仓库根
- `SQL_DSN` 仍是 `postgres://...`

潜在风险：新版本 migration 需要更高 DB 权限。

---

## 新代码（boss_router/main）带来的额外注意点
- **前端改为 Vite**：仍输出 `web/build`，但构建命令是 `npm run build --prefix web`。  
  开发端口为 5181，前端 API 基地址环境变量从 `REACT_APP_SERVER` 改为 `VITE_SERVER`。
- **新增 UCAN（可选）**：支持 `/api/v1/public/profile` 使用 UCAN 或原有钱包 JWT。  
  可选环境变量：`UCAN_AUD` / `UCAN_RESOURCE` / `UCAN_ACTION`。
- **CORS 逻辑更精细**：如需白名单，使用 `CORS_ALLOWED_ORIGINS`（或旧名 `CORS_ORIGINS`）。  
  若设置白名单，必须包含 `https://router.yeying.pub`，否则浏览器会被拦。

---

## 如何确认当前实例确实连 PG
**最可靠：看启动日志**
```
journalctl -u router --since 'today' --no-pager | rg "openPostgreSQL|openSQLite|openMySQL" -S
```
看到 `openPostgreSQL` 即确认。

**辅助：本地日志**
```
rg -n "openPostgreSQL|openSQLite|openMySQL" logs/oneapi-*.log -S
```

**业务侧提示**
- 若突然出现默认 `root/123456`，基本说明误连 SQLite。

---

## 高风险踩坑
- 在非仓库目录手动运行二进制 → `.env` 不加载 → 误用 SQLite
- 改端口只改了一处（Nginx 与程序不一致）→ 502
- 远程 PG 网络/权限异常 → 服务启动失败
