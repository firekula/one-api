# One API 管理 API 参考文档

> 本文档涵盖 One API 的所有管理 API 和中继 API 接口。

## 鉴权方式

One API 支持两种鉴权方式：

### 1. Cookie 鉴权

浏览器登录后自动携带 Session Cookie，适用于 Web UI 访问场景。

### 2. Access Token 鉴权

通过 `Authorization` 请求头传递 Access Token，适用于编程调用场景：

```
Authorization: Bearer <user_access_token>
```

**获取方式**：登录 Web UI -> 个人设置 -> 生成系统访问令牌（Access Token）。

### 3. Token 鉴权（用于中继 API）

中继 API 使用独立的 Token 鉴权机制，通过请求头传递 `sk-` 开头的令牌：

```
Authorization: Bearer sk-<token_key>
```

该令牌在 Web UI -> 令牌页面进行管理。

### 权限等级

| 角色 | 说明 |
|------|------|
| 普通用户 (RoleCommonUser) | 基础用户权限 |
| 管理员 (RoleAdminUser) | 可管理用户、渠道、兑换码等 |
| 超级管理员 (RoleRootUser) | 可管理系统选项等最高权限操作 |

## 通用响应格式

### 成功响应

```json
{
  "success": true,
  "message": "操作描述",
  "data": {}
}
```

### 失败响应

```json
{
  "success": false,
  "message": "错误描述",
  "data": null
}
```

---

## API 列表

所有 API 路由均挂载在 `/api` 前缀下，全局应用 Gzip 压缩和速率限制（`GlobalAPIRateLimit`）中间件。

### 1. 公共接口

无需鉴权的公开接口。

| 方法 | 路径 | 鉴权 | 说明 |
|------|------|------|------|
| GET | /api/status | — | 获取系统状态 |
| GET | /api/notice | — | 获取系统公告 |
| GET | /api/about | — | 获取关于页面信息 |
| GET | /api/home_page_content | — | 获取首页内容 |
| GET | /api/models | UserAuth | 获取模型列表 |

---

### 2. 认证与注册

| 方法 | 路径 | 鉴权 | 说明 |
|------|------|------|------|
| POST | /api/user/register | Turnstile + RateLimit | 用户注册 |
| POST | /api/user/login | RateLimit | 用户登录 |
| GET | /api/user/logout | — | 退出登录 |
| GET | /api/verification | Turnstile + RateLimit | 发送邮箱验证码 |
| GET | /api/reset_password | Turnstile + RateLimit | 发送密码重置邮件 |
| POST | /api/user/reset | RateLimit | 重置密码 |
| GET | /api/oauth/github | RateLimit | GitHub OAuth 登录 |
| GET | /api/oauth/oidc | RateLimit | OIDC 登录 |
| GET | /api/oauth/lark | RateLimit | 飞书登录 |
| GET | /api/oauth/state | RateLimit | 生成 OAuth State |
| GET | /api/oauth/wechat | RateLimit | 微信登录 |
| GET | /api/oauth/wechat/bind | UserAuth + RateLimit | 绑定微信账号 |
| GET | /api/oauth/email/bind | UserAuth + RateLimit | 绑定邮箱 |

> 注：Turnstile 指 Cloudflare Turnstile 人机验证。当系统开启 Turnstile 验证时，相关接口需要携带验证 token。

#### 2.1 用户注册

**POST** `/api/user/register`

请求体：

```json
{
  "username": "newuser",
  "password": "password123",
  "display_name": "新用户",
  "email": "user@example.com",
  "verification_code": "123456",
  "aff_code": "邀请码（可选）"
}
```

响应：

```json
{
  "success": true,
  "message": ""
}
```

#### 2.2 用户登录

**POST** `/api/user/login`

请求体：

```json
{
  "username": "root",
  "password": "123456"
}
```

响应：

```json
{
  "success": true,
  "message": "",
  "data": {
    "id": 1,
    "username": "root",
    "display_name": "root",
    "role": 100,
    "status": 1
  }
}
```

登录成功后，服务端会设置 Session Cookie。

