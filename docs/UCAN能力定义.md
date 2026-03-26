# UCAN 能力定义（Router 基线）

## 1. 设计目标

- 符合 UCAN 常见能力语义：`with` / `can` / `nb`
- 兼容历史实现：`resource` / `action`
- 兼容 SIWE ReCap 能力声明：`att`
- 可扩展：后续可在 `nb`（约束）中增加模型、配额、路径等条件

## 2. Router 接受的能力输入形态

Router 在校验时会把以下形态统一归一：

1. 标准形态（推荐）

```json
{ "with": "app:all:chat.example.com", "can": "invoke", "nb": {"model": "gpt-5.4"} }
```

2. 兼容形态（历史）

```json
{ "resource": "app:all:chat.example.com", "action": "invoke" }
```

3. ReCap 形态（SIWE statement / recap）

```json
{
  "att": {
    "app:all:chat.example.com": {
      "invoke": [{"model": "gpt-5.4"}]
    }
  }
}
```

## 3. 资源语义

推荐资源格式：`app:<scope>:<appId>`

- 推荐：`app:all:<appId>`
- 兼容：`app:<appId>`（Router 会归一成 `app:all:<appId>`）
- 通配：`app:*`（历史兼容，表示任意 app 资源）

说明：

- Router 仍兼容历史资源 `llm:*`、`router:llm`、`profile`，用于旧 token 平滑迁移。
- Router 不强制 SDK 采用某一种资源命名；能力语义由服务端策略定义。

## 4. 动作语义

当前推荐动作：

- `invoke`：模型调用
- `write`：写入
- `read`：读取

后续可扩展新动作，不影响现有校验流程。

## 5. 归一化与匹配规则

1. 字段归一
- `with/can` 与 `resource/action` 统一映射到内部能力结构
- `cap`、`capabilities`、`att` 三种来源合并去重

2. 资源归一
- `app:<appId>` -> `app:all:<appId>`
- `app:*` 保持不变

3. 匹配
- 支持精确匹配与后缀 `*` 前缀匹配
- 保持旧行为兼容：`app:*` 可匹配 `app:all:<appId>` 与历史 `app:<appId>`

## 6. 扩展约束（nb）

Router 当前保留并透传能力约束 `nb`，用于后续策略扩展，例如：

- 模型限制：`{"model":"gpt-5.4"}`
- 路径限制：`{"path_prefix":"/apps/chat.example.com/"}`
- 配额限制：`{"daily_quota":100000}`

当前版本仅完成解析归一与链路保留，不对 `nb` 做强校验。
