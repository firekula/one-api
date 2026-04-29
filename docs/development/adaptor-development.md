# 渠道适配器开发指南

> 本文档面向希望为 One API 项目新增 AI 渠道支持的贡献者。阅读前请确保你对 Go 语言和 OpenAI API 格式有基本了解。

---

## 1. 判断：需要新适配器吗？

One API 中存在两种适配模式，选择哪种取决于目标渠道的 API 格式。

### 不需要新适配器：OpenAI 兼容渠道

目标渠道的 API 请求/响应格式与 OpenAI 一致（或接近一致），可以直接复用 OpenAI 适配器。你只需要在现有代码中注册渠道类型、Base URL 和模型列表即可。

典型判断标准：

- 请求体使用 `messages` 数组，格式为 `[{role, content}, ...]`
- 响应体返回 `choices[].message.content`
- Stream 模式使用 SSE，数据格式为 `data: {...}`
- 鉴权方式为 `Authorization: Bearer <token>`

**这类渠道的例子**：Groq、DeepSeek、Moonshot、SiliconFlow、XunfeiV2、BaiduV2、Novita、TogetherAI、StepFun、零一万物、百川、豆包等。

### 需要新适配器：非 OpenAI 兼容渠道

目标渠道的 API 格式与 OpenAI 存在显著差异，请求和响应都需要格式转换。

典型判断标准：

- 消息结构不同（如 Anthropic 的 `content` 数组格式）
- 请求端点不同（如 Gemini 的 `generateContent`）
- 鉴权方式特殊（如百度文心的 OAuth access_token）
- 流式传输协议不同（如讯飞星火的 WebSocket）

**这类渠道的例子**：Anthropic Messages API、Gemini Pro API、百度文心千帆 V1、阿里通义千问、智谱 ChatGLM、腾讯混元等。

### 决策流程图

```
目标渠道 API 格式
        │
        ▼
   与 OpenAI 格式兼容？
   ┌────┴────┐
  是│        │否
  ┌─▼──┐  ┌─▼──────┐
  │注册 │  │实现完整 │
  │兼容 │  │适配器   │
  │渠道 │  │        │
  └────┘  └────────┘
```

---

## 2. 适配器目录结构

### OpenAI 兼容渠道

```
relay/adaptor/<provider>/
  constants.go        # 模型常量列表（必需）
  main.go             # 自定义 GetRequestURL 等（可选，仅在需要覆盖默认 URL 时）
```

### 完整适配器

```
relay/adaptor/<provider>/
  constants.go        # 模型常量列表（必需）
  main.go             # ConvertRequest + DoResponse 等核心转换逻辑（必需）
  adaptor.go          # 自定义 Adaptor 结构体，实现 Adaptor 接口（必需）
  model.go            # 渠道专用的请求/响应数据结构（可选，当格式复杂时）
```

---

## 3. OpenAI 兼容渠道注册教程（以 BaiduV2 为例）

以下步骤展示如何将一个 OpenAI 兼容渠道添加到 One API 中。

### Step 1 — 定义渠道类型常量

编辑 `relay/channeltype/define.go`，在 `const` 块中新增一个常量，取值为一个未被使用的编号：

```go
const (
    // ... 已有的常量
    Replicate = 46
    BaiduV2   = 47  // 选择一个未使用的编号
    XunfeiV2  = 48
    AliBailian = 49
)
```

> **注意**：`Dummy` 常量之前的最后一个值是编号上限，`Dummy` 本身只用于计数，不要在其之后添加渠道。

### Step 2 — 注册 Base URL

编辑 `relay/channeltype/url.go`，在 `ChannelBaseURLs` 切片中对应索引位置填入默认 Base URL：

```go
var ChannelBaseURLs = []string{
    // ... 索引 46 之前的值
    "https://api.replicate.com/v1/models/",  // 46 Replicate
    "https://qianfan.baidubce.com",           // 47 BaiduV2
    "https://spark-api-open.xf-yun.com",      // 48 XunfeiV2
    "https://dashscope.aliyuncs.com",          // 49 AliBailian
    "",                                        // 50 OpenAICompatible
    "https://generativelanguage.googleapis.com/v1beta/openai/", // 51 GeminiOpenAICompatible
}
```

