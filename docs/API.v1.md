# Router API v1（JWT + public/admin/internal）

本文件只描述 **JWT** 使用方式下的 `/api/v1/*` 接口（public/admin/internal 分层）。  
旧接口与兼容接口（如 `/api/*`、`/v1/*`）不在本文档范围。

> Swagger/OpenAPI  
> 本文档对应的 OpenAPI 文件为 `docs/openapi.json`，由 swag 注释生成：`scripts/gen-openapi.sh`。

## 认证方式（JWT）
### 请求头
```
Authorization: Bearer <JWT>
```

### JWT 来源（推荐）
- **钱包登录（public/auth 或 public/common/auth）** 会返回 JWT  
- `/api/v1/public/profile` 支持 **钱包 JWT 或 UCAN**

> 说明  
> - 普通用户 JWT 可访问 public 用户侧接口。  
> - 管理员/Root JWT 才能访问 admin 接口。  
> - `/api/v1/public/user/login` 是密码登录（Session/Cookie），不属于 JWT 主路径；如只用 JWT 可忽略。

## 响应格式说明
- 业务接口（public/admin）一般返回：
```json
{
  "success": true,
  "message": "",
  "data": {}
}
```
- OpenAI 兼容接口（/api/v1/public 下的模型调用）返回 **OpenAI 风格响应**。

---

## Public（公开/用户侧）

### 1) 钱包认证（JWT）
#### proto 风格
- `POST /api/v1/public/common/auth/challenge`
- `POST /api/v1/public/common/auth/verify`
- `POST /api/v1/public/common/auth/refreshToken`

#### web3 风格
- `POST /api/v1/public/auth/challenge`
- `POST /api/v1/public/auth/verify`
- `POST /api/v1/public/auth/refresh`
- `POST /api/v1/public/auth/logout`

> 说明  
> - `verify` 支持可选的 `message` 字段（SIWE 标准消息），后端会从消息里解析 `Nonce:` 并校验。  
> - 若不传 `message`，仍使用 challenge 返回的 `challenge` 进行签名验证（兼容旧流程）。

#### 个人 profile（JWT 或 UCAN）
- `GET /api/v1/public/profile`

### 2) 公共信息与找回密码
- `GET /api/v1/public/status`
- `GET /api/v1/public/notice`
- `GET /api/v1/public/about`
- `GET /api/v1/public/home_page_content`
- `GET /api/v1/public/reset_password`
- `POST /api/v1/public/user/reset`

### 3) 钱包 OAuth（JWT 认证链路）
- `GET /api/v1/public/oauth/wallet/nonce`
- `POST /api/v1/public/oauth/wallet/login`
- `POST /api/v1/public/oauth/wallet/bind`（需 JWT / UserAuth）

### 4) 第三方 OAuth（Session/Cookie）
- `GET /api/v1/public/oauth/state`
- `GET /api/v1/public/oauth/github`
- `GET /api/v1/public/oauth/lark`

### 5) 用户自助（JWT）
> 说明：`/api/v1/public/user/login` 是密码登录（Session/Cookie），非 JWT；如只用 JWT 可忽略。

- `POST /api/v1/public/user/register`（无需 JWT）
- `POST /api/v1/public/user/login`（Session/Cookie）
- `GET  /api/v1/public/user/logout`（Session/Cookie）
- `GET  /api/v1/public/user/self`（JWT）
- `PUT  /api/v1/public/user/self`（JWT）
- `DELETE /api/v1/public/user/self`（JWT）
- `GET  /api/v1/public/user/dashboard`（JWT）
- `GET  /api/v1/public/user/spend/overview`（JWT）
- `GET  /api/v1/public/user/available_models`（JWT）
- `GET  /api/v1/public/user/token`（JWT）
- `GET  /api/v1/public/user/aff`（JWT）
- `POST /api/v1/public/user/topup`（JWT）

#### GET /api/v1/public/user/dashboard
用户侧用量统计（兼容旧 `/api/user/dashboard`）。

**Query 参数（可选）**
- `start_timestamp`：起始时间（Unix 秒）
- `end_timestamp`：结束时间（Unix 秒）
- `granularity`：`hour | day | week | month | year`
- `models`：逗号分隔的模型列表（仅统计所选模型）
- `include_meta=1`：返回 `meta.providers`、`meta.granularity`、`meta.start`、`meta.end`

**默认行为**
- 不传参数 → 保持旧逻辑：近 7 天 + day 粒度

#### GET /api/v1/public/user/spend/overview
用户花费总览（消费/充值汇总）。

**Query 参数（可选）**
- `period`：`last_week | last_month | this_year | last_year | last_12_months | all_time`，默认 `last_month`

**返回字段**
- `yesterday_cost` / `yesterday_revenue`：昨日消费/充值（quota）
- `period_cost` / `period_revenue`：周期消费/充值（quota）
- `period_start` / `period_end`：周期开始/结束（Unix 秒）
- `yesterday_start` / `yesterday_end`：昨日开始/结束（Unix 秒）