#### 2.3 发送邮箱验证码

**GET** `/api/verification?email=user@example.com`

响应：

```json
{
  "success": true,
  "message": "验证码已发送至邮箱"
}
```

---

### 3. 用户管理

#### 3.1 当前用户（需 UserAuth）

| 方法 | 路径 | 鉴权 | 说明 |
|------|------|------|------|
| GET | /api/user/self | UserAuth | 获取当前用户信息 |
| PUT | /api/user/self | UserAuth | 更新当前用户信息 |
| DELETE | /api/user/self | UserAuth | 删除当前账户 |
| GET | /api/user/dashboard | UserAuth | 获取仪表板数据（近7天按日统计） |
| GET | /api/user/token | UserAuth | 生成系统访问令牌 |
| GET | /api/user/aff | UserAuth | 获取邀请码 |
| POST | /api/user/topup | UserAuth | 使用兑换码充值 |
| GET | /api/user/available_models | UserAuth | 获取当前用户可用模型列表 |

##### 获取当前用户信息

**GET** `/api/user/self`

响应：

```json
{
  "success": true,
  "message": "",
  "data": {
    "id": 1,
    "username": "root",
    "display_name": "root",
    "role": 100,
    "status": 1,
    "email": "",
    "github_id": "",
    "wechat_id": "",
    "lark_id": "",
    "oidc_id": "",
    "quota": 1000000,
    "used_quota": 50000,
    "request_count": 123,
    "group": "default",
    "aff_code": "ABCDEF",
    "inviter_id": 0
  }
}
```

##### 更新当前用户信息

**PUT** `/api/user/self`

请求体：

```json
{
  "display_name": "新显示名称",
  "password": "newpassword123"
}
```

响应：

```json
{
  "success": true,
  "message": ""
}
```

##### 使用兑换码充值

**POST** `/api/user/topup`

请求体：

```json
{
  "key": "兑换码字符串"
}
```

响应：

```json
{
  "success": true,
  "message": "",
  "data": 500000
}
```

> `data` 字段返回充值后的额度数量。

##### 生成 Access Token

**GET** `/api/user/token`

响应：

```json
{
  "success": true,
  "message": "",
  "data": "abcdef1234567890abcdef1234567890"
}
```

注意：每次调用此接口都会重新生成 Access Token，旧的 Token 将失效。

#### 3.2 用户管理（需 AdminAuth）

| 方法 | 路径 | 鉴权 | 说明 |
|------|------|------|------|
| GET | /api/user/ | AdminAuth | 获取所有用户列表（query: p, order） |
| GET | /api/user/search | AdminAuth | 搜索用户（query: keyword） |
| GET | /api/user/:id | AdminAuth | 获取指定用户详情 |
| POST | /api/user/ | AdminAuth | 创建新用户 |
| PUT | /api/user/ | AdminAuth | 编辑用户 |
| POST | /api/user/manage | AdminAuth | 管理用户（启用/禁用/删除） |
| DELETE | /api/user/:id | AdminAuth | 删除用户 |
| POST | /api/topup | AdminAuth | 管理员手动为用户充值 |

##### 获取用户列表

**GET** `/api/user/?p=0&order=id_desc`

查询参数：

| 参数 | 类型 | 说明 |
|------|------|------|
| p | int | 分页页码，从 0 开始 |
| order | string | 排序方式，如 `id_desc` |

响应：

```json
{
  "success": true,
  "message": "",
  "data": [
    {
      "id": 1,
      "username": "root",
      "display_name": "root",
      "role": 100,
      "status": 1,
      "quota": 1000000,
      "used_quota": 50000,
      "group": "default"
    }
  ]
}
```

##### 创建用户

**POST** `/api/user/`

请求体：

```json
{
  "username": "newuser",
  "password": "password123",
  "display_name": "新用户"
}
```

响应：

```json
{
  "success": true,
  "message": ""
}
```

##### 管理用户（启用/禁用/删除）

**POST** `/api/user/manage`

请求体：

```json
{
  "username": "target_user",
  "action": "disable"
}
```