`ChannelBaseURLs` 的长度必须等于 `Dummy` 的值，否则 `init()` 函数会 panic。

### Step 3 — 创建模型常量文件

创建 `relay/adaptor/<provider>/constants.go`，定义该渠道支持的模型列表：

```go
package baiduv2

// ModelList 是百度文心千帆 V2 渠道支持的模型列表。
// 参考文档:
// https://cloud.baidu.com/doc/WENXINWORKSHOP/s/Fm2vrveyu
var ModelList = []string{
    "ernie-4.0-8k-latest",
    "ernie-4.0-8k-preview",
    "ernie-4.0-8k",
    "ernie-4.0-turbo-8k-latest",
    "ernie-4.0-turbo-8k-preview",
    "ernie-4.0-turbo-8k",
    "ernie-4.0-turbo-128k",
    "ernie-3.5-8k-preview",
    "ernie-3.5-8k",
    "ernie-3.5-128k",
    "ernie-speed-8k",
    "ernie-speed-128k",
    "ernie-speed-pro-128k",
    "ernie-lite-8k",
    "ernie-lite-pro-128k",
    "ernie-tiny-8k",
    "ernie-char-8k",
    "ernie-char-fiction-8k",
    "ernie-novel-8k",
    "deepseek-v3",
    "deepseek-r1",
    "deepseek-r1-distill-qwen-32b",
    "deepseek-r1-distill-qwen-14b",
}
```

### Step 4 — 创建自定义 URL 处理（可选）

如果渠道的请求 URL 构造方式与标准 OpenAI 不同，创建 `relay/adaptor/<provider>/main.go`，实现 `GetRequestURL` 函数。

例如 BaiduV2 的 URL 路径为 `/v2/chat/completions`，与 OpenAI 的标准路径 `/v1/chat/completions` 不同：

```go
package baiduv2

import (
    "fmt"

    "github.com/songquanpeng/one-api/relay/meta"
    "github.com/songquanpeng/one-api/relay/relaymode"
)

func GetRequestURL(meta *meta.Meta) (string, error) {
    switch meta.Mode {
    case relaymode.ChatCompletions:
        return fmt.Sprintf("%s/v2/chat/completions", meta.BaseURL), nil
    default:
    }
    return "", fmt.Errorf("unsupported relay mode %d for baidu v2", meta.Mode)
}
```

对于大多数 OpenAI 兼容渠道（如 Groq、DeepSeek、Moonshot），请求 URL 就是 `meta.BaseURL + meta.RequestURLPath`，无需自定义 `GetRequestURL`，标准实现会自动处理。

### Step 5 — 在兼容渠道注册表中注册

编辑 `relay/adaptor/openai/compatible.go`，做两件事：

**5a. 将渠道类型加入 `CompatibleChannels` 切片：**

```go
var CompatibleChannels = []int{
    channeltype.Azure,
    channeltype.AI360,
    // ... 其他兼容渠道
    channeltype.BaiduV2,
    channeltype.XunfeiV2,
    // ...
}
```

**5b. 在 `GetCompatibleChannelMeta` 函数中添加 case 分支：**

```go
func GetCompatibleChannelMeta(channelType int) (string, []string) {
    switch channelType {
    // ... 已有 case
    case channeltype.BaiduV2:
        return "baiduv2", baiduv2.ModelList
    case channeltype.XunfeiV2:
        return "xunfeiv2", xunfeiv2.ModelList
    // ...
    }
}
```

该函数返回两个值：渠道名称（用于显示和日志）和模型列表。

**5c. 在 `relay/adaptor/openai/adaptor.go` 的 `GetRequestURL` 中添加自定义 URL 路由（如果 Step 4 实现了 `main.go`）：**

