# Channel Advanced Settings

This document describes the advanced configuration options in channel management, helping admins control channel behavior precisely, optimize traffic distribution and manage models flexibly.

## Quick Navigation

| Feature | Description |
|------|------|
| [Model Mapping](#model-mapping) | Redirect model requests |
| [Status Code Mapping](#status-code-mapping) | Custom error handling |
| [Priority & Weight](#priority--weight) | Traffic distribution strategy |
| [Auto-disable](#auto-disable) | Automatic failure handling |
| [Request Header Override](#request-header-override) | Custom request headers |
| [Parameter Override](#parameter-override) | Force request parameters with conditions |
| [Multi-Key Mode](#multi-key-mode) | Key rotation and status management |
| [Tag Management](#tag-management) | Channel classification and filtering |

---

## Model Mapping

Model mapping lets you map the model name requested by the user to the model actually used, enabling seamless model switching and unified management.

### Use Cases

| Scenario | Example |
|------|------|
| **Model upgrade** | `gpt-4` → `gpt-4-turbo`, transparent to users |
| **Model replacement** | `claude-3-opus` → `claude-3-5-sonnet` |
| **Unified naming** | Unify model names across providers |
| **Cost optimization** | Map expensive models to cheaper equivalents |

### Configuration Format

Configure it in the **Model Mapping** field of the channel edit page, in JSON format:

```json
{
  "gpt-4": "gpt-4-turbo",
  "gpt-3.5-turbo": "gpt-3.5-turbo-16k",
  "claude-3-opus": "claude-3-5-sonnet-20241022"
}
```

::: tip Mapping rules
- Left (Key): the model name requested by the user
- Right (Value): the model name actually sent upstream
- Unmapped models pass through unchanged
:::

### Advanced Usage

**Multi-level mapping**: chained mapping is supported, e.g. `gpt-4` → `gpt-4-turbo` → `gpt-4-turbo-preview`

---

## Status Code Mapping

Status code mapping lets you customize upstream API error responses, converting technical errors into user-friendly messages.

### Configuration Format

```json
{
  "400": "Invalid request parameters, please check your input",
  "401": "Authentication failed, please check whether the API Key is valid",
  "403": "Access denied, please check your account permissions",
  "429": "Too many requests, please retry later",
  "500": "Upstream service error, processing now",
  "502": "Gateway error, please contact the administrator",
  "503": "Service temporarily unavailable, please retry later"
}
```

### Use Cases

::: code-group
```json [Friendly message]
{
  "429": "Request volume is high right now, please retry in 30 seconds"
}
```

```json [Silent handling]
{
  "500": ""
}
```

```json [Notice]
{
  "401": "Please go to the settings page and update your API Key"
}
```
:::

---

## Priority & Weight

Priority and weight control how requests are routed between different channels.

### Priority

Priority determines the order in which channels are selected:

| Setting | Behavior |
|------|------|
| **Higher value** | Higher priority, selected first |
| **Same priority** | Requests distributed by weight |
| **Negative priority** | Standby channel, used only when necessary |

**Example configuration**:

| Channel | Priority | Description |
|------|:------:|------|
| Primary channel | 10 | Used preferentially under normal conditions |
| Standby channel A | 5 | Used when the primary channel is unavailable |
| Standby channel B | 0 | Last resort |

### Weight

Weight distributes traffic among channels with the same priority:

| Channel | Priority | Weight | Traffic share |
|------|:------:|:----:|:--------:|
| Channel A | 10 | 70 | 70% |
| Channel B | 10 | 30 | 30% |
| Channel C | 10 | 0 | 0% (none) |

::: warning Zero weight
A channel with weight 0 receives no traffic, but can still be used for manual testing or specific scenarios.
:::

### Traffic Distribution Example

**Scenario**: mixing official APIs with proxy services

```json
{
  "channels": [
    { "name": "OpenAI Official", "priority": 10, "weight": 20 },
    { "name": "Azure GPT-4", "priority": 10, "weight": 50 },
    { "name": "Proxy service", "priority": 5, "weight": 100 }
  ]
}
```

**Result**:
- 70% of traffic is handled by Azure (50/70)
- 30% of traffic is handled by OpenAI Official (20/70)
- The proxy service is only used when the above channels are unavailable

---

## Auto-disable

When a channel fails consecutively, the system automatically disables it to prevent repeated failures from hurting the user experience.

### Configuration Options

| Option | Default | Description |
|------|:------:|------|
| **Enable auto-disable** | On | Whether to auto-disable after consecutive failures |
| **Failure threshold** | 5 | How many consecutive failures trigger disabling |
| **Recovery interval** | 300 s | How long to wait before automatically retrying recovery |

### Use Cases

| Scenario | Recommended setting |
|------|----------|
| **Production** | Enable auto-disable to keep the whole service stable |
| **Test channels** | Disable auto-disable so test failures don't affect the service |
| **Standby channels** | Enable auto-disable but set a lower priority |

### Manual Recovery

An auto-disabled channel can be recovered by:

1. **Manually enabling it**: click **Enable** in the channel details
2. **Automatic recovery**: the system retries after the recovery interval

::: tip Best practice
Configure multiple channels for critical models and enable auto-disable to achieve automatic failover.
:::

---

## Request Header Override

Request header override lets you customize the HTTP headers sent to the upstream API, useful for special authentication or proxy scenarios.

### Configuration Format

```json
{
  "X-Custom-Header": "custom-value",
  "X-Api-Version": "v2",
  "Authorization": "Bearer ${key}"
}
```

::: tip Variable substitution
The `${key}` placeholder is supported — the system replaces it with the actual API Key automatically.
:::

### Use Cases

**Custom authentication**:
```json
{
  "X-API-Key": "your-custom-key",
  "X-Organization": "your-org-id"
}
```

**Proxy service**:
```json
{
  "X-Proxy-Auth": "proxy-token",
  "X-Forwarded-For": "client-ip"
}
```

---

## Parameter Override

Parameter override lets you force-set or override request parameters, ensuring certain parameters always use specified values.

::: warning Parameter precedence
Parameter overrides take precedence over parameters in the user's request — even if the user specifies a different value, it will be overridden.
:::

Parameter override may only be used to stay compatible with legitimate upstream interface formats, for enterprise network compatibility, and for request normalization.

### Simple Override Mode

The backward-compatible mode. Specify the fields and values to override directly; the system merges them into the original request:

```json
{
  "temperature": 0.8,
  "max_tokens": 2000,
  "model": "gpt-4"
}
```

### Advanced Operation Mode

The `operations` array defines complex parameter operations, supporting condition checks, array operations, string concatenation and string normalization.

#### Basic Structure

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

**Field reference (fill in as needed):**

- `mode`: required
- `path`: applies to `set` / `delete` / `append` / `prepend` / `trim_prefix` / `trim_suffix` / `ensure_prefix` / `ensure_suffix` / `trim_space` / `to_lower` / `to_upper` / `replace` / `regex_replace`
- `value`: commonly used with `set` / `append` / `prepend` / `trim_prefix` / `trim_suffix` / `ensure_prefix` / `ensure_suffix`
- `from` / `to`: apply to `move` / `copy` / `replace` / `regex_replace`
- `keep_origin`: used by `set` (skip if a value already exists) and by `append` / `prepend` when merging objects

### Operation Modes

#### 1. set

Set the value at the specified path:

```json
{
  "path": "temperature",
  "mode": "set",
  "value": 0.8,
  "keep_origin": false
}
```

**Parameters:**
- `keep_origin`: when `true`, skips setting if a value already exists at the target path

#### 2. delete

Delete the field at the specified path:

```json
{
  "path": "messages.0",
  "mode": "delete"
}
```

#### 3. move

Move a field's value to another location:

```json
{
  "mode": "move",
  "from": "messages.0.content",
  "to": "system"
}
```

#### 4. append

Append new content after existing content:

```json
{
  "path": "messages.0.content",
  "mode": "append",
  "value": "\n\nPlease answer in Chinese."
}
```

**Supported data types:**
- **String**: appends to the end of the original string
- **Array**: adds elements to the end of the array (single element or array)
- **Object**: merges object properties

#### 5. prepend

Prepend new content before existing content:

```json
{
  "path": "messages.0.content",
  "mode": "prepend",
  "value": "Important: please read the following carefully.\n\n"
}
```

**Supported data types:**
- **String**: prepends to the beginning of the original string
- **Array**: adds elements to the beginning of the array (single element or array)
- **Object**: merges object properties

#### 6. copy

Copy the value at the `from` path to the `to` path (the source field is not deleted):

```json
{
  "mode": "copy",
  "from": "model",
  "to": "original_model"
}
```

#### 7. trim_prefix

Remove a specified prefix from a string field (unchanged if it doesn't match):

```json
{
  "path": "model",
  "mode": "trim_prefix",
  "value": "openai/"
}
```

#### 8. trim_suffix

Remove a specified suffix from a string field (unchanged if it doesn't match):

```json
{
  "path": "model",
  "mode": "trim_suffix",
  "value": "-latest"
}
```

#### 9. ensure_prefix

Ensure a string field starts with the specified prefix (unchanged if it already does):

```json
{
  "path": "model",
  "mode": "ensure_prefix",
  "value": "openai/"
}
```

#### 10. ensure_suffix

Ensure a string field ends with the specified suffix (unchanged if it already does):

```json
{
  "path": "model",
  "mode": "ensure_suffix",
  "value": "-latest"
}
```

#### 11. trim_space

Trim leading and trailing whitespace from a string field (spaces, newlines, tabs, etc.):

```json
{
  "path": "model",
  "mode": "trim_space"
}
```

#### 12. to_lower

Convert a string field to lowercase:

```json
{
  "path": "model",
  "mode": "to_lower"
}
```

#### 13. to_upper

Convert a string field to uppercase:

```json
{
  "path": "model",
  "mode": "to_upper"
}
```

#### 14. replace

Perform substring replacement on a string field:

```json
{
  "path": "model",
  "mode": "replace",
  "from": "openai/",
  "to": ""
}
```

**Parameter requirements:**
- `from`: required and must not be an empty string
- `to`: optional; omitted is equivalent to an empty string

#### 15. regex_replace

Perform regular-expression replacement on a string field:

```json
{
  "path": "model",
  "mode": "regex_replace",
  "from": "^gpt-",
  "to": "openai/gpt-"
}
```

**Parameter requirements:**
- `from`: required (regular expression, Go regexp syntax)
- `to`: optional; omitted is equivalent to an empty string

### Conditions

The `conditions` array sets the conditions under which an operation executes — the operation only runs when the conditions are met.

#### Condition Structure

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

#### Condition Match Modes

- `full`: exact match (default)
- `prefix`: prefix match
- `suffix`: suffix match
- `contains`: substring match
- `gt`: greater than (numeric only)
- `gte`: greater than or equal (numeric only)
- `lt`: less than (numeric only)
- `lte`: less than or equal (numeric only)

**Notes:**
- Numeric comparisons only work with numeric types
- String operations (prefix, suffix, contains) convert values to strings before comparing

#### Condition Parameters

- `invert`: invert the result; `true` negates the outcome
- `pass_missing_key`: behavior when the specified path does not exist
  - `true`: the condition passes when the path is missing
  - `false`: the condition fails when the path is missing (default)

#### Logic

- `AND`: all conditions must be met
- `OR`: any condition met is enough (default)

### Path Syntax

JSON path syntax is used to access nested fields:

- `temperature` - root-level field
- `messages.0.content` - the `content` field of the first array element
- `messages.-1.content` - the `content` field of the last array element
- `metadata.user.name` - nested object field

In addition, `path` supports the following built-in variables (they don't need to exist in the request body) for use in conditions:

| Variable | Meaning | Typical use |
| --- | --- | --- |
| `model` / `upstream_model` | The target model after redirection | Match against the actual upstream model |
| `original_model` | The target model before redirection | Match against the model the user originally requested |

### Practical Examples

#### 1. Dynamically Adjusting Model Parameters

Adjust the temperature based on message content:

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
          "value": "code"
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
          "value": "creative"
        }
      ]
    }
  ]
}
```

#### 2. Adding a System Prompt

Prepend a system message to the messages array:

```json
{
  "operations": [
    {
      "path": "messages",
      "mode": "prepend",
      "value": [
        {
          "role": "system",
          "content": "You are a professional AI assistant. Please always remain polite and professional."
        }
      ]
    }
  ]
}
```

#### 3. Adjusting Parameters by Model Type

Set different max_tokens for different models:

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

#### 4. Combining Conditions (AND Logic)

Execute an operation only when multiple conditions are met:

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
          "value": "long text"
        }
      ],
      "logic": "AND"
    }
  ]
}
```

#### 5. Numeric Comparison Conditions

Evaluate conditions based on numeric values:

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

#### 6. Inverted Conditions

Use `invert` to negate a condition:

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

#### 7. Handling Missing Fields

Use `pass_missing_key` to handle fields that may not exist:

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

#### 8. String Concatenation

Append instructions to the user's last message:

```json
{
  "operations": [
    {
      "path": "messages.-1.content",
      "mode": "append",
      "value": "\n\nPlease explain your thinking process in detail."
    }
  ]
}
```

### Notes

::: warning Execution order
Operations execute in the order they appear in the `operations` array; earlier operations can affect later ones.
:::

---

## Multi-Key Mode

### Rotation Strategies

| Mode | Description | Use case |
|------|------|----------|
| **Sequential rotation** | Uses Keys in list order | Even usage across Keys |
| **Random rotation** | Picks a random available Key | Spread requests, avoid overloading one Key |
| **Weighted rotation** | Distributes by Key weight | When Keys have different quotas |

### Key Status Management

The system tracks each Key's status automatically:

| Status | Description |
|------|------|
| 🟢 **Enabled** | Key is healthy |
| 🔴 **Disabled** | Key disabled due to errors |

In the channel details you can:

- View the disable reason for each Key
- Manually re-enable disabled Keys
- View usage statistics for each Key

---

## Tag Management

Tags classify and filter channels, making it easier to manage many of them.

### How to Use

1. Set the **Tag** field on the channel edit page
2. Multiple tags are supported (comma-separated)
3. Filter channels by tag in the list

### Common Tag Categories

| Tag type | Examples |
|----------|------|
| **By purpose** | `production`, `test`, `standby` |
| **By model** | `gpt-4`, `claude`, `embedding` |
| **By provider** | `openai`, `azure`, `anthropic` |
| **By region** | `us`, `eu`, `cn` |

---

## Notes Field

The notes field records management information about the channel:

- Channel purpose
- Expiry reminders
- Special configuration notes
- Contact information

::: info Character limit
Notes are limited to **255 characters**.
:::

---

## Configuration Summary

| Feature | Configuration location | Format |
|------|----------|------|
| Model mapping | Channel edit → Model mapping | JSON |
| Status code mapping | Channel edit → Status code mapping | JSON |
| Priority | Channel edit → Priority | Number |
| Weight | Channel edit → Weight | Number |
| Auto-disable | Channel edit → Auto-disable | Toggle |
| Request header override | Channel edit → Request header override | JSON |
| Parameter override | Channel edit → Parameter override | JSON |
| Tags | Channel edit → Tags | Text (comma-separated) |
| Notes | Channel edit → Notes | Text |

---

**Back to**: [Channel Management](./index)