`action` 可选值：`enable`（启用）、`disable`（禁用）、`delete`（删除）。

响应：

```json
{
  "success": true,
  "message": ""
}
```

##### 管理员充值

**POST** `/api/topup`

请求体：

```json
{
  "user_id": 1,
  "quota": 100000,
  "remark": "充值 100000 额度"
}
```

响应：

```json
{
  "success": true,
  "message": ""
}
```

---

### 4. 渠道管理（需 AdminAuth）

| 方法 | 路径 | 鉴权 | 说明 |
|------|------|------|------|
| GET | /api/channel/ | AdminAuth | 获取所有渠道列表（query: p, order） |
| GET | /api/channel/search | AdminAuth | 搜索渠道（query: keyword） |
| GET | /api/channel/models | AdminAuth | 列出所有渠道的模型 |
| GET | /api/channel/:id | AdminAuth | 获取指定渠道详情 |
| GET | /api/channel/test | AdminAuth | 测试所有渠道 |
| GET | /api/channel/test/:id | AdminAuth | 测试指定渠道 |
| GET | /api/channel/update_balance | AdminAuth | 更新所有渠道余额 |
| GET | /api/channel/update_balance/:id | AdminAuth | 更新指定渠道余额 |
| POST | /api/channel/ | AdminAuth | 添加渠道 |
| PUT | /api/channel/ | AdminAuth | 编辑渠道 |
| DELETE | /api/channel/disabled | AdminAuth | 删除所有禁用渠道 |
| DELETE | /api/channel/:id | AdminAuth | 删除指定渠道 |

#### 4.1 创建渠道

**POST** `/api/channel/`

请求体：