```go
func (a *Adaptor) GetRequestURL(meta *meta.Meta) (string, error) {
    switch meta.ChannelType {
    // ...
    case channeltype.BaiduV2:
        return baiduv2.GetRequestURL(meta)  // 调用自定义 URL 构造
    case channeltype.XunfeiV2:
        return xunfeiv2.GetRequestURL(meta)
    // ...
    default:
        return GetFullRequestURL(meta.BaseURL, meta.RequestURLPath, meta.ChannelType), nil
    }
}
```

> **注意**：如果渠道不需要自定义 URL 构造（即标准 `BaseURL + /v1/...` 路径即可），则不需要添加此 case，默认分支会自动处理。

### Step 6 — 添加前端选项

编辑 `web/default/src/constants/channel.constants.js`，在 `CHANNEL_OPTIONS` 数组中新增一条：

```javascript
export const CHANNEL_OPTIONS = [
  // ... 已有的选项
  {
    key: 47,
    text: '百度文心千帆 V2',
    value: 47,
    color: 'blue',
    tip: '请前往<a href="https://console.bce.baidu.com/iam/#/iam/apikey/list" target="_blank">此处</a>获取 API Key',
  },
  // ...
];
```

字段说明：

| 字段 | 说明 |
|------|------|
| `key` | 渠道类型编号，与 `channeltype/define.go` 中的常量值一致 |
| `text` | 前端显示的渠道名称 |
| `value` | 与 `key` 相同，表单提交值 |
| `color` | 标签颜色，可选值参考 Semantic UI 颜色 |
| `tip` | 提示信息，支持 HTML 格式。用于告诉用户如何获取 API Key 等 |
| `description` | 选项描述（可选） |

---

## 4. 完整适配器实现教程（以百度文心 V1 为例）

当渠道 API 格式与 OpenAI 完全不兼容时，需要实现完整适配器。以下以百度文心千帆 V1（`relay/adaptor/baidu/`）为例，展示完整的适配器实现流程。

### Step 1 — 定义渠道类型和 API 类型常量

**`relay/channeltype/define.go`** — 添加渠道类型常量：

```go
const (
    // ...
    Baidu = 19  // 选择未使用的编号
)
```

**`relay/apitype/define.go`** — 添加 API 类型常量：

```go
const (
    OpenAI = iota
    Anthropic
    PaLM
    Baidu    // 新增
    // ...
)
```

`channeltype` 用于前端展示和路由分发，`apitype` 用于选择适配器实现。两者通过 `channeltype.ToAPIType()` 函数映射。

### Step 2 — 注册渠道元数据

**`relay/channeltype/url.go`** — 添加 Base URL：

```go
var ChannelBaseURLs = []string{
    // ...
    "https://aip.baidubce.com",  // 对应 Baidu 的索引
}
```

**`relay/channeltype/helper.go`** — 添加 channel 到 API type 的映射：

```go
func ToAPIType(channelType int) int {
    switch channelType {
    // ...
    case Baidu:
        apiType = apitype.Baidu
    // ...
    }
}
```

### Step 3 — 创建模型常量文件

`relay/adaptor/baidu/constants.go`：

```go
package baidu

var ModelList = []string{
    "ERNIE-4.0-8K",
    "ERNIE-3.5-8K",
    "ERNIE-Speed-8K",
    "ERNIE-Speed-128K",
    // ...
}
```

### Step 4 — 定义数据结构

`relay/adaptor/baidu/model.go`：

```go
package baidu

import (
    "github.com/songquanpeng/one-api/relay/model"
    "time"
)

// ChatRequest 是发送给百度 API 的请求体
type ChatRequest struct {
    Messages        []Message `json:"messages"`
    Temperature     *float64  `json:"temperature,omitempty"`
    TopP            *float64  `json:"top_p,omitempty"`
    Stream          bool      `json:"stream,omitempty"`
    System          string    `json:"system,omitempty"`
    MaxOutputTokens int       `json:"max_output_tokens,omitempty"`
    // ...
}

type Message struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

// ChatResponse 是百度 API 非流式响应
type ChatResponse struct {
    Id      string      `json:"id"`
    Result  string      `json:"result"`
    Usage   model.Usage `json:"usage"`
    // 错误信息嵌入
    ErrorCode int    `json:"error_code"`
    ErrorMsg  string `json:"error_msg"`
}

// ChatStreamResponse 是百度 API 流式响应
type ChatStreamResponse struct {
    ChatResponse
    SentenceId int  `json:"sentence_id"`
    IsEnd      bool `json:"is_end"`
}
```

