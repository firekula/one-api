# 功能规范：入站 Anthropic Messages API 原生中继

## 1. 背景

One API 当前对外只暴露 OpenAI 兼容端点（`/v1/chat/completions` 等），认证方式为 `Authorization: Bearer sk-xxx`。Claude Code 等工具原生使用 Anthropic Messages API 格式，无法直接接入 One API 作为中转网关。

**核心需求**：新建一条**完整的原生 Anthropic API 中继链路**——入站接受 Anthropic Messages API 格式请求，直接透传到上游 AI 提供商的 Anthropic 兼容端点，响应原生 Anthropic 格式返回。**整个链路不做任何格式转换，杜绝语义损耗。**

### 与现有 OpenAI 管道的关系

```
现有管道  : 客户端 --OpenAI--> OneAPI --[adaptor转换]--> 上游
新增管道  : Claude Code --Anthropic--> OneAPI --[原样透传]--> 上游 Anthropic 兼容端点
```

两条管道相互独立，共享鉴权体系（token 验证）和渠道管理，但请求/响应格式完全不同。

## 2. 功能需求

### 2.1 根因分析

| # | 问题 | 根因 | 源码证据 |
|---|------|------|----------|
| 1 | 无 Anthropic 格式入站端点 | [router/relay.go:20-73](file:///Y:/Project/Server/one-api/router/relay.go#L20) 路由组仅注册 OpenAI 格式端点，无 `/v1/messages` | 需新增独立路由组 |
| 2 | 认证中间件仅识别 Bearer Token | [middleware/auth.go:94](file:///Y:/Project/Server/one-api/middleware/auth.go#L94) `TokenAuth` 只从 `Authorization` 头提取 token；Anthropic 使用 `x-api-key` 头 | 需新增 `AnthropicTokenAuth` |
| 3 | 渠道分发依赖 `requestModel` 从 context 读取 | [middleware/distributor.go:45](file:///Y:/Project/Server/one-api/middleware/distributor.go#L45) 从 `c.GetString(ctxkey.RequestModel)` 获取模型名 | Anthropic 请求的 `model` 字段需在分发前解析注入 |
| 4 | 现有出站 Anthropic 适配器做了 OpenAI 格式转换 | [adaptor/anthroptic/main.go:338-378](file:///Y:/Project/Server/one-api/relay/adaptor/anthropic/main.go#L338) `Handler` 中将 Anthropic 响应转为 OpenAI 格式再写入客户端 | 需新增**原生响应处理函数**，跳过 OpenAI 转换，原样透传 |
| 5 | Stream 处理也做了格式转换 | [main.go:249-335](file:///Y:/Project/Server/one-api/relay/adaptor/anthropic/main.go#L249) `StreamHandler` 将 Anthropic SSE 事件转为 OpenAI SSE 格式 | 需新增**原生 Stream 处理函数**，SSE 事件原样透传 |
| 6 | `GET /v1/models` 仅返回 OpenAI 格式 + 仅接受 Bearer 认证 | [controller/model.go:132-169](file:///Y:/Project/Server/one-api/controller/model.go#L132) `ListModels` 返回 OpenAI 的 `{"object":"list","data":[...]}` 格式；`modelsRouter` 只使用 `TokenAuth`（Bearer）。Claude Code 调 `GET /v1/models` 时会因 `x-api-key` 头被拒（401）或收到无法解析的 OpenAI 格式 | 需新增 `FlexibleTokenAuth` 中间件同时接受两种头，handler 根据请求头类型返回对应格式 |

### 2.2 方案概览

```mermaid
sequenceDiagram
    participant CC as Claude Code
    participant Auth as AnthropicTokenAuth
    participant Parse as 模型提取
    participant Dist as Distributor
    participant Relay as RelayAnthropicHelper
    participant Native as NativeHandler/Stream
    participant Up as 上游 Anthropic 兼容端点

    CC->>Auth: POST /v1/messages<br/>x-api-key: sk-xxx<br/>{"model":"claude-sonnet-4-20250514","messages":[...]}
    Auth->>Auth: 提取 sk-xxx，调用 ValidateUserToken
    Auth->>Parse: 鉴权通过，解析 JSON 提取 model 字段
    Parse->>Dist: model 注入 ctxkey.RequestModel
    Dist->>Dist: CacheGetRandomSatisfiedChannel(group, model)
    Dist->>Relay: 设置 channel 上下文 (BaseURL/APIKey/Type)
    Relay->>Native: 原始 Anthropic 请求体 → DoRequest (透传)
    Native->>Up: POST {baseURL}/v1/messages<br/>x-api-key: {channel.Key}<br/>Body: 原始 Anthropic JSON
    Up-->>Native: Anthropic 原生响应
    alt 流模式
        Native->>Native: SSE 逐行原样转发<br/>从 message_delta 事件提取 usage
        Native-->>CC: event: message_start<br/>event: content_block_delta<br/>...<br/>event: message_stop
    else 非流模式
        Native->>Native: 读取完整响应体<br/>解析 usage 字段<br/>原样写入 c.Writer
        Native-->>CC: {"id":"msg_xxx","type":"message","content":[...],...}
    end
    Relay->>Relay: postConsumeQuota(usage)
```

**关键设计决策**：
- 请求体从客户端到上游**原样透传**，不经 `ConvertRequest`
- 响应体从上游到客户端**原样透传**，不经 `ResponseClaude2OpenAI` / `StreamResponseClaude2OpenAI`
- usage 提取逻辑**复用**现有 Anthropic 响应解析（`anthropic.Response` / `anthropic.StreamResponse` 结构体），但仅用于记账，不影响响应内容

### 2.3 改动详情

#### 改动 1：新增原生响应处理函数（透传模式）

**文件**：新建 `relay/adaptor/anthropic/native.go`

这两个函数是核心——与现有 `Handler` / `StreamHandler` 的区别在"写响应"这一步：**不调转换函数，不 marshal OpenAI 格式，直接原样写入**。

**非流式原生透传**：

```go
// NativeHandler 非流模式原生透传：读取 Anthropic 响应 → 提取 usage → 原样写入客户端
func NativeHandler(c *gin.Context, resp *http.Response) (*model.ErrorWithStatusCode, *model.Usage) {
    responseBody, err := io.ReadAll(resp.Body)
    if err != nil {
        return openai.ErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError), nil
    }
    _ = resp.Body.Close()

    var claudeResponse Response
    if err = json.Unmarshal(responseBody, &claudeResponse); err != nil {
        return openai.ErrorWrapper(err, "unmarshal_response_body_failed", http.StatusInternalServerError), nil
    }
    // 上游错误 → 保持 Anthropic 错误格式返回
    if claudeResponse.Error.Type != "" {
        c.Writer.Header().Set("Content-Type", "application/json")
        c.Writer.WriteHeader(resp.StatusCode)
        _, _ = c.Writer.Write(responseBody)
        return &model.ErrorWithStatusCode{
            Error:      model.Error{Message: claudeResponse.Error.Message, Code: claudeResponse.Error.Type},
            StatusCode: resp.StatusCode,
        }, nil
    }
    usage := model.Usage{
        PromptTokens:     claudeResponse.Usage.InputTokens,
        CompletionTokens: claudeResponse.Usage.OutputTokens,
        TotalTokens:      claudeResponse.Usage.InputTokens + claudeResponse.Usage.OutputTokens,
    }
    // 关键：原样写回 Anthropic 响应 JSON，不做 OpenAI 格式转换
    c.Writer.Header().Set("Content-Type", "application/json")
    c.Writer.WriteHeader(resp.StatusCode)
    _, _ = c.Writer.Write(responseBody)
    return nil, &usage
}
```

**流式原生透传**：

```go
// NativeStreamHandler 流模式原生透传：SSE 事件原样转发，从 message_delta 提取 usage
func NativeStreamHandler(c *gin.Context, resp *http.Response) (*model.ErrorWithStatusCode, *model.Usage) {
    scanner := bufio.NewScanner(resp.Body)
    scanner.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
        if atEOF && len(data) == 0 { return 0, nil, nil }
        if i := strings.Index(string(data), "\n"); i >= 0 { return i + 1, data[0:i], nil }
        if atEOF { return len(data), data, nil }
        return 0, nil, nil
    })

    common.SetEventStreamHeaders(c)
    var usage model.Usage

    for scanner.Scan() {
        data := scanner.Text()
        if len(data) < 6 || !strings.HasPrefix(data, "data:") { continue }

        // 原样转发这条 SSE 行，不做格式转换
        render.StringData(c, strings.TrimPrefix(data, "data: "))

        // 静默解析：仅提取 usage，不改变响应
        payload := strings.TrimSpace(strings.TrimPrefix(data, "data:"))
        var streamResp StreamResponse
        if json.Unmarshal([]byte(payload), &streamResp) == nil {
            if streamResp.Type == "message_delta" && streamResp.Usage != nil {
                usage.PromptTokens += streamResp.Usage.InputTokens
                usage.CompletionTokens += streamResp.Usage.OutputTokens
                usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
            }
        }
    }
    if scanner.Err() != nil {
        logger.SysError("error reading stream: " + scanner.Err().Error())
    }
    render.Done(c)
    _ = resp.Body.Close()
    return nil, &usage
}
```

#### 改动 2：新增认证中间件 — `x-api-key` 头支持

**文件**：修改 `middleware/auth.go`

新增 `AnthropicTokenAuth`，与现有 `TokenAuth` 的唯一区别：token 从 `x-api-key` 头提取而非 `Authorization: Bearer`。

```go
// AnthropicTokenAuth 从 x-api-key 头提取 token，其余鉴权逻辑与 TokenAuth 完全一致
func AnthropicTokenAuth() func(c *gin.Context) {
    return func(c *gin.Context) {
        ctx := c.Request.Context()
        key := c.Request.Header.Get("x-api-key")
        key = strings.TrimPrefix(key, "sk-")
        parts := strings.Split(key, "-")
        key = parts[0]
        token, err := model.ValidateUserToken(key)
        if err != nil {
            abortWithMessage(c, http.StatusUnauthorized, err.Error())
            return
        }
        // 子网检查（同 TokenAuth）
        if token.Subnet != nil && *token.Subnet != "" {
            if !network.IsIpInSubnets(ctx, c.ClientIP(), *token.Subnet) {
                abortWithMessage(c, http.StatusForbidden, fmt.Sprintf("该令牌只能在指定网段使用：%s，当前 ip：%s", *token.Subnet, c.ClientIP()))
                return
            }
        }
        // 用户状态 + 黑名单检查
        userEnabled, err := model.CacheIsUserEnabled(token.UserId)
        if err != nil {
            abortWithMessage(c, http.StatusInternalServerError, err.Error())
            return
        }
        if !userEnabled || blacklist.IsUserBanned(token.UserId) {
            abortWithMessage(c, http.StatusForbidden, "用户已被封禁")
            return
        }
        // 从 Anthropic 请求体解析 model（需要在 middleware 中做，因为 distributor 需要）
        requestModel, err := getRequestModel(c)
        if err != nil {
            abortWithMessage(c, http.StatusBadRequest, err.Error())
            return
        }
        c.Set(ctxkey.RequestModel, requestModel)
        // 模型权限检查
        if token.Models != nil && *token.Models != "" && requestModel != "" &&
            !isModelInList(requestModel, *token.Models) {
            abortWithMessage(c, http.StatusForbidden, fmt.Sprintf("该令牌无权使用模型：%s", requestModel))
            return
        }
        c.Set(ctxkey.Id, token.UserId)
        c.Set(ctxkey.TokenId, token.Id)
        c.Set(ctxkey.TokenName, token.Name)
        if len(parts) > 1 {
            if model.IsAdmin(token.UserId) {
                c.Set(ctxkey.SpecificChannelId, parts[1])
            } else {
                abortWithMessage(c, http.StatusForbidden, "普通用户不支持指定渠道")
                return
            }
        }
        c.Next()
    }
}
```

> **代码复用说明**：`AnthropicTokenAuth` 与 `TokenAuth`（[auth.go:91-151](file:///Y:/Project/Server/one-api/middleware/auth.go#L91)）共享 90% 逻辑。为避免重复，可以考虑抽取共享的 `validateTokenAndSetContext(c, token, key)` 函数，两个 Auth 各自提取 token 后调用同一个校验函数。这作为实施阶段的优化选项。

#### 改动 3：模型名提取 — 从 Anthropic JSON 中提取 `model` 字段

**文件**：修改 `middleware/utils.go` 中的 `getRequestModel` 函数（或新增变体）

现有 [`getRequestModel`](file:///Y:/Project/Server/one-api/middleware/utils.go#L23) 从请求体顶层 JSON 取 `model` 字段，而 Anthropic Messages API 请求的 `model` 字段也在顶层 JSON：

```json
{"model": "claude-sonnet-4-20250514", "max_tokens": 4096, "messages": [...]}
```

好消息：Anthropic 请求的 model 字段位置与 OpenAI 请求**完全一致**——都在顶层 JSON 的 `model` key。现有 `getRequestModel` 不需要修改，可直接复用。

#### 改动 4：新增路由 `POST /v1/messages`

**文件**：修改 `router/relay.go`

```go
// 在 SetRelayRouter 函数内新增
anthropicRouter := router.Group("/v1")
anthropicRouter.Use(middleware.RelayPanicRecover(), middleware.AnthropicTokenAuth(), middleware.Distribute())
{
    anthropicRouter.POST("/messages", controller.RelayAnthropic)
}
```

> 使用独立路由组是因为中间件链不同：`AnthropicTokenAuth` 替代 `TokenAuth`。`Distribute` 保持不变。

#### 改动 5：新增 relay mode 常量 + 路径映射

**文件**：修改 `relay/relaymode/define.go`

```go
const (
    // ... 现有 14 个常量 ... (Unknown=0 到 Proxy=15)
    AnthropicMessages  // 16 — 入站 Anthropic Messages API
)
```

**文件**：修改 `relay/relaymode/helper.go`

```go
func GetByPath(path string) int {
    // ... 现有逻辑 ...
    if strings.HasPrefix(path, "/v1/messages") {
        return AnthropicMessages
    }
    return Unknown
}
```

#### 改动 6：新增 Anthropic 原生中继控制器

**文件**：新建 `relay/controller/anthropic.go`

核心函数 `RelayAnthropicHelper`——这是整条新管道的调度中心：

```go
func RelayAnthropicHelper(c *gin.Context) *model.ErrorWithStatusCode {
    ctx := c.Request.Context()
    meta := meta.GetByContext(c)
    // meta.Mode 此时为 AnthropicMessages
    // meta.APIType 由 channeltype.ToAPIType（如果 channel type=18 则 apiType=Anthropic）

    // 1. 读取原始 Anthropic 请求体（透传用）
    originalBody, err := common.GetRequestBody(c)
    if err != nil {
        return openai.ErrorWrapper(err, "read_request_body_failed", http.StatusBadRequest)
    }
    // 2. 解析 model 用于后续逻辑（已在 middleware 中设置到 context，这里做二次确认）
    var anthropicReq anthropic.Request
    if err := json.Unmarshal(originalBody, &anthropicReq); err != nil {
        return openai.ErrorWrapper(err, "invalid_anthropic_request", http.StatusBadRequest)
    }
    meta.IsStream = anthropicReq.Stream
    meta.OriginModelName = anthropicReq.Model

    // 3. 模型映射
    mappedModel, _ := getMappedModelName(anthropicReq.Model, meta.ModelMapping)
    meta.ActualModelName = mappedModel

    // 4. 获取 adaptor（对 Anthropic channel 返回 &anthropic.Adaptor{}）
    adaptor := relay.GetAdaptor(meta.APIType)
    if adaptor == nil {
        return openai.ErrorWrapper(fmt.Errorf("unsupported channel for Anthropic relay"), "invalid_api_type", http.StatusBadRequest)
    }
    adaptor.Init(meta)

    // 5. 透传发送：直接使用原始 Anthropic 请求体，不调 ConvertRequest
    resp, err := adaptor.DoRequest(c, meta, bytes.NewReader(originalBody))
    if err != nil {
        logger.Errorf(ctx, "DoRequest failed: %s", err.Error())
        return openai.ErrorWrapper(err, "do_request_failed", http.StatusInternalServerError)
    }
    if isErrorHappened(meta, resp) {
        return RelayErrorHandler(resp)
    }

    // 6. 原生响应处理：透传 Anthropic 响应，仅提取 usage 用于计费
    usage, respErr := anthropic.NativeDoResponse(c, resp, meta)
    if respErr != nil {
        return respErr
    }
    return nil
}
```

在 `controller/relay.go` 的 `relayHelper` 中新增分支：

```go
case relaymode.AnthropicMessages:
    err = controller.RelayAnthropicHelper(c)
```

#### 改动 7：NativeDoResponse 分发函数

**文件**：新建 `relay/adaptor/anthropic/native.go`（与改动 1 同文件）

```go
// NativeDoResponse 根据 stream 模式分发到对应的原生处理函数
func NativeDoResponse(c *gin.Context, resp *http.Response, meta *meta.Meta) (*model.Usage, *model.ErrorWithStatusCode) {
    if meta.IsStream {
        return NativeStreamHandler(c, resp)
    }
    return NativeHandler(c, resp)
}
```

#### 改动 8：modelsRouter 使用 FlexibleTokenAuth — 同时接受 Bearer + x-api-key

**文件**：修改 `middleware/auth.go`

新增 `FlexibleTokenAuth` 中间件。该中间件同时兼容 OpenAI 和 Anthropic 两种认证方式：优先从 `x-api-key` 头提取（Anthropic），若为空则从 `Authorization: Bearer` 提取（OpenAI）。认证通过后设置 context 标记 `ctxkey.AuthFormat` 指示响应格式。

```go
func FlexibleTokenAuth() func(c *gin.Context) {
    return func(c *gin.Context) {
        ctx := c.Request.Context()
        key := c.Request.Header.Get("x-api-key")
        authFormat := "anthropic"
        if key == "" {
            key = c.Request.Header.Get("Authorization")
            key = strings.TrimPrefix(key, "Bearer ")
            authFormat = "openai"
        }
        // 后续 token 校验逻辑与 TokenAuth 完全一致
        key = strings.TrimPrefix(key, "sk-")
        parts := strings.Split(key, "-")
        key = parts[0]
        token, err := model.ValidateUserToken(key)
        if err != nil {
            abortWithMessage(c, http.StatusUnauthorized, err.Error())
            return
        }
        if token.Subnet != nil && *token.Subnet != "" {
            if !network.IsIpInSubnets(ctx, c.ClientIP(), *token.Subnet) {
                abortWithMessage(c, http.StatusForbidden, fmt.Sprintf(...))
                return
            }
        }
        userEnabled, err := model.CacheIsUserEnabled(token.UserId)
        if err != nil {
            abortWithMessage(c, http.StatusInternalServerError, err.Error())
            return
        }
        if !userEnabled || blacklist.IsUserBanned(token.UserId) {
            abortWithMessage(c, http.StatusForbidden, "用户已被封禁")
            return
        }
        c.Set(ctxkey.Id, token.UserId)
        c.Set(ctxkey.TokenId, token.Id)
        c.Set(ctxkey.TokenName, token.Name)
        c.Set("auth_format", authFormat) // "anthropic" 或 "openai"
        c.Next()
    }
}
```

**文件**：修改 `router/relay.go`

将 `modelsRouter` 的认证中间件从 `TokenAuth` 改为 `FlexibleTokenAuth`：

```go
modelsRouter := router.Group("/v1/models")
modelsRouter.Use(middleware.FlexibleTokenAuth()) // 改动前：middleware.TokenAuth()
{
    modelsRouter.GET("", controller.ListModels)
    modelsRouter.GET("/:model", controller.RetrieveModel)
}
```

#### 改动 9：ListModels / RetrieveModel — 支持 Anthropic 格式返回

**文件**：修改 `controller/model.go`

在 `ListModels` 和 `RetrieveModel` 函数中，检测 `auth_format` context 值，若为 `"anthropic"` 则返回 Anthropic Messages API 格式。

**Anthropic 模型列表格式**：
```json
{
  "data": [
    {"id": "claude-sonnet-4-20250514", "type": "model", "display_name": "Claude Sonnet 4", "created_at": "2025-05-14T00:00:00Z"}
  ],
  "has_more": false,
  "first_id": "claude-instant-1.2",
  "last_id": "claude-4-opus-20250514"
}
```

**Anthropic 单个模型格式**：
```json
{"id": "claude-sonnet-4-20250514", "type": "model", "display_name": "Claude Sonnet 4", "created_at": "2025-05-14T00:00:00Z"}
```

```go
type AnthropicModel struct {
    Id          string `json:"id"`
    Type        string `json:"type"`
    DisplayName string `json:"display_name"`
    CreatedAt   string `json:"created_at"`
}

type AnthropicModelList struct {
    Data    []AnthropicModel `json:"data"`
    HasMore bool             `json:"has_more"`
    FirstId string           `json:"first_id"`
    LastId  string           `json:"last_id"`
}

// 在 ListModels 中增加格式判断
func ListModels(c *gin.Context) {
    // ... 现有模型收集逻辑不变 ...
    authFormat, _ := c.Get("auth_format")
    if authFormat == "anthropic" {
        c.JSON(200, convertToAnthropicModelList(availableOpenAIModels))
        return
    }
    // 原有 OpenAI 格式返回
    c.JSON(200, gin.H{"object": "list", "data": availableOpenAIModels})
}

// RetrieveModel 同理
func RetrieveModel(c *gin.Context) {
    modelId := c.Param("model")
    if model, ok := modelsMap[modelId]; ok {
        authFormat, _ := c.Get("auth_format")
        if authFormat == "anthropic" {
            c.JSON(200, convertToAnthropicModel(model))
            return
        }
        c.JSON(200, model)
        return
    }
    // ... 错误处理
}
```

### 2.4 现有机制复用清单

| 复用机制 | 位置 | 说明 |
|----------|------|------|
| `ValidateUserToken` | [auth.go:99](file:///Y:/Project/Server/one-api/middleware/auth.go#L99) | `AnthropicTokenAuth` 调用同一函数 |
| `Distribute` 中间件 | [distributor.go:20](file:///Y:/Project/Server/one-api/middleware/distributor.go#L20) | 100% 复用，只依赖 context 中的 `requestModel` |
| `SetupContextForSelectedChannel` | [distributor.go:64](file:///Y:/Project/Server/one-api/middleware/distributor.go#L64) | 100% 复用 |
| `getRequestModel` | [utils.go:23](file:///Y:/Project/Server/one-api/middleware/utils.go#L23) | Anthropic 的 model 字段也在顶层 JSON，直接复用 |
| `GetAdaptor` | [adaptor.go:27](file:///Y:/Project/Server/one-api/relay/adaptor.go#L27) | 100% 复用，`apiType=Anthropic` → `&anthropic.Adaptor{}` |
| `anthropic.Adaptor.GetRequestURL` | [adaptor.go:23](file:///Y:/Project/Server/one-api/relay/adaptor/anthropic/adaptor.go#L23) | 100% 复用，构造 `/v1/messages` URL |
| `anthropic.Adaptor.SetupRequestHeader` | [adaptor.go:27-44](file:///Y:/Project/Server/one-api/relay/adaptor/anthropic/adaptor.go#L27) | 100% 复用，设置 `x-api-key`/`anthropic-version`/`anthropic-beta` |
| `DoRequestHelper` | [common.go:21](file:///Y:/Project/Server/one-api/relay/adaptor/common.go#L21) | 100% 复用，标准 HTTP 请求发送 |
| `anthropic.Request` / `Response` / `StreamResponse` 结构体 | [model.go](file:///Y:/Project/Server/one-api/relay/adaptor/anthropic/model.go) | 仅用于 usage 提取，不影响响应格式 |
| `postConsumeQuota` | [helper.go:97](file:///Y:/Project/Server/one-api/relay/controller/helper.go#L97) | 100% 复用 |
| `isErrorHappened` / `RelayErrorHandler` | [helper.go:154](file:///Y:/Project/Server/one-api/relay/controller/helper.go#L154) / error.go | 100% 复用 |
| 重试逻辑 `Relay()` | [relay.go:45](file:///Y:/Project/Server/one-api/controller/relay.go#L45) | 新增 relay mode 分支后复用重试循环 |
| `getMappedModelName` | [helper.go:143](file:///Y:/Project/Server/one-api/relay/controller/helper.go#L143) | 100% 复用 |
| `render.StringData` / `render.Done` | [render.go](file:///Y:/Project/Server/one-api/common/render/render.go) | 100% 复用，SSE 事件转发 |
| `ListModels` / `RetrieveModel` 中的模型收集逻辑 | [model.go:132-169](file:///Y:/Project/Server/one-api/controller/model.go#L132) | 模型集合构建逻辑不变，仅追加 Anthropic 格式输出分支 |
| `models` 全局变量 + `init()` 中的模型初始化 | [model.go:45-115](file:///Y:/Project/Server/one-api/controller/model.go#L45) | 100% 复用，Anthropic 格式输出使用同一数据源 |

**不新增/不修改的组件**：渠道类型常量（`Anthropic=18` 已存在）、API 类型常量（`apitype.Anthropic=1` 已存在）、数据库模型、Adaptor 接口定义、出站 Adaptor 注册。

## 3. 涉及文件清单

| 文件 | 改动类型 | 所属层 | 说明 |
|------|----------|--------|------|
| `middleware/auth.go` | 修改 | Go 中间件 | 新增 `AnthropicTokenAuth()` 函数 |
| `router/relay.go` | 修改 | Go 路由 | 新增 `/v1/messages` 路由组 |
| `relay/relaymode/define.go` | 修改 | Go 常量 | 新增 `AnthropicMessages` 常量 |
| `relay/relaymode/helper.go` | 修改 | Go 辅助 | `/v1/messages` → `AnthropicMessages` 映射 |
| `controller/relay.go` | 修改 | Go 控制器 | `relayHelper` switch 新增分支 |
| `relay/controller/anthropic.go` | **新增** | Go 控制器 | `RelayAnthropicHelper` — 原生中继调度 |
| `relay/adaptor/anthropic/native.go` | **新增** | Go 适配器 | `NativeHandler` / `NativeStreamHandler` / `NativeDoResponse` — 原生透传响应处理 |
| `controller/model.go` | 修改 | Go 控制器 | `ListModels` / `RetrieveModel` 增加 Anthropic 格式输出分支 |

**总计**：6 个文件修改 + 2 个文件新增 = 8 个文件

## 4. 边界情况

| # | 场景 | 风险 | 缓解策略 |
|---|------|------|----------|
| 1 | **上游返回非 Anthropic 格式错误**（如反向代理返回 HTML 502 页面） | `json.Unmarshal` 失败，`NativeHandler` 返回解析错误而非上游原始错误信息 | 在 unmarshal 失败时回退：将原始响应体作为 Anthropic error 格式包装：`{"type":"error","error":{"type":"upstream_error","message":"<原始响应体>"}}` |
| 2 | **上游 channel 不是 Anthropic 类型**（如用户将 OpenAI 渠道配置为 Anthropic 入站的目标） | 原生透传发送 Anthropic 格式请求到 OpenAI 端点，上游返回 404 或格式错误 | `RelayAnthropicHelper` 中检查 `meta.APIType != apitype.Anthropic` 时返回明确错误："Anthropic 入站仅支持 Anthropic 类型渠道" |
| 3 | **stream 中断未收到 `message_stop`** | 上游连接断开时未发送 `message_delta` 事件，`NativeStreamHandler` 无法提取 usage，导致 usage=0 | 在 stream 结束后（scanner 退出）检查 usage.TotalTokens==0 时记录 warning 日志（不影响客户端），额度按 0 结算 |
| 4 | **Anthropic 请求中 `model` 字段不存在** | `getRequestModel` 返回空字符串，distributor 无法匹配渠道 | middleware 中 `getRequestModel` 失败时返回 400 "model field is required" |
| 5 | **并发安全：`NativeStreamHandler` 中 scanner + c.Writer 并发写入** | scanner 读取和 `render.StringData` 写入在不同的 goroutine 中可能冲突 | Gin 的 `c.Stream()` 模式保证了回调函数在单 goroutine 中执行；当前使用 `for scanner.Scan()` 在主 goroutine 中同步写入，无并发问题 |
| 6 | **上游 Anthropic channel 的 base_url 配置为空** | `meta.BaseURL` fallback 到 `channeltype.ChannelBaseURLs[18]` = `https://api.anthropic.com`，正确的默认值 | 无需缓解；但如果用户配置了自定义 base_url（如 `https://my-proxy.com/anthropic`），GetRequestURL 拼接 `/v1/messages` 后 URL 为 `https://my-proxy.com/anthropic/v1/messages`，需确保 base_url 末尾无斜杠 |
| 7 | **大量并发 Anthropic 流式请求** | 每个流式请求持有 scanner + HTTP 连接，goroutine 泄漏风险 | `NativeStreamHandler` 中显式 `resp.Body.Close()` 确保连接释放；现有 `client.HTTPClient` 的连接池机制复用 TCP 连接 |
| 8 | **`GET /v1/models` 同时被 Bearer 和 x-api-key 访问** | 同一个 endpoint 返回两种格式，若 context 中 `auth_format` 标记丢失或错误设置会导致客户端收到错误格式 | `FlexibleTokenAuth` 在认证阶段就确定 `auth_format`，model handler 只读不写；无并发修改风险 |

## 5. 出厂自检

| 自检项 | 要求 | 实际 | 达标 |
|--------|------|------|------|
| file:/// 链接数 | ≥ 3 | 15 | ✅ |
| Mermaid 图 | ≥ 1 | 1 (时序图) | ✅ |
| 代码片段数 | ≥ 2 | 6 | ✅ |
| 边界情况条目 | ≥ 4 | 8 | ✅ |
| 机制复用说明 | 有 | 有 (16 项) | ✅ |
| 文件清单表格 | 有 | 有 (8 文件) | ✅ |
