# 渠道高级设置

本文档介绍渠道管理中的高级配置选项，帮助管理员精细控制渠道行为、优化流量分配、实现灵活的模型管理。

## 快速导航

| 功能 | 说明 |
|------|------|
| [模型映射](#模型映射) | 重定向模型请求 |
| [状态码映射](#状态码映射) | 自定义错误处理 |
| [优先级与权重](#优先级与权重) | 流量分配策略 |
| [自动禁用](#自动禁用) | 故障自动处理 |
| [请求头覆盖](#请求头覆盖) | 自定义请求头 |
| [参数覆盖](#参数覆盖) | 强制请求参数，支持条件判断与动态调整 |
| [多 Key 模式](#多-key-模式) | 多密钥轮询与状态管理 |
| [标签管理](#标签管理) | 渠道分类筛选 |

---

## 模型映射

模型映射允许将用户请求的模型名称映射到实际使用的模型，实现无感知的模型切换和统一管理。

### 使用场景

| 场景 | 示例 |
|------|------|
| **模型升级** | `gpt-4` → `gpt-4-turbo`，用户无感知 |
| **模型替代** | `claude-3-opus` → `claude-3-5-sonnet` |
| **统一命名** | 将不同提供商的模型统一命名 |
| **成本优化** | 高价模型自动映射到同类型低价模型 |

### 配置格式

在渠道编辑页面的 **模型映射** 字段中，以 JSON 格式配置：

```json
{
  "gpt-4": "gpt-4-turbo",
  "gpt-3.5-turbo": "gpt-3.5-turbo-16k",
  "claude-3-opus": "claude-3-5-sonnet-20241022"
}
```

::: tip 映射规则
- 左侧（Key）：用户请求的模型名称
- 右侧（Value）：实际发送给上游的模型名称
- 未配置映射的模型将原样传递
:::

### 进阶用法

**多级映射**：支持链式映射，如 `gpt-4` → `gpt-4-turbo` → `gpt-4-turbo-preview`

---

## 状态码映射

状态码映射允许自定义上游 API 错误响应，将技术性错误转换为用户友好的提示信息。

### 配置格式

```json
{
  "400": "请求参数错误，请检查输入格式",
  "401": "认证失败，请检查 API Key 是否有效",
  "403": "访问被拒绝，请检查账户权限",
  "429": "请求过于频繁，请稍后重试",
  "500": "上游服务异常，正在处理中",
  "502": "网关错误，请联系管理员",
  "503": "服务暂时不可用，请稍后重试"
}
```

### 应用场景

::: code-group
```json [友好提示]
{
  "429": "当前请求量较大，请等待 30 秒后重试"
}
```

```json [静默处理]
{
  "500": ""
}
```

```json [提示信息]
{
  "401": "请前往设置页面更新 API Key"
}
```
:::

---

## 优先级与权重

优先级和权重用于控制请求在不同渠道之间的路由策略。

### 优先级设置

优先级决定渠道的选择顺序：

| 设置 | 行为 |
|------|------|
| **数值越大** | 优先级越高，优先被选中 |
| **相同优先级** | 按权重分配请求 |
| **负数优先级** | 作为备用渠道，仅在必要时使用 |

**示例配置**：

| 渠道 | 优先级 | 说明 |
|------|:------:|------|
| 主渠道 | 10 | 正常情况下优先使用 |
| 备用渠道 A | 5 | 主渠道不可用时使用 |
| 备用渠道 B | 0 | 最后备选 |

### 权重设置

权重用于在相同优先级的渠道之间分配流量：

| 渠道 | 优先级 | 权重 | 流量分配 |
|------|:------:|:----:|:--------:|
| 渠道 A | 10 | 70 | 70% |
| 渠道 B | 10 | 30 | 30% |
| 渠道 C | 10 | 0 | 0%（不分配） |

::: warning 权重为 0
权重设置为 0 的渠道不会分配流量，但仍然可用于手动测试或特定场景调用。
:::

### 流量分配示例

**场景**：混合使用官方 API 和代理服务

```json
{
  "channels": [
    { "name": "OpenAI 官方", "priority": 10, "weight": 20 },
    { "name": "Azure GPT-4", "priority": 10, "weight": 50 },
    { "name": "代理服务", "priority": 5, "weight": 100 }
  ]
}
```

**结果**：
- 70% 流量由 Azure 处理（50/70）
- 30% 流量由 OpenAI 官方处理（20/70）
- 代理服务仅在上述渠道不可用时启用

---

## 自动禁用

当渠道连续请求失败时，系统会自动禁用该渠道，防止持续失败影响用户体验。

### 配置选项

| 选项 | 默认值 | 说明 |
|------|:------:|------|
| **启用自动禁用** | 开启 | 是否在连续失败后自动禁用 |
| **失败阈值** | 5 次 | 连续失败多少次后禁用 |
| **恢复间隔** | 300 秒 | 禁用多久后自动尝试恢复 |

### 适用场景

| 场景 | 建议设置 |
|------|----------|
| **生产环境** | 开启自动禁用，保障整体服务稳定性 |
| **测试渠道** | 关闭自动禁用，避免测试失败影响服务 |
| **备用渠道** | 开启自动禁用，但设置较低优先级 |

### 手动恢复

被自动禁用的渠道可以通过以下方式恢复：

1. **手动启用**：在渠道详情中点击「启用」
2. **自动恢复**：等待恢复间隔后系统自动尝试

::: tip 最佳实践
建议为关键模型配置多个渠道，开启自动禁用功能，实现故障自动转移。
:::

---

## 请求头覆盖

请求头覆盖允许自定义发送给上游 API 的 HTTP 请求头，适用于特殊认证或代理场景。

### 配置格式

```json
{
  "X-Custom-Header": "custom-value",
  "X-Api-Version": "v2",
  "Authorization": "Bearer ${key}"
}
```

::: tip 变量替换
支持使用 `${key}` 占位符，系统会自动替换为实际的 API Key。
:::

### 使用场景

**自定义认证**：
```json
{
  "X-API-Key": "your-custom-key",
  "X-Organization": "your-org-id"
}
```

**代理服务**：
```json
{
  "X-Proxy-Auth": "proxy-token",
  "X-Forwarded-For": "client-ip"
}
```

---

## 参数覆盖

参数覆盖允许强制设置或覆盖请求参数，确保某些参数始终使用指定值。

::: warning 参数优先级
参数覆盖的优先级高于用户请求中的参数，即使用户指定了不同的值，也会被覆盖。
:::

参数覆盖功能只能用于兼容合法上游接口格式、企业网络兼容和请求规范化。

### 简单覆盖模式

向前兼容模式，直接指定要覆盖的字段和值，系统会将这些字段合并到原始请求中：

```json
{
  "temperature": 0.8,
  "max_tokens": 2000,
  "model": "gpt-4"
}
```

### 高级操作模式

通过 `operations` 数组定义复杂的参数操作，支持条件判断、数组操作、字符串拼接与字符串规范化等高级功能。

#### 基本结构

```json
{
  "operations": [
    {
      "path": "temperature",
      "mode": "set",
      "value": 0.8,
      "conditions": [
        {
          "path": "model",
          "mode": "contains",
          "value": "gpt-4"
        }
      ],
      "logic": "AND"
    }
  ]
}
```

**字段说明（按需填写）：**

- `mode`: 必填
- `path`: 适用于 `set` / `delete` / `append` / `prepend` / `trim_prefix` / `trim_suffix` / `ensure_prefix` / `ensure_suffix` / `trim_space` / `to_lower` / `to_upper` / `replace` / `regex_replace`
- `value`: 常见于 `set` / `append` / `prepend` / `trim_prefix` / `trim_suffix` / `ensure_prefix` / `ensure_suffix`
- `from` / `to`: 适用于 `move` / `copy` / `replace` / `regex_replace`
- `keep_origin`: 用于 `set`（已有值则跳过）以及对象合并时的 `append` / `prepend`

### 操作模式 (mode)

#### 1. set - 设置值

设置指定路径的值：

```json
{
  "path": "temperature",
  "mode": "set",
  "value": 0.8,
  "keep_origin": false
}
```

**参数说明：**
- `keep_origin`: 为 `true` 时，如果目标路径已存在值则跳过设置

#### 2. delete - 删除字段

删除指定路径的字段：

```json
{
  "path": "messages.0",
  "mode": "delete"
}
```

#### 3. move - 移动字段

将一个字段的值移动到另一个位置：

```json
{
  "mode": "move",
  "from": "messages.0.content",
  "to": "system"
}
```

#### 4. append - 追加内容

在现有内容后追加新内容：

```json
{
  "path": "messages.0.content",
  "mode": "append",
  "value": "\n\n请用中文回答。"
}
```

**支持的数据类型：**
- **字符串**: 在原字符串末尾追加
- **数组**: 在数组末尾添加元素（支持添加单个元素或数组）
- **对象**: 合并对象属性

#### 5. prepend - 前置内容

在现有内容前添加新内容：

```json
{
  "path": "messages.0.content",
  "mode": "prepend",
  "value": "重要提示：请仔细阅读以下内容。\n\n"
}
```

**支持的数据类型：**
- **字符串**: 在原字符串开头前置
- **数组**: 在数组开头添加元素（支持添加单个元素或数组）
- **对象**: 合并对象属性

#### 6. copy - 复制字段

将 `from` 指定路径的值复制到 `to` 指定路径（不删除源字段）：

```json
{
  "mode": "copy",
  "from": "model",
  "to": "original_model"
}
```

#### 7. trim_prefix - 去除前缀

对字符串字段去除指定前缀（若不匹配则不变）：

```json
{
  "path": "model",
  "mode": "trim_prefix",
  "value": "openai/"
}
```

#### 8. trim_suffix - 去除后缀

对字符串字段去除指定后缀（若不匹配则不变）：

```json
{
  "path": "model",
  "mode": "trim_suffix",
  "value": "-latest"
}
```

#### 9. ensure_prefix - 确保前缀

确保字符串字段以指定前缀开头（已存在则不变）：

```json
{
  "path": "model",
  "mode": "ensure_prefix",
  "value": "openai/"
}
```

#### 10. ensure_suffix - 确保后缀

确保字符串字段以指定后缀结尾（已存在则不变）：

```json
{
  "path": "model",
  "mode": "ensure_suffix",
  "value": "-latest"
}
```

#### 11. trim_space - 去除首尾空白

对字符串字段执行 `TrimSpace`（空格、换行、制表符等都会被移除）：

```json
{
  "path": "model",
  "mode": "trim_space"
}
```

#### 12. to_lower - 转小写

将字符串字段转换为小写：

```json
{
  "path": "model",
  "mode": "to_lower"
}
```

#### 13. to_upper - 转大写

将字符串字段转换为大写：

```json
{
  "path": "model",
  "mode": "to_upper"
}
```

#### 14. replace - 字符串替换

对字符串字段执行子串替换：

```json
{
  "path": "model",
  "mode": "replace",
  "from": "openai/",
  "to": ""
}
```

**参数要求：**
- `from`: 必填且不能为空字符串
- `to`: 可选，省略时等同于空字符串

#### 15. regex_replace - 正则替换

对字符串字段执行正则匹配替换：

```json
{
  "path": "model",
  "mode": "regex_replace",
  "from": "^gpt-",
  "to": "openai/gpt-"
}
```

**参数要求：**
- `from`: 必填（正则表达式，Go regexp 语法）
- `to`: 可选，省略时等同于空字符串

### 条件判断

通过 `conditions` 数组设置操作执行的条件，仅当条件满足时才会执行对应操作。

#### 条件结构

```json
{
  "conditions": [
    {
      "path": "model",
      "mode": "contains",
      "value": "gpt-4",
      "invert": false,
      "pass_missing_key": false
    }
  ],
  "logic": "AND"
}
```

#### 条件匹配模式

- `full`: 完全匹配（默认）
- `prefix`: 前缀匹配
- `suffix`: 后缀匹配
- `contains`: 包含匹配
- `gt`: 大于（仅数字类型）
- `gte`: 大于等于（仅数字类型）
- `lt`: 小于（仅数字类型）
- `lte`: 小于等于（仅数字类型）

**须知：**
- 数值比较只能用于数字类型
- 字符串操作（prefix、suffix、contains）会将值转换为字符串进行比较

#### 条件参数说明

- `invert`: 反选功能，`true` 表示取反结果
- `pass_missing_key`: 当指定路径不存在时的行为
  - `true`: 路径不存在时条件通过
  - `false`: 路径不存在时条件不通过（默认）

#### 逻辑关系 (logic)

- `AND`: 所有条件都必须满足
- `OR`: 任意条件满足即可（默认）

### 路径语法

使用 JSON 路径语法访问嵌套字段：

- `temperature` - 根级字段
- `messages.0.content` - 数组第一个元素的 content 字段
- `messages.-1.content` - 数组最后一个元素的 content 字段
- `metadata.user.name` - 嵌套对象字段

同时，`path` 支持以下内置变量（无需在请求体中显式存在），可直接用于条件判断：

| 变量 | 含义 | 典型用途 |
| --- | --- | --- |
| `model` / `upstream_model` | 重定向后的目标模型 | 按实际调用的上游模型做条件匹配 |
| `original_model` | 重定向前的目标模型 | 按用户请求的原始模型做条件匹配 |

### 实用示例

#### 1. 动态调整模型参数

根据消息内容动态调整温度参数：

```json
{
  "operations": [
    {
      "path": "temperature",
      "mode": "set",
      "value": 0.3,
      "conditions": [
        {
          "path": "messages.0.content",
          "mode": "contains",
          "value": "代码"
        }
      ]
    },
    {
      "path": "temperature",
      "mode": "set",
      "value": 0.9,
      "conditions": [
        {
          "path": "messages.0.content",
          "mode": "contains",
          "value": "创意"
        }
      ]
    }
  ]
}
```

#### 2. 添加系统提示

在消息数组开头添加系统消息：

```json
{
  "operations": [
    {
      "path": "messages",
      "mode": "prepend",
      "value": [
        {
          "role": "system",
          "content": "你是一个专业的AI助手，请始终保持礼貌和专业。"
        }
      ]
    }
  ]
}
```

#### 3. 根据模型类型调整参数

根据不同模型设置不同的 max_tokens：

```json
{
  "operations": [
    {
      "path": "max_tokens",
      "mode": "set",
      "value": 4000,
      "conditions": [
        {
          "path": "model",
          "mode": "prefix",
          "value": "gpt-4"
        }
      ]
    },
    {
      "path": "max_tokens",
      "mode": "set",
      "value": 2000,
      "conditions": [
        {
          "path": "model",
          "mode": "prefix",
          "value": "gpt-3.5"
        }
      ]
    }
  ]
}
```

#### 4. 多条件组合（AND逻辑）

同时满足多个条件时才执行操作：

```json
{
  "operations": [
    {
      "path": "stream",
      "mode": "set",
      "value": false,
      "conditions": [
        {
          "path": "model",
          "mode": "contains",
          "value": "claude"
        },
        {
          "path": "messages.0.content",
          "mode": "contains",
          "value": "长文"
        }
      ],
      "logic": "AND"
    }
  ]
}
```

#### 5. 数值比较条件

根据数值大小进行条件判断：

```json
{
  "operations": [
    {
      "path": "temperature",
      "mode": "set",
      "value": 0.1,
      "conditions": [
        {
          "path": "max_tokens",
          "mode": "gt",
          "value": 1000
        }
      ]
    }
  ]
}
```

#### 6. 反选条件

使用 `invert` 实现反选逻辑：

```json
{
  "operations": [
    {
      "path": "stream",
      "mode": "set",
      "value": true,
      "conditions": [
        {
          "path": "model",
          "mode": "contains",
          "value": "gpt-3.5",
          "invert": true
        }
      ]
    }
  ]
}
```

#### 7. 处理缺失字段

使用 `pass_missing_key` 处理可能不存在的字段：

```json
{
  "operations": [
    {
      "path": "temperature",
      "mode": "set",
      "value": 0.7,
      "conditions": [
        {
          "path": "custom_field",
          "mode": "full",
          "value": "special",
          "pass_missing_key": true
        }
      ]
    }
  ]
}
```

#### 8. 字符串拼接示例

在用户消息后追加指导语：

```json
{
  "operations": [
    {
      "path": "messages.-1.content",
      "mode": "append",
      "value": "\n\n请详细解释你的思考过程。"
    }
  ]
}
```

### 注意事项

::: warning 执行顺序
操作按照在 `operations` 数组中的顺序依次执行，前面的操作会影响后续操作。
:::

---

## 多 Key 模式

### 轮询策略

| 模式 | 说明 | 适用场景 |
|------|------|----------|
| **顺序轮询** | 按 Key 列表顺序依次使用 | 需要均匀使用各 Key |
| **随机轮询** | 随机选择一个可用的 Key | 分散请求，避免单 Key 过载 |
| **权重轮询** | 按 Key 权重分配请求 | 不同 Key 配额不同时 |

### Key 状态管理

系统自动跟踪每个 Key 的状态：

| 状态 | 说明 |
|------|------|
| 🟢 **已启用** | Key 正常可用 |
| 🔴 **已禁用** | Key 因错误被禁用 |

在渠道详情中可以：

- 查看每个 Key 的禁用原因
- 手动重新启用被禁用的 Key
- 查看各 Key 的使用统计

---

## 标签管理

标签用于对渠道进行分类和筛选，便于管理大量渠道。

### 使用方式

1. 在渠道编辑页面设置 **标签** 字段
2. 支持添加多个标签（逗号分隔）
3. 在渠道列表中使用标签筛选

### 常见分类

| 标签类型 | 示例 |
|----------|------|
| **按用途** | `生产`、`测试`、`备用` |
| **按模型** | `gpt-4`、`claude`、`embedding` |
| **按提供商** | `openai`、`azure`、`anthropic` |
| **按地区** | `us`、`eu`、`cn` |

---

## 备注信息

备注字段用于记录渠道的相关管理信息：

- 渠道用途说明
- 到期时间提醒
- 特殊配置说明
- 联系人信息

::: info 字符限制
备注最大长度为 **255 个字符**。
:::

---

## 配置汇总

| 功能 | 配置位置 | 格式 |
|------|----------|------|
| 模型映射 | 渠道编辑 → 模型映射 | JSON |
| 状态码映射 | 渠道编辑 → 状态码映射 | JSON |
| 优先级 | 渠道编辑 → 优先级 | 数字 |
| 权重 | 渠道编辑 → 权重 | 数字 |
| 自动禁用 | 渠道编辑 → 自动禁用 | 开关 |
| 请求头覆盖 | 渠道编辑 → 请求头覆盖 | JSON |
| 参数覆盖 | 渠道编辑 → 参数覆盖 | JSON |
| 标签 | 渠道编辑 → 标签 | 文本（逗号分隔） |
| 备注 | 渠道编辑 → 备注 | 文本 |

---

**返回**：[渠道管理](./index)