### Step 5 — 实现 Adaptor 接口

`relay/adaptor/baidu/adaptor.go`。完整适配器需要实现 `relay/adaptor/interface.go` 中定义的 9 个方法：

```go
package baidu

import (
    "github.com/songquanpeng/one-api/relay/adaptor"
    "github.com/songquanpeng/one-api/relay/model"
    "github.com/songquanpeng/one-api/relay/meta"
    "github.com/songquanpeng/one-api/relay/relaymode"
    "github.com/gin-gonic/gin"
    "io"
    "net/http"
)

type Adaptor struct{}

func (a *Adaptor) Init(meta *meta.Meta) {
    // 初始化逻辑（可选）
}

func (a *Adaptor) GetRequestURL(meta *meta.Meta) (string, error) {
    // 构造请求 URL，包含 access_token 等认证参数
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Request, meta *meta.Meta) error {
    adaptor.SetupCommonRequestHeader(c, req, meta)
    req.Header.Set("Authorization", "Bearer "+meta.APIKey)
    return nil
}

func (a *Adaptor) ConvertRequest(c *gin.Context, relayMode int, request *model.GeneralOpenAIRequest) (any, error) {
    // 将 OpenAI 格式请求转换为渠道格式
    if request == nil {
        return nil, errors.New("request is nil")
    }
    switch relayMode {
    case relaymode.Embeddings:
        return ConvertEmbeddingRequest(*request), nil
    default:
        return ConvertRequest(*request), nil
    }
}

func (a *Adaptor) ConvertImageRequest(request *model.ImageRequest) (any, error) {
    // 图片生成请求转换（如果不支持则返回 request）
    return request, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, meta *meta.Meta, requestBody io.Reader) (*http.Response, error) {
    return adaptor.DoRequestHelper(a, c, meta, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, meta *meta.Meta) (usage *model.Usage, err *model.ErrorWithStatusCode) {
    if meta.IsStream {
        err, usage = StreamHandler(c, resp)
    } else {
        switch meta.Mode {
        case relaymode.Embeddings:
            err, usage = EmbeddingHandler(c, resp)
        default:
            err, usage = Handler(c, resp)
        }
    }
    return
}

func (a *Adaptor) GetModelList() []string {
    return ModelList
}

func (a *Adaptor) GetChannelName() string {
    return "baidu"
}
```

### Step 6 — 在适配器入口注册

编辑 `relay/adaptor.go`，在 `GetAdaptor` 函数中添加 case：

```go
func GetAdaptor(apiType int) adaptor.Adaptor {
    switch apiType {
    // ...
    case apitype.Baidu:
        return &baidu.Adaptor{}
    // ...
    }
    return nil
}
```

### Step 7 — 前端联动

与 OpenAI 兼容渠道相同，在 `web/default/src/constants/channel.constants.js` 中添加前端选项（详见上文 Step 6）。

---

## 5. Adaptor 接口方法实现要点

以下是 `relay/adaptor/interface.go` 中定义的 9 个方法的实现指南：

### 5.1 Init

```go
Init(meta *meta.Meta)
```

在请求处理开始时调用，用于初始化适配器状态。`meta` 中包含请求的所有上下文信息：

| 字段 | 说明 |
|------|------|
| `Mode` | 中继模式，见 `relaymode` 包 |
| `ChannelType` | 渠道类型编号 |
| `BaseURL` | 渠道的 Base URL，优先使用用户配置，否则使用 `ChannelBaseURLs` 中的默认值 |
| `APIKey` | 用户配置的 API Key |
| `APIType` | API 类型，由 `channeltype.ToAPIType()` 计算得到 |
| `IsStream` | 是否为流式请求 |
| `OriginModelName` | 用户请求中的原始模型名称 |
| `ActualModelName` | 经过模型映射后的实际模型名称 |
| `RequestURLPath` | 原始请求 URL 路径 |