**说明**
- `last_week`/`last_month` 为上个自然周/月，`this_year` 为本年截至今日，`last_year` 为去年自然年，`last_12_months` 为近 12 个月，`all_time` 为使用以来。

### 6) 个人 Token 管理（JWT）
- `GET    /api/v1/public/token`
- `GET    /api/v1/public/token/search`
- `GET    /api/v1/public/token/:id`
- `POST   /api/v1/public/token`
- `PUT    /api/v1/public/token`
- `DELETE /api/v1/public/token/:id`

### 7) 个人日志（JWT）
- `GET /api/v1/public/log/self`
- `GET /api/v1/public/log/self/stat`
- `GET /api/v1/public/log/self/search`

### 8) 用户侧模型/渠道（JWT）
- `GET /api/v1/public/channel/models`（前端展示供应商/模型，支持 `provider` 与 `model_provider` 过滤；model_provider 可用 gpt/gemini/claude/deepseek/qwen/千问 等别名）
- `GET /api/v1/public/models-all`（全量模型列表，非 OpenAI 兼容）

### 9) OpenAI 兼容的模型调用（JWT）
> 与 OpenAI API 语义一致，只是路径前缀改为 `/api/v1/public`。

#### 模型列表
- `GET /api/v1/public/models`
- `GET /api/v1/public/models/:model`

#### 文本与多模态
- `POST /api/v1/public/chat/completions`
- `POST /api/v1/public/completions`
- `POST /api/v1/public/embeddings`
- `POST /api/v1/public/moderations`
- `POST /api/v1/public/images/generations`

#### 音频
- `POST /api/v1/public/audio/transcriptions`
- `POST /api/v1/public/audio/translations`
- `POST /api/v1/public/audio/speech`

#### 目前未实现（返回 501）
- `POST /api/v1/public/edits`
- `POST /api/v1/public/images/edits`
- `POST /api/v1/public/images/variations`
- `GET/POST/DELETE /api/v1/public/files*`
- `POST/GET /api/v1/public/fine_tuning/*`
- `POST/GET /api/v1/public/assistants/*`
- `POST/GET /api/v1/public/threads/*`

---

## Admin（运营/管理）
> 需 Admin/Root JWT

### 1) 用户管理
- `GET    /api/v1/admin/user`
- `GET    /api/v1/admin/user/search`
- `GET    /api/v1/admin/user/:id`
- `POST   /api/v1/admin/user`
- `POST   /api/v1/admin/user/manage`
- `PUT    /api/v1/admin/user`
- `DELETE /api/v1/admin/user/:id`

### 2) 渠道管理
- `GET    /api/v1/admin/channel`
- `GET    /api/v1/admin/channel/search`
- `GET    /api/v1/admin/channel/:id`
- `GET    /api/v1/admin/channel/test`
- `GET    /api/v1/admin/channel/test/:id`
- `GET    /api/v1/admin/channel/update_balance`
- `GET    /api/v1/admin/channel/update_balance/:id`
- `POST   /api/v1/admin/channel/preview/models`（OpenAI 兼容渠道模型预览）
- `POST   /api/v1/admin/channel`
- `PUT    /api/v1/admin/channel`
- `DELETE /api/v1/admin/channel/disabled`
- `DELETE /api/v1/admin/channel/:id`

#### /api/v1/admin/channel/preview/models
用于创建/编辑渠道时预览模型列表（仅 OpenAI 兼容类）。
- 请求体：
```json
{
  "type": 50,
  "key": "sk-***",
  "base_url": "https://api.openai.com",
  "config": {}
}
```
- 响应体：
```json
{
  "success": true,
  "message": "",
  "data": ["gpt-4o-mini", "gpt-4o", "gpt-3.5-turbo"]
}
```

### 3) 兑换码管理
- `GET    /api/v1/admin/redemption`
- `GET    /api/v1/admin/redemption/search`
- `GET    /api/v1/admin/redemption/:id`
- `POST   /api/v1/admin/redemption`
- `PUT    /api/v1/admin/redemption`
- `DELETE /api/v1/admin/redemption/:id`

### 4) 日志管理
- `GET    /api/v1/admin/log`
- `DELETE /api/v1/admin/log`
- `GET    /api/v1/admin/log/stat`
- `GET    /api/v1/admin/log/search`

### 5) 分组管理
- `GET /api/v1/admin/group`

### 6) 系统配置（Root）
- `GET /api/v1/admin/option`
- `PUT /api/v1/admin/option`

---

## Internal（内部接口）
当前预留，暂无可用接口：
- `/api/v1/internal/*`

---

## 示例（JWT 调用）
```bash
BASE="https://router.yeying.pub"
JWT="<YOUR_JWT>"

# 模型列表
curl -s -H "Authorization: Bearer $JWT" \
  "$BASE/api/v1/public/models"

# Chat Completions
curl -s -H "Authorization: Bearer $JWT" -H "Content-Type: application/json" \
  -d '{"model":"deepseek-chat","messages":[{"role":"user","content":"ping"}],"max_tokens":16,"temperature":0}' \
  "$BASE/api/v1/public/chat/completions"
```
