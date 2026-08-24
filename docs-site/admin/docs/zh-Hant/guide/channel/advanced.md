# 渠道高級設置

本文件介紹渠道管理中的高級配置選項，幫助管理員精細控制渠道行為、優化流量分配、實現靈活的模型管理。

## 快速導航

| 功能 | 說明 |
|------|------|
| [模型映射](#模型映射) | 重定向模型請求 |
| [狀態碼映射](#狀態碼映射) | 自定義錯誤處理 |
| [優先級與權重](#優先級與權重) | 流量分配策略 |
| [自動禁用](#自動禁用) | 故障自動處理 |
| [請求頭覆蓋](#請求頭覆蓋) | 自定義請求頭 |
| [參數覆蓋](#參數覆蓋) | 強制請求參數，支援條件判斷與動態調整 |
| [多 Key 模式](#多-key-模式) | 多金鑰輪詢與狀態管理 |
| [標籤管理](#標籤管理) | 渠道分類篩選 |

---

## 模型映射

模型映射允許將用戶請求的模型名稱映射到實際使用的模型，實現無感知的模型切換和統一管理。

### 使用場景

| 場景 | 示例 |
|------|------|
| **模型升級** | `gpt-4` → `gpt-4-turbo`，用戶無感知 |
| **模型替代** | `claude-3-opus` → `claude-3-5-sonnet` |
| **統一命名** | 將不同提供商的模型統一命名 |
| **成本優化** | 高價模型自動映射到同類型低價模型 |

### 配置格式

在渠道編輯頁面的 **模型映射** 字段中，以 JSON 格式配置：

```json
{
  "gpt-4": "gpt-4-turbo",
  "gpt-3.5-turbo": "gpt-3.5-turbo-16k",
  "claude-3-opus": "claude-3-5-sonnet-20241022"
}
```

::: tip 映射規則
- 左側（Key）：用戶請求的模型名稱
- 右側（Value）：實際發送給上游的模型名稱
- 未配置映射的模型將原樣傳遞
:::

### 進階用法

**多級映射**：支援鏈式映射，如 `gpt-4` → `gpt-4-turbo` → `gpt-4-turbo-preview`

---

## 狀態碼映射

狀態碼映射允許自定義上游 API 錯誤響應，將技術性錯誤轉換為用戶友好的提示資訊。

### 配置格式

```json
{
  "400": "請求參數錯誤，請檢查輸入格式",
  "401": "認證失敗，請檢查 API Key 是否有效",
  "403": "訪問被拒絕，請檢查賬戶權限",
  "429": "請求過於頻繁，請稍後重試",
  "500": "上游服務異常，正在處理中",
  "502": "網關錯誤，請聯繫管理員",
  "503": "服務暫時不可用，請稍後重試"
}
```

### 應用場景

::: code-group
```json [友好提示]
{
  "429": "當前請求量較大，請等待 30 秒後重試"
}
```

```json [靜默處理]
{
  "500": ""
}
```

```json [提示資訊]
{
  "401": "請前往設置頁面更新 API Key"
}
```
:::

---

## 優先級與權重

優先級和權重用於控制請求在不同渠道之間的路由策略。

### 優先級設置

優先級決定渠道的選擇順序：

| 設置 | 行為 |
|------|------|
| **數值越大** | 優先級越高，優先被選中 |
| **相同優先級** | 按權重分配請求 |
| **負數優先級** | 作為備用渠道，僅在必要時使用 |

**示例配置**：

| 渠道 | 優先級 | 說明 |
|------|:------:|------|
| 主渠道 | 10 | 正常情況下優先使用 |
| 備用渠道 A | 5 | 主渠道不可用時使用 |
| 備用渠道 B | 0 | 最後備選 |

### 權重設置

權重用於在相同優先級的渠道之間分配流量：

| 渠道 | 優先級 | 權重 | 流量分配 |
|------|:------:|:----:|:--------:|
| 渠道 A | 10 | 70 | 70% |
| 渠道 B | 10 | 30 | 30% |
| 渠道 C | 10 | 0 | 0%（不分配） |

::: warning 權重為 0
權重設置為 0 的渠道不會分配流量，但仍然可用於手動測試或特定場景調用。
:::

### 流量分配示例

**場景**：混合使用官方 API 和代理服務

```json
{
  "channels": [
    { "name": "OpenAI 官方", "priority": 10, "weight": 20 },
    { "name": "Azure GPT-4", "priority": 10, "weight": 50 },
    { "name": "代理服務", "priority": 5, "weight": 100 }
  ]
}
```

**結果**：
- 70% 流量由 Azure 處理（50/70）
- 30% 流量由 OpenAI 官方處理（20/70）
- 代理服務僅在上述渠道不可用時啟用

---

## 自動禁用

當渠道連續請求失敗時，系統會自動禁用該渠道，防止持續失敗影響用戶體驗。

### 配置選項

| 選項 | 預設值 | 說明 |
|------|:------:|------|
| **啟用自動禁用** | 開啓 | 是否在連續失敗後自動禁用 |
| **失敗閾值** | 5 次 | 連續失敗多少次後禁用 |
| **恢復間隔** | 300 秒 | 禁用多久後自動嘗試恢復 |

### 適用場景

| 場景 | 建議設置 |
|------|----------|
| **生產環境** | 開啓自動禁用，保障整體服務穩定性 |
| **測試渠道** | 關閉自動禁用，避免測試失敗影響服務 |
| **備用渠道** | 開啓自動禁用，但設置較低優先級 |

### 手動恢復

被自動禁用的渠道可以通過以下方式恢復：

1. **手動啟用**：在渠道詳情中點擊「啟用」
2. **自動恢復**：等待恢復間隔後系統自動嘗試

::: tip 最佳實踐
建議為關鍵模型配置多個渠道，開啓自動禁用功能，實現故障自動轉移。
:::

---

## 請求頭覆蓋

請求頭覆蓋允許自定義發送給上游 API 的 HTTP 請求頭，適用於特殊認證或代理場景。

### 配置格式

```json
{
  "X-Custom-Header": "custom-value",
  "X-Api-Version": "v2",
  "Authorization": "Bearer ${key}"
}
```

::: tip 變量替換
支援使用 `${key}` 佔位符，系統會自動替換為實際的 API Key。
:::

### 使用場景

**自定義認證**：
```json
{
  "X-API-Key": "your-custom-key",
  "X-Organization": "your-org-id"
}
```

**代理服務**：
```json
{
  "X-Proxy-Auth": "proxy-token",
  "X-Forwarded-For": "client-ip"
}
```

---

## 參數覆蓋

參數覆蓋允許強制設置或覆蓋請求參數，確保某些參數始終使用指定值。

::: warning 參數優先級
參數覆蓋的優先級高於用戶請求中的參數，即使用戶指定了不同的值，也會被覆蓋。
:::

參數覆蓋功能只能用於兼容合法上游接口格式、企業網絡兼容和請求規範化。

### 簡單覆蓋模式

向前兼容模式，直接指定要覆蓋的字段和值，系統會將這些字段合併到原始請求中：

```json
{
  "temperature": 0.8,
  "max_tokens": 2000,
  "model": "gpt-4"
}
```

### 高級操作模式

通過 `operations` 數組定義複雜的參數操作，支援條件判斷、數組操作、字符串拼接與字符串規範化等高級功能。

#### 基本結構

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

**字段說明（按需填寫）：**

- `mode`: 必填
- `path`: 適用於 `set` / `delete` / `append` / `prepend` / `trim_prefix` / `trim_suffix` / `ensure_prefix` / `ensure_suffix` / `trim_space` / `to_lower` / `to_upper` / `replace` / `regex_replace`
- `value`: 常見於 `set` / `append` / `prepend` / `trim_prefix` / `trim_suffix` / `ensure_prefix` / `ensure_suffix`
- `from` / `to`: 適用於 `move` / `copy` / `replace` / `regex_replace`
- `keep_origin`: 用於 `set`（已有值則跳過）以及對象合併時的 `append` / `prepend`

### 操作模式 (mode)

#### 1. set - 設置值

設置指定路徑的值：

```json
{
  "path": "temperature",
  "mode": "set",
  "value": 0.8,
  "keep_origin": false
}
```

**參數說明：**
- `keep_origin`: 為 `true` 時，如果目標路徑已存在值則跳過設置

#### 2. delete - 刪除字段

刪除指定路徑的字段：

```json
{
  "path": "messages.0",
  "mode": "delete"
}
```

#### 3. move - 移動字段

將一個字段的值移動到另一個位置：

```json
{
  "mode": "move",
  "from": "messages.0.content",
  "to": "system"
}
```

#### 4. append - 追加內容

在現有內容後追加新內容：

```json
{
  "path": "messages.0.content",
  "mode": "append",
  "value": "\n\n請用中文回答。"
}
```

**支援的數據類型：**
- **字符串**: 在原字符串末尾追加
- **數組**: 在數組末尾添加元素（支援添加單個元素或數組）
- **對象**: 合併對象屬性

#### 5. prepend - 前置內容

在現有內容前添加新內容：

```json
{
  "path": "messages.0.content",
  "mode": "prepend",
  "value": "重要提示：請仔細閱讀以下內容。\n\n"
}
```

**支援的數據類型：**
- **字符串**: 在原字符串開頭前置
- **數組**: 在數組開頭添加元素（支援添加單個元素或數組）
- **對象**: 合併對象屬性

#### 6. copy - 複製字段

將 `from` 指定路徑的值複製到 `to` 指定路徑（不刪除源字段）：

```json
{
  "mode": "copy",
  "from": "model",
  "to": "original_model"
}
```

#### 7. trim_prefix - 去除前綴

對字符串字段去除指定前綴（若不匹配則不變）：

```json
{
  "path": "model",
  "mode": "trim_prefix",
  "value": "openai/"
}
```

#### 8. trim_suffix - 去除後綴

對字符串字段去除指定後綴（若不匹配則不變）：

```json
{
  "path": "model",
  "mode": "trim_suffix",
  "value": "-latest"
}
```

#### 9. ensure_prefix - 確保前綴

確保字符串字段以指定前綴開頭（已存在則不變）：

```json
{
  "path": "model",
  "mode": "ensure_prefix",
  "value": "openai/"
}
```

#### 10. ensure_suffix - 確保後綴

確保字符串字段以指定後綴結尾（已存在則不變）：

```json
{
  "path": "model",
  "mode": "ensure_suffix",
  "value": "-latest"
}
```

#### 11. trim_space - 去除首尾空白

對字符串字段執行 `TrimSpace`（空格、換行、製表符等都會被移除）：

```json
{
  "path": "model",
  "mode": "trim_space"
}
```

#### 12. to_lower - 轉小寫

將字符串字段轉換為小寫：

```json
{
  "path": "model",
  "mode": "to_lower"
}
```

#### 13. to_upper - 轉大寫

將字符串字段轉換為大寫：

```json
{
  "path": "model",
  "mode": "to_upper"
}
```

#### 14. replace - 字符串替換

對字符串字段執行子串替換：

```json
{
  "path": "model",
  "mode": "replace",
  "from": "openai/",
  "to": ""
}
```

**參數要求：**
- `from`: 必填且不能為空字符串
- `to`: 可選，省略時等同於空字符串

#### 15. regex_replace - 正則替換

對字符串字段執行正則匹配替換：

```json
{
  "path": "model",
  "mode": "regex_replace",
  "from": "^gpt-",
  "to": "openai/gpt-"
}
```

**參數要求：**
- `from`: 必填（正則表達式，Go regexp 語法）
- `to`: 可選，省略時等同於空字符串

### 條件判斷

通過 `conditions` 數組設置操作執行的條件，僅當條件滿足時纔會執行對應操作。

#### 條件結構

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

#### 條件匹配模式

- `full`: 完全匹配（預設）
- `prefix`: 前綴匹配
- `suffix`: 後綴匹配
- `contains`: 包含匹配
- `gt`: 大於（僅數字類型）
- `gte`: 大於等於（僅數字類型）
- `lt`: 小於（僅數字類型）
- `lte`: 小於等於（僅數字類型）

**須知：**
- 數值比較只能用於數字類型
- 字符串操作（prefix、suffix、contains）會將值轉換為字符串進行比較

#### 條件參數說明

- `invert`: 反選功能，`true` 表示取反結果
- `pass_missing_key`: 當指定路徑不存在時的行為
  - `true`: 路徑不存在時條件通過
  - `false`: 路徑不存在時條件不通過（預設）

#### 邏輯關係 (logic)

- `AND`: 所有條件都必須滿足
- `OR`: 任意條件滿足即可（預設）

### 路徑語法

使用 JSON 路徑語法訪問嵌套字段：

- `temperature` - 根級字段
- `messages.0.content` - 數組第一個元素的 content 字段
- `messages.-1.content` - 數組最後一個元素的 content 字段
- `metadata.user.name` - 嵌套對象字段

同時，`path` 支援以下內置變量（無需在請求體中顯式存在），可直接用於條件判斷：

| 變量 | 含義 | 典型用途 |
| --- | --- | --- |
| `model` / `upstream_model` | 重定向後的目標模型 | 按實際調用的上游模型做條件匹配 |
| `original_model` | 重定向前的目標模型 | 按用戶請求的原始模型做條件匹配 |

### 實用示例

#### 1. 動態調整模型參數

根據消息內容動態調整溫度參數：

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
          "value": "代碼"
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
          "value": "創意"
        }
      ]
    }
  ]
}
```

#### 2. 添加系統提示

在消息數組開頭添加系統消息：

```json
{
  "operations": [
    {
      "path": "messages",
      "mode": "prepend",
      "value": [
        {
          "role": "system",
          "content": "你是一個專業的AI助手，請始終保持禮貌和專業。"
        }
      ]
    }
  ]
}
```

#### 3. 根據模型類型調整參數

根據不同模型設置不同的 max_tokens：

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

#### 4. 多條件組合（AND邏輯）

同時滿足多個條件時才執行操作：

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
          "value": "長文"
        }
      ],
      "logic": "AND"
    }
  ]
}
```

#### 5. 數值比較條件

根據數值大小進行條件判斷：

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

#### 6. 反選條件

使用 `invert` 實現反選邏輯：

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

#### 7. 處理缺失字段

使用 `pass_missing_key` 處理可能不存在的字段：

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

在用戶消息後追加指導語：

```json
{
  "operations": [
    {
      "path": "messages.-1.content",
      "mode": "append",
      "value": "\n\n請詳細解釋你的思考過程。"
    }
  ]
}
```

### 注意事項

::: warning 執行順序
操作按照在 `operations` 數組中的順序依次執行，前面的操作會影響後續操作。
:::

---

## 多 Key 模式

### 輪詢策略

| 模式 | 說明 | 適用場景 |
|------|------|----------|
| **順序輪詢** | 按 Key 列表順序依次使用 | 需要均勻使用各 Key |
| **隨機輪詢** | 隨機選擇一個可用的 Key | 分散請求，避免單 Key 過載 |
| **權重輪詢** | 按 Key 權重分配請求 | 不同 Key 配額不同時 |

### Key 狀態管理

系統自動跟蹤每個 Key 的狀態：

| 狀態 | 說明 |
|------|------|
| 🟢 **已啟用** | Key 正常可用 |
| 🔴 **已禁用** | Key 因錯誤被禁用 |

在渠道詳情中可以：

- 查看每個 Key 的禁用原因
- 手動重新啟用被禁用的 Key
- 查看各 Key 的使用統計

---

## 標籤管理

標籤用於對渠道進行分類和篩選，便於管理大量渠道。

### 使用方式

1. 在渠道編輯頁面設置 **標籤** 字段
2. 支援添加多個標籤（逗號分隔）
3. 在渠道列表中使用標籤篩選

### 常見分類

| 標籤類型 | 示例 |
|----------|------|
| **按用途** | `生產`、`測試`、`備用` |
| **按模型** | `gpt-4`、`claude`、`embedding` |
| **按提供商** | `openai`、`azure`、`anthropic` |
| **按地區** | `us`、`eu`、`cn` |

---

## 備註資訊

備註字段用於記錄渠道的相關管理資訊：

- 渠道用途說明
- 到期時間提醒
- 特殊配置說明
- 聯繫人資訊

::: info 字符限制
備註最大長度為 **255 個字符**。
:::

---

## 配置彙總

| 功能 | 配置位置 | 格式 |
|------|----------|------|
| 模型映射 | 渠道編輯 → 模型映射 | JSON |
| 狀態碼映射 | 渠道編輯 → 狀態碼映射 | JSON |
| 優先級 | 渠道編輯 → 優先級 | 數字 |
| 權重 | 渠道編輯 → 權重 | 數字 |
| 自動禁用 | 渠道編輯 → 自動禁用 | 開關 |
| 請求頭覆蓋 | 渠道編輯 → 請求頭覆蓋 | JSON |
| 參數覆蓋 | 渠道編輯 → 參數覆蓋 | JSON |
| 標籤 | 渠道編輯 → 標籤 | 文本（逗號分隔） |
| 備註 | 渠道編輯 → 備註 | 文本 |

---

**返回**：[渠道管理](./index)