对于简单的适配器，此方法可以为空。

### 5.2 GetRequestURL

```go
GetRequestURL(meta *meta.Meta) (string, error)
```

构造发送给渠道的请求 URL。需要处理不同的中继模式：

```go
func (a *Adaptor) GetRequestURL(meta *meta.Meta) (string, error) {
    switch meta.Mode {
    case relaymode.ChatCompletions:
        return fmt.Sprintf("%s/v1/chat/completions", meta.BaseURL), nil
    case relaymode.Embeddings:
        return fmt.Sprintf("%s/v1/embeddings", meta.BaseURL), nil
    case relaymode.ImagesGenerations:
        return fmt.Sprintf("%s/v1/images/generations", meta.BaseURL), nil
    case relaymode.AudioTranscription:
        return fmt.Sprintf("%s/v1/audio/transcriptions", meta.BaseURL), nil
    default:
        return "", fmt.Errorf("unsupported relay mode %d", meta.Mode)
    }
}
```

**常见模式**：

- **标准 OpenAI 兼容**：返回 `meta.BaseURL + meta.RequestURLPath` 即可
- **路径不同**：如 BaiduV2 使用 `/v2/chat/completions` 而非 `/v1/...`
- **需要鉴权参数**：如百度 V1 需要先获取 `access_token` 再拼接到 URL 上
- **模型映射到端点**：如 Azure 需要将模型名映射为 deployment name

### 5.3 SetupRequestHeader

```go
SetupRequestHeader(c *gin.Context, req *http.Request, meta *meta.Meta) error
```

设置 HTTP 请求头。通常先调用 `adaptor.SetupCommonRequestHeader` 设置通用头，再设置鉴权头：

**常见鉴权模式**：

| 模式 | 示例代码 |
|------|----------|
| Bearer Token | `req.Header.Set("Authorization", "Bearer "+meta.APIKey)` |
| API Key Header | `req.Header.Set("api-key", meta.APIKey)` |
| Custom Header | `req.Header.Set("X-API-Key", meta.APIKey)` |
| 无额外鉴权 | 仅调用 `SetupCommonRequestHeader` |

```go
func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Request, meta *meta.Meta) error {
    adaptor.SetupCommonRequestHeader(c, req, meta)
    req.Header.Set("Authorization", "Bearer "+meta.APIKey)
    return nil
}
```

### 5.4 ConvertRequest

```go
ConvertRequest(c *gin.Context, relayMode int, request *model.GeneralOpenAIRequest) (any, error)
```

这是适配器的核心方法。将 OpenAI 格式的通用请求转换为渠道特定的请求格式。

**需要处理的字段**：

| 字段 | 注意事项 |
|------|----------|
| `system` 消息 | 不同渠道处理方式不同。百度放在顶层 `system` 字段，Anthropic 放在 `system` 参数中，大部分渠道直接作为 role=system 的 message |
| `stream_options.include_usage` | OpenAI 兼容渠道建议强制设为 true，以便流式模式中返回用量信息 |
| `reasoning_effort` | 仅 `o1`/`o3` 系列模型支持，如果目标渠道不支持需要忽略或报错 |
| `tools` / `tool_choice` | 函数调用（Function Calling）的转换，需要验证目标渠道是否支持 |
| 多模态内容 | `content` 为数组时包含 `image_url`，需要检查目标渠道是否支持图片输入 |
| `temperature` / `top_p` | 大多数渠道支持，注意默认值和范围可能不同 |
| `max_tokens` / `max_completion_tokens` | 不同渠道字段名可能不同 |

**Baidu V1 示例**（从 OpenAI 格式转换）：