```json
{
  "type": 1,
  "key": "sk-xxxxxxxxxxxx",
  "name": "我的渠道",
  "models": "gpt-3.5-turbo,gpt-4",
  "base_url": "https://api.openai.com",
  "group": "default",
  "weight": 1,
  "model_mapping": "",
  "priority": 0,
  "system_prompt": ""
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| type | int | 渠道类型（如 1=OpenAI, 3=Azure 等） |
| key | string | API Key，多行以 `\n` 分隔可批量创建 |
| name | string | 渠道名称 |
| models | string | 支持模型列表，逗号分隔 |
| base_url | string | API 地址 |
| group | string | 渠道分组，默认 `default` |
| weight | uint | 负载均衡权重 |
| model_mapping | string | 模型映射配置 |
| priority | int64 | 优先级 |

响应：

```json
{
  "success": true,
  "message": ""
}
```

#### 4.2 测试指定渠道

**GET** `/api/channel/test/:id`

响应：

```json
{
  "success": true,
  "message": ""
}
```

#### 4.3 更新渠道余额

**GET** `/api/channel/update_balance/:id`

响应：

```json
{
  "success": true,
  "message": "",
  "data": {
    "balance": 12.5,
    "balance_updated_time": 1700000000
  }
}
```

---

### 5. 令牌管理（需 UserAuth）

| 方法 | 路径 | 鉴权 | 说明 |
|------|------|------|------|
| GET | /api/token/ | UserAuth | 获取当前用户所有令牌（query: p, order） |
| GET | /api/token/search | UserAuth | 搜索令牌（query: keyword） |
| GET | /api/token/:id | UserAuth | 获取指定令牌详情 |
| POST | /api/token/ | UserAuth | 创建新令牌 |
| PUT | /api/token/ | UserAuth | 编辑令牌 |
| DELETE | /api/token/:id | UserAuth | 删除令牌 |

#### 5.1 创建令牌

**POST** `/api/token/`

请求体：

```json
{
  "name": "我的应用令牌",
  "remain_quota": 500000,
  "unlimited_quota": false,
  "expired_time": -1,
  "models": "",
  "subnet": ""
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| name | string | 令牌名称 |
| remain_quota | int64 | 剩余配额，`unlimited_quota=true` 时忽略 |
| unlimited_quota | bool | 是否无限制配额 |
| expired_time | int64 | 过期时间戳，-1 表示永不过期 |
| models | string | 允许使用的模型，为空表示不限制 |
| subnet | string | 允许使用的网段，为空表示不限制 |

响应：

```json
{
  "success": true,
  "message": "",
  "data": {
    "id": 1,
    "user_id": 1,
    "key": "sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
    "name": "我的应用令牌",
    "status": 1,
    "created_time": 1700000000,
    "accessed_time": 1700000000,
    "expired_time": -1,
    "remain_quota": 500000,
    "unlimited_quota": false,
    "used_quota": 0,
    "models": "",
    "subnet": ""
  }
}
```

#### 5.2 获取令牌列表

**GET** `/api/token/?p=0&order=id_desc`

响应：

```json
{
  "success": true,
  "message": "",
  "data": [
    {
      "id": 1,
      "user_id": 1,
      "key": "sk-xxxxxxxxxxxxxxxx",
      "name": "我的应用令牌",
      "status": 1,
      "expired_time": -1,
      "remain_quota": 500000,
      "unlimited_quota": false,
      "used_quota": 10000
    }
  ]
}
```

---

### 6. 兑换码管理（需 AdminAuth）

| 方法 | 路径 | 鉴权 | 说明 |
|------|------|------|------|
| GET | /api/redemption/ | AdminAuth | 获取所有兑换码列表（query: p, order） |
| GET | /api/redemption/search | AdminAuth | 搜索兑换码（query: keyword） |
| GET | /api/redemption/:id | AdminAuth | 获取指定兑换码详情 |
| POST | /api/redemption/ | AdminAuth | 创建兑换码 |
| PUT | /api/redemption/ | AdminAuth | 编辑兑换码 |
| DELETE | /api/redemption/:id | AdminAuth | 删除兑换码 |

#### 6.1 创建兑换码

**POST** `/api/redemption/`

请求体：

```json
{
  "name": "新用户优惠",
  "quota": 100000,
  "count": 10
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| name | string | 兑换码名称，长度 1-20 |
| quota | int64 | 每个兑换码的额度 |
| count | int | 批量生成数量，1-100 |

响应：

```json
{
  "success": true,
  "message": "",
  "data": [
    "uuid1xxxxxxxxxxxx",
    "uuid2xxxxxxxxxxxx"
  ]
}
```

> `data` 字段返回批量生成的兑换码字符串数组。

---

### 7. 日志

| 方法 | 路径 | 鉴权 | 说明 |
|------|------|------|------|
| GET | /api/log/ | AdminAuth | 获取所有日志列表（query: p, order） |
| DELETE | /api/log/ | AdminAuth | 删除历史日志（query: timestamp） |
| GET | /api/log/stat | AdminAuth | 获取日志统计信息 |
| GET | /api/log/search | AdminAuth | 搜索日志（query: keyword） |
| GET | /api/log/self | UserAuth | 获取当前用户日志（query: p, order） |
| GET | /api/log/self/stat | UserAuth | 获取当前用户日志统计 |
| GET | /api/log/self/search | UserAuth | 搜索当前用户日志（query: keyword） |

#### 7.1 获取日志列表

**GET** `/api/log/?p=0&order=id_desc`

响应：

```json
{
  "success": true,
  "message": "",
  "data": [
    {
      "id": 1,
      "user_id": 1,
      "username": "root",
      "channel_id": 1,
      "channel_name": "我的渠道",
      "model": "gpt-3.5-turbo",
      "prompt_tokens": 100,
      "completion_tokens": 50,
      "quota": 150,
      "created_time": 1700000000,
      "content": ""
    }
  ]
}
```

#### 7.2 删除历史日志

**DELETE** `/api/log/?timestamp=1700000000`

查询参数 `timestamp`：删除该时间戳之前的所有日志。

响应：

```json
{
  "success": true,
  "message": ""
}
```

#### 7.3 获取日志统计

**GET** `/api/log/stat`

响应：

```json
{
  "success": true,
  "message": "",
  "data": {
    "count": 1000,
    "quota": 500000
  }
}
```

---

### 8. 系统选项（需 RootAuth）

| 方法 | 路径 | 鉴权 | 说明 |
|------|------|------|------|
| GET | /api/option/ | RootAuth | 获取所有系统选项 |
| PUT | /api/option/ | RootAuth | 更新系统选项 |

#### 8.1 获取系统选项

**GET** `/api/option/`

响应：

```json
{
  "success": true,
  "message": "",
  "data": [
    {
      "key": "PasswordLoginEnabled",
      "value": "true"
    },
    {
      "key": "PasswordRegisterEnabled",
      "value": "true"
    },
    {
      "key": "EmailVerificationEnabled",
      "value": "false"
    },
    {
      "key": "GitHubOAuthEnabled",
      "value": "false"
    },
    {
      "key": "WeChatAuthEnabled",
      "value": "false"
    }
  ]
}
```

> 注意：以 `Token` 或 `Secret` 结尾的敏感选项不会返回。

#### 8.2 更新系统选项

**PUT** `/api/option/`

请求体：

```json
{
  "key": "PasswordRegisterEnabled",
  "value": "false"
}
```

响应：

```json
{
  "success": true,
  "message": ""
}
```

---

### 9. 分组管理（需 AdminAuth）

| 方法 | 路径 | 鉴权 | 说明 |
|------|------|------|------|
| GET | /api/group/ | AdminAuth | 获取所有分组 |

#### 9.1 获取分组列表

**GET** `/api/group/`

响应：

```json
{
  "success": true,
  "message": "",
  "data": [
    "default",
    "vip",
    "svip"
  ]
}
```

---

### 10. 模型列表

| 方法 | 路径 | 鉴权 | 说明 |
|------|------|------|------|
| GET | /api/models | UserAuth | 获取模型列表（仪表板模式） |

#### 10.1 获取模型列表

**GET** `/api/models`

响应：

```json
{
  "success": true,
  "message": "",
  "data": [
    "gpt-3.5-turbo",
    "gpt-4",
    "gpt-4-turbo"
  ]
}
```

---

### 11. 系统状态

| 方法 | 路径 | 鉴权 | 说明 |
|------|------|------|------|
| GET | /api/status | — | 获取系统状态信息 |

#### 11.1 系统状态

**GET** `/api/status`

响应：

```json
{
  "success": true,
  "message": "",
  "data": {
    "version": "v0.6.0",
    "user_count": 100,
    "channel_count": 5,
    "token_count": 50,
    "quota": 10000000
  }
}
```

---

## 中继 API（Relay）

中继 API 路由实现 OpenAI 兼容接口的转发。使用 **Token 鉴权**，请参考上文"鉴权方式"章节。

鉴权头格式：

```
Authorization: Bearer sk-<token_key>
```

### 公开模型查询接口

这些路由仅需 TokenAuth，无需分发中间件。

| 方法 | 路径 | 鉴权 | 说明 | 状态 |
|------|------|------|------|------|
| GET | /v1/models | TokenAuth | 列出所有可用模型 | 已实现 |
| GET | /v1/models/:model | TokenAuth | 获取指定模型信息 | 已实现 |

### 中继接口

以下路由应用中间件链：`RelayPanicRecover` -> `TokenAuth` -> `Distribute`。

#### 已实现的接口

| 方法 | 路径 | 说明 | 状态 |
|------|------|------|------|
| POST | /v1/chat/completions | 聊天补全 | 已实现 |
| POST | /v1/completions | 文本补全 | 已实现 |
| POST | /v1/edits | 文本编辑 | 已实现 |
| POST | /v1/images/generations | 图片生成 | 已实现 |
| POST | /v1/embeddings | 文本嵌入 | 已实现 |
| POST | /v1/engines/:model/embeddings | 嵌入（老版本引擎接口） | 已实现 |
| POST | /v1/audio/transcriptions | 语音转文字 | 已实现 |
| POST | /v1/audio/translations | 语音翻译 | 已实现 |
| POST | /v1/audio/speech | 文字转语音（TTS） | 已实现 |
| POST | /v1/moderations | 内容审核 | 已实现 |
| ANY | /v1/oneapi/proxy/:channelid/*target | 代理到指定渠道 | 已实现 |

#### 未实现的接口

以下路由会返回 "Not implemented" 错误信息。

| 方法 | 路径 | 说明 | 状态 |
|------|------|------|------|
| POST | /v1/images/edits | 图片编辑 | 未实现 |
| POST | /v1/images/variations | 图片变体 | 未实现 |
| GET | /v1/files | 文件列表 | 未实现 |
| POST | /v1/files | 上传文件 | 未实现 |
| DELETE | /v1/files/:id | 删除文件 | 未实现 |
| GET | /v1/files/:id | 获取文件信息 | 未实现 |
| GET | /v1/files/:id/content | 获取文件内容 | 未实现 |
| POST | /v1/fine_tuning/jobs | 创建微调任务 | 未实现 |
| GET | /v1/fine_tuning/jobs | 微调任务列表 | 未实现 |
| GET | /v1/fine_tuning/jobs/:id | 获取微调任务 | 未实现 |
| POST | /v1/fine_tuning/jobs/:id/cancel | 取消微调任务 | 未实现 |
| GET | /v1/fine_tuning/jobs/:id/events | 微调任务事件 | 未实现 |
| DELETE | /v1/models/:model | 删除模型 | 未实现 |
| POST | /v1/assistants | 创建 Assistant | 未实现 |
| GET | /v1/assistants | Assistant 列表 | 未实现 |
| GET | /v1/assistants/:id | 获取 Assistant | 未实现 |
| POST | /v1/assistants/:id | 更新 Assistant | 未实现 |
| DELETE | /v1/assistants/:id | 删除 Assistant | 未实现 |
| POST | /v1/assistants/:id/files | 关联文件到 Assistant | 未实现 |
| GET | /v1/assistants/:id/files | Assistant 文件列表 | 未实现 |
| GET | /v1/assistants/:id/files/:fileId | 获取 Assistant 文件 | 未实现 |
| DELETE | /v1/assistants/:id/files/:fileId | 删除 Assistant 文件 | 未实现 |
| POST | /v1/threads | 创建 Thread | 未实现 |
| GET | /v1/threads/:id | 获取 Thread | 未实现 |
| POST | /v1/threads/:id | 更新 Thread | 未实现 |
| DELETE | /v1/threads/:id | 删除 Thread | 未实现 |
| POST | /v1/threads/:id/messages | 创建消息 | 未实现 |
| GET | /v1/threads/:id/messages/:messageId | 获取消息 | 未实现 |
| POST | /v1/threads/:id/messages/:messageId | 更新消息 | 未实现 |
| GET | /v1/threads/:id/messages/:messageId/files | 消息文件列表 | 未实现 |
| GET | /v1/threads/:id/messages/:messageId/files/:filesId | 获取消息文件 | 未实现 |
| POST | /v1/threads/:id/runs | 创建 Run | 未实现 |
| GET | /v1/threads/:id/runs | Run 列表 | 未实现 |
| GET | /v1/threads/:id/runs/:runsId | 获取 Run | 未实现 |
| POST | /v1/threads/:id/runs/:runsId | 更新 Run | 未实现 |
| POST | /v1/threads/:id/runs/:runsId/submit_tool_outputs | 提交工具输出 | 未实现 |
| POST | /v1/threads/:id/runs/:runsId/cancel | 取消 Run | 未实现 |
| GET | /v1/threads/:id/runs/:runsId/steps | Run 步骤列表 | 未实现 |
| GET | /v1/threads/:id/runs/:runsId/steps/:stepId | 获取 Run 步骤 | 未实现 |

#### 中继接口使用示例

##### 聊天补全

**POST** `/v1/chat/completions`

```json
{
  "model": "gpt-3.5-turbo",
  "messages": [
    {"role": "user", "content": "Hello!"}
  ]
}
```

##### 代理到指定渠道

**ANY** `/v1/oneapi/proxy/:channelid/*target`

示例：`POST /v1/oneapi/proxy/1/chat/completions`

将请求转发到 ID 为 1 的渠道，目标路径为 `/chat/completions`。
