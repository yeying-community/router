# Claude Token 计费方案

## 一、核心结论

> **计费必须用 Claude 返回的 usage；其他方法只能用于预估，不能用于收费。**

---

## 二、可选方案对比（只保留关键）

### 方案 A：Claude 响应 `usage`（唯一计费方案）

**优点**
- 官方账单来源，100%准确
- 覆盖所有真实消耗（cache / 压缩 / 多轮推理）
- 不会产生计费误差（避免亏损）

**缺点**
- 只能请求后获取（无法提前预估）

---

### 方案 B：官方 `count_tokens`（预估方案）

**优点**
- 官方支持，准确度高
- 可用于发送前估算成本
- 支持复杂输入（tools / 图片 / PDF）

**缺点**
- 只是预估，和实际计费存在偏差
- 需要额外调用一次 API


---

### 方案 C：本地 tokenizer（tiktoken / 第三方）

**优点**
- 本地计算，速度快
- 无额外调用成本

**缺点**
- 非 Claude 官方规则
- 误差不可控（尤其中文 / tools / 多模态）
- 模型升级后会失效


---

## 三、最终推荐方案（直接落地）

### 计费逻辑
- 使用 Claude 返回的 `usage`
- 累加：
  - input_tokens
  - output_tokens
  - cache_read_input_tokens
  - cache_creation_input_tokens
  - iterations（如存在）

---