```go
func ConvertRequest(request model.GeneralOpenAIRequest) *ChatRequest {
    baiduRequest := ChatRequest{
        Messages:        make([]Message, 0, len(request.Messages)),
        Temperature:     request.Temperature,
        TopP:            request.TopP,
        Stream:          request.Stream,
        MaxOutputTokens: request.MaxTokens,
        UserId:          request.User,
    }
    for _, message := range request.Messages {
        if message.Role == "system" {
            baiduRequest.System = message.StringContent()
        } else {
            baiduRequest.Messages = append(baiduRequest.Messages, Message{
                Role:    message.Role,
                Content: message.StringContent(),
            })
        }
    }
    return &baiduRequest
}
```

### 5.5 ConvertImageRequest

```go
ConvertImageRequest(request *model.ImageRequest) (any, error)
```

将 OpenAI 图片生成请求转换为渠道格式。如果渠道不支持图片生成，直接原样返回 `request`：

```go
func (a *Adaptor) ConvertImageRequest(request *model.ImageRequest) (any, error) {
    if request == nil {
        return nil, errors.New("request is nil")
    }
    return request, nil  // 不支持，原样返回
}
```

### 5.6 DoRequest

```go
DoRequest(c *gin.Context, meta *meta.Meta, requestBody io.Reader) (*http.Response, error)
```

发送 HTTP 请求到渠道。大多数情况下可以直接复用 `adaptor.DoRequestHelper`：

```go
func (a *Adaptor) DoRequest(c *gin.Context, meta *meta.Meta, requestBody io.Reader) (*http.Response, error) {
    return adaptor.DoRequestHelper(a, c, meta, requestBody)
}
```

如果需要自定义请求行为（如 WebSocket 协议、gRPC 等），在此方法中实现自定义逻辑。

### 5.7 DoResponse

```go
DoResponse(c *gin.Context, resp *http.Response, meta *meta.Meta) (usage *model.Usage, err *model.ErrorWithStatusCode)
```

处理渠道返回的响应，将其转换为 OpenAI 标准格式，并提取用量信息。

**实现要点**：

1. **始终区分流式和非流式**：

```go
func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, meta *meta.Meta) (usage *model.Usage, err *model.ErrorWithStatusCode) {
    if meta.IsStream {
        err, usage = StreamHandler(c, resp)
    } else {
        err, usage = Handler(c, resp)
    }
    return
}
```

2. **非流式 Handler 示例**（百度 V1）：

```go
func Handler(c *gin.Context, resp *http.Response) (*model.ErrorWithStatusCode, *model.Usage) {
    // 1. 读取响应体
    responseBody, err := io.ReadAll(resp.Body)
    if err != nil { /* 返回错误 */ }
    defer resp.Body.Close()

    // 2. 解析为渠道响应格式
    var baiduResponse ChatResponse
    err = json.Unmarshal(responseBody, &baiduResponse)
    if err != nil { /* 返回错误 */ }

    // 3. 检查渠道错误
    if baiduResponse.ErrorMsg != "" {
        return &model.ErrorWithStatusCode{
            Error: model.Error{
                Message: baiduResponse.ErrorMsg,
                Type:    "baidu_error",
                Code:    baiduResponse.ErrorCode,
            },
            StatusCode: resp.StatusCode,
        }, nil
    }

    // 4. 转换为 OpenAI 格式并写入响应
    fullTextResponse := responseBaidu2OpenAI(&baiduResponse)
    jsonResponse, _ := json.Marshal(fullTextResponse)
    c.Writer.Header().Set("Content-Type", "application/json")
    c.Writer.WriteHeader(resp.StatusCode)
    c.Writer.Write(jsonResponse)

    // 5. 返回用量信息
    return nil, &fullTextResponse.Usage
}
```

3. **流式 Handler 示例**（百度 V1）：

```go
func StreamHandler(c *gin.Context, resp *http.Response) (*model.ErrorWithStatusCode, *model.Usage) {
    var usage model.Usage
    scanner := bufio.NewScanner(resp.Body)
    scanner.Split(bufio.ScanLines)

    common.SetEventStreamHeaders(c)

    for scanner.Scan() {
        data := scanner.Text()
        // 解析 SSE 数据行
        // 将渠道流式块转换为 OpenAI 流式格式
        // 提取用量信息（通常在最后一个块中）
        response := streamResponseBaidu2OpenAI(&baiduResponse)
        render.ObjectData(c, response)
    }

    render.Done(c)
    return nil, &usage
}
```

4. **OpenAI 兼容渠道**的 DoResponse 已经在 `openai.Adaptor` 中实现，直接复用即可：

```go
func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, meta *meta.Meta) (usage *model.Usage, err *model.ErrorWithStatusCode) {
    if meta.IsStream {
        err, responseText, usage = StreamHandler(c, resp, meta.Mode)
        // 回退：如果渠道未返回用量，从响应文本估算
        if usage == nil || usage.TotalTokens == 0 {
            usage = ResponseText2Usage(responseText, meta.ActualModelName, meta.PromptTokens)
        }
    } else {
        err, usage = Handler(c, resp, meta.PromptTokens, meta.ActualModelName)
    }
    return
}
```

### 5.8 GetModelList

```go
GetModelList() []string
```

返回该渠道支持的模型列表。对于 OpenAI 兼容渠道，`openai.Adaptor` 已经通过 `GetCompatibleChannelMeta` 自动获取。

```go
func (a *Adaptor) GetModelList() []string {
    return ModelList
}
```

### 5.9 GetChannelName

```go
GetChannelName() string
```

返回渠道名称，用于日志和显示。对于 OpenAI 兼容渠道，`openai.Adaptor` 已经通过 `GetCompatibleChannelMeta` 自动获取。

```go
func (a *Adaptor) GetChannelName() string {
    return "baidu"
}
```

---

## 6. 响应转换参考

### 非流式响应转换

将渠道响应映射为 OpenAI 标准 `TextResponse` 结构：

```go
// OpenAI 标准响应结构
type TextResponse struct {
    Id      string               `json:"id"`
    Object  string               `json:"object"`
    Created int64                `json:"created"`
    Model   string               `json:"model,omitempty"`
    Choices []TextResponseChoice `json:"choices"`
    Usage   model.Usage          `json:"usage"`
}

type TextResponseChoice struct {
    Index        int           `json:"index"`
    Message      model.Message `json:"message"`
    FinishReason string        `json:"finish_reason"`
}
```

示例转换函数（百度 V1）：

```go
func responseBaidu2OpenAI(response *ChatResponse) *openai.TextResponse {
    choice := openai.TextResponseChoice{
        Index: 0,
        Message: model.Message{
            Role:    "assistant",
            Content: response.Result,
        },
        FinishReason: "stop",
    }
    fullTextResponse := openai.TextResponse{
        Id:      response.Id,
        Object:  "chat.completion",
        Created: response.Created,
        Choices: []openai.TextResponseChoice{choice},
        Usage:   response.Usage,
    }
    return &fullTextResponse
}
```

### 流式响应转换

将渠道流式块映射为 OpenAI 标准 `ChatCompletionsStreamResponse` 结构：

```go
// OpenAI 标准流式块结构
type ChatCompletionsStreamResponse struct {
    Id      string                                `json:"id"`
    Object  string                                `json:"object"`
    Created int64                                 `json:"created"`
    Model   string                                `json:"model"`
    Choices []ChatCompletionsStreamResponseChoice `json:"choices"`
    Usage   *model.Usage                          `json:"usage,omitempty"`
}

type ChatCompletionsStreamResponseChoice struct {
    Index        int           `json:"index"`
    Delta        model.Message `json:"delta"`
    FinishReason *string       `json:"finish_reason,omitempty"`
}
```

示例转换函数（百度 V1）：

```go
func streamResponseBaidu2OpenAI(baiduResponse *ChatStreamResponse) *openai.ChatCompletionsStreamResponse {
    var choice openai.ChatCompletionsStreamResponseChoice
    choice.Delta.Content = baiduResponse.Result
    if baiduResponse.IsEnd {
        choice.FinishReason = &constant.StopFinishReason
    }
    response := openai.ChatCompletionsStreamResponse{
        Id:      baiduResponse.Id,
        Object:  "chat.completion.chunk",
        Created: baiduResponse.Created,
        Model:   "ernie-bot",
        Choices: []openai.ChatCompletionsStreamResponseChoice{choice},
    }
    return &response
}
```

---

## 7. 测试清单

实现完成后，逐项验证：

| # | 测试项 | 验证方法 | 预期结果 |
|---|--------|----------|----------|
| 1 | 非流式请求 | 调用聊天接口，`stream=false` | 返回正确的 `choices[0].message.content` |
| 2 | 流式请求 | 调用聊天接口，`stream=true` | SSE 逐块正确转发，以 `data: [DONE]` 结束 |
| 3 | 错误响应 | 使用无效 API Key 请求 | 返回标准 OpenAI 错误格式 `{"error": {"message": "...", "type": "..."}}` |
| 4 | 用量提取 | 检查非流式响应中的 `usage` 字段 | `prompt_tokens` + `completion_tokens` = `total_tokens`，值合理 |
| 5 | 流式用量 | 流式请求，检查最后一块是否包含 `usage` 字段 | 用量信息在最后一个 chunk 中返回 |
| 6 | 模型列表 | 调用模型列表接口 | `GetModelList()` 返回正确的模型名列表 |
| 7 | 渠道测试 | Web UI 的渠道"测试"按钮 | 能成功测试，返回正常结果 |
| 8 | 多模态 | 如果支持，发送含 `image_url` 的请求 | 图片正确传递和处理 |

---

## 8. 提交 PR 前 Checklist

```
- [ ] 后端: relay/adaptor/<provider>/ 完整实现
      - [ ] constants.go（模型列表）
      - [ ] main.go（核心转换逻辑）
      - [ ] adaptor.go（自定义 Adaptor 结构体，完整适配器需要）
- [ ] 后端: relay/channeltype/define.go 添加常量
- [ ] 后端: relay/channeltype/url.go 添加 Base URL
- [ ] 后端: relay/channeltype/helper.go 添加 ToAPIType 映射（完整适配器需要）
- [ ] 后端: relay/apitype/define.go 添加 API 类型常量（完整适配器需要）
- [ ] 后端: relay/adaptor.go 添加 GetAdaptor 映射（完整适配器需要）
- [ ] 后端: relay/adaptor/openai/compatible.go 添加：
      - [ ] CompatibleChannels 列表
      - [ ] GetCompatibleChannelMeta switch case
      - [ ] GetRequestURL 路由（如果需要自定义 URL）
- [ ] 前端: web/default/src/constants/channel.constants.js 添加选项
- [ ] 通过上述测试清单所有项
- [ ] `go build` / `go test ./...` 编译通过
- [ ] PR 描述完整（变更内容 + 测试方式）
```

---

## 附录：关键文件索引

| 文件 | 用途 |
|------|------|
| `relay/adaptor/interface.go` | Adaptor 接口定义（9 个方法） |
| `relay/channeltype/define.go` | 渠道类型常量定义 |
| `relay/channeltype/url.go` | 渠道默认 Base URL 映射表 |
| `relay/channeltype/helper.go` | 渠道类型到 API 类型的转换 |
| `relay/apitype/define.go` | API 类型常量定义（用于选择适配器） |
| `relay/adaptor.go` | 全局适配器注册表（API 类型 -> Adaptor 实例） |
| `relay/adaptor/openai/compatible.go` | OpenAI 兼容渠道注册表 |
| `relay/adaptor/openai/adaptor.go` | OpenAI 适配器（兼容渠道的统一处理入口） |
| `relay/adaptor/openai/main.go` | OpenAI 标准 StreamHandler / Handler |
| `relay/adaptor/openai/model.go` | OpenAI 标准响应数据结构 |
| `relay/model/general.go` | OpenAI 通用请求结构 `GeneralOpenAIRequest` |
| `relay/model/misc.go` | `Usage`、`Error`、`ErrorWithStatusCode` 等公用结构 |
| `relay/relaymode/define.go` | 中继模式常量 |
| `relay/meta/relay_meta.go` | Meta 数据结构定义 |
| `web/default/src/constants/channel.constants.js` | 前端渠道选项 |
