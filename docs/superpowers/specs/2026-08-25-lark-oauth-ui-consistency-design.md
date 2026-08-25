# 飞书 OAuth 前端状态与回调地址一致性设计

日期：2026-08-25

## 背景

One API 支持 default、berry、air 三个前端主题。管理员配置飞书 App ID、App Secret 并启用飞书 OAuth 后，`GET /api/status` 会正确返回：

```json
{
  "lark_client_id": "cli_xxx",
  "lark_oauth_enabled": true,
  "server_address": "http://192.168.0.86:3000"
}
```

当前实现仍存在两类用户可见问题：

1. default 和 air 登录页仅在首次挂载时从 `localStorage.status` 读取认证配置，而 `/api/status` 在其他组件中异步加载。登录页可能先读到旧值或空值，导致飞书入口必须再次刷新页面才出现。
2. 飞书授权请求使用 `window.location.origin` 生成 `redirect_uri`，后端换取 access token 时使用 `config.ServerAddress` 生成 `redirect_uri`。当用户通过与 `ServerAddress` 不同的地址访问系统时，两次地址不一致，飞书返回 `The provided redirect URI does not match the one used during authorization.`。

此外，default 和 air 首页的“系统配置”总览展示邮箱、GitHub、微信与 Turnstile 状态，但没有展示飞书 OAuth 状态。

## 目标

- 飞书 OAuth 启用且 App ID 有效时，三个主题在状态加载完成后立即显示飞书登录入口，无需二次刷新。
- 飞书 OAuth 未启用时，三个主题均不显示飞书登录入口。
- 发起飞书授权前验证当前页面来源与系统规范地址一致，避免进入必然失败的授权流程。
- default 和 air 首页的系统配置总览展示飞书 OAuth 启用状态。
- 保持现有后端飞书 OAuth 接口格式、用户创建、账号绑定和 State 校验流程不变；仅统一回调地址的归一化规则。

## 非目标

- 不根据请求 Host、容器 IP、网卡地址或代理头自动修改 `ServerAddress`。
- 不新增反向代理发现、可信代理列表或多域名 OAuth 支持。
- 不修改飞书应用后台配置。
- 不移除 berry 的 Service Worker 或 PWA 能力。
- 不为 berry 新增一整套首页系统配置卡片；berry 当前没有与 default、air 对应的同类卡片。
- 不重构 GitHub、OIDC 或微信 OAuth 流程。

## 设计原则

### `ServerAddress` 是规范外部地址

服务可能运行在 Docker、反向代理、NAT、内网穿透或多域名环境中。后端观察到的 Host 或 IP 不一定是用户实际应访问的外部地址，因此系统继续使用管理员配置的 `ServerAddress` 作为规范外部地址。

管理员必须同时保证：

```text
浏览器访问来源 = ServerAddress
飞书后台重定向 URL = ServerAddress + /oauth/lark
```

本次修改负责在前端提前发现不一致并给出明确提示，不尝试猜测或修正部署地址。

### 运行时状态以 `/api/status` 为准

`localStorage` 只保留缓存用途，不能作为登录页认证能力的唯一响应式数据源。登录页应订阅主题已有的全局状态容器，由 `/api/status` 成功响应驱动重新渲染。

## 组件设计

### default 主题

`web/default/src/App.js` 已加载 `/api/status` 并写入 `StatusContext`。`LoginForm` 改为直接订阅该 Context，不再维护一份仅初始化一次的 `status` 本地状态。

飞书入口显示条件为：

```text
status.lark_oauth_enabled === true
并且 status.lark_client_id 非空
```

首页“系统配置”卡片增加飞书 OAuth 状态行。文案通过现有 i18n 体系提供中文、英文和日文翻译。

### air 主题

`web/air/src/components/SiderBar.js` 已加载 `/api/status` 并写入 `StatusContext`。`LoginForm` 改为订阅该 Context，不再仅在挂载时读取 `localStorage.status`。

飞书入口采用与 default 相同的双条件判断。首页“系统配置”卡片增加“飞书身份验证：已启用/未启用”。

### berry 主题

berry 的 `StatusProvider` 已在 `/api/status` 返回后更新 Redux `siteInfo`，登录页具备响应式更新能力。本次补齐 `config.siteInfo` 的默认字段：

```js
lark_client_id: '',
lark_oauth_enabled: false,
```

登录入口和账号绑定入口统一使用以下条件：

```text
siteInfo.lark_oauth_enabled === true
并且 siteInfo.lark_client_id 非空
```

这可以避免仅配置 App ID、但管理员未启用飞书 OAuth 时仍展示入口。

## OAuth 地址一致性检查

三个主题的飞书授权工具函数接收当前全局状态中的 `server_address`。点击飞书登录或绑定时执行以下流程：

1. 将 `server_address` 解析为 URL。仅接受 HTTP 或 HTTPS、无用户名密码、无查询参数和片段、路径为空或仅为 `/` 的绝对来源地址。
2. 去除末尾斜杠，得到规范来源地址。
3. 将规范来源地址与 `window.location.origin` 做严格比较。
4. 若不一致，停止流程并显示错误提示。
5. 若一致，再获取 OAuth State；现有后端 Session 和 CSRF 防护流程保持不变。
6. 以规范来源地址构造回调地址：`{server_address}/oauth/lark`。
7. 使用 `URLSearchParams` 或等价 URL 编码方式构造飞书授权参数，避免手工拼接未编码参数。

后端换取 token 时通过一个单一用途的回调地址构造函数去除 `config.ServerAddress` 的末尾斜杠，再追加 `/oauth/lark`。前后端因此对合法 `ServerAddress` 使用完全相同的回调地址。该调整不改变路由、请求或响应格式。

建议的错误提示包含当前地址和期望地址，例如：

```text
当前访问地址 http://192.168.0.20:3000 与系统服务器地址
http://192.168.0.86:3000 不一致。请通过系统服务器地址访问，
或由管理员修改 ServerAddress 后重试。
```

检查必须发生在打开飞书授权页面之前。地址不一致时不调用 `window.open` 或修改 `window.location`。

### 为什么不自动使用当前 IP

自动采用 `window.location.origin` 虽能匹配当前浏览器地址，却仍会与后端使用的 `config.ServerAddress` 不一致。后端若反过来信任任意 Host 或转发头，又会在反向代理、多域名和 Host 伪造场景中引入不确定性。

显式规范地址加前端预检具有可预测性，也能保证 OAuth State Cookie 与回调页面处于同一来源。

## 数据流

```text
页面启动
  → GET /api/status
  → 更新 StatusContext 或 Redux siteInfo
  → 登录页响应式重新渲染
  → lark_oauth_enabled && lark_client_id
  → 显示飞书入口

点击飞书入口
  → 校验并归一化 server_address
  → 比较 window.location.origin 与 server_address
  → 不一致：显示错误并终止
  → 一致：GET /api/oauth/state
  → 构造 server_address/oauth/lark
  → 跳转飞书授权页面
  → 飞书回调 /oauth/lark
  → GET /api/oauth/lark?code=...&state=...
  → 后端使用相同 ServerAddress/oauth/lark 换取 token
  → 登录或绑定成功
```

## 错误处理

- `/api/status` 加载失败：沿用各主题现有错误提示和缓存回退行为。若主题回退到缓存状态，点击飞书入口时仍必须通过地址检查和 OAuth State 接口，不能绕过服务端校验。
- OAuth State 获取失败：沿用现有错误提示，不继续授权跳转。
- `server_address` 为空或不是有效绝对来源地址：显示“系统服务器地址配置无效”，不继续授权。
- 当前来源与规范来源不一致：显示包含两者的可操作提示，不继续授权。
- 飞书授权或 token 交换失败：沿用后端当前错误响应和前端消息展示。

错误信息不得包含 App Secret、授权码、access token 或 Session 内容。

## 兼容性

- `ServerAddress` 与当前访问来源一致的现有部署行为不变。
- 已正确配置的飞书应用无需修改。
- default 和 air 从本地缓存状态切换为全局响应式状态后，GitHub 与微信入口也会同步受益，不再依赖二次刷新；其授权协议不做修改。
- berry 仍保留现有 Redux、Service Worker 和主题结构。
- 后端 API 响应格式及数据库选项保持不变；仅将飞书 token 交换使用的回调地址改为统一归一化后的值。

## 预计修改范围

### 后端

- `controller/auth/lark.go`
- 飞书回调地址构造函数对应的 Go 测试文件

### default

- `web/default/src/components/LoginForm.js`
- `web/default/src/components/utils.js`
- `web/default/src/pages/Home/index.js`
- `web/default/src/locales/zh/translation.json`
- `web/default/src/locales/en/translation.json`
- `web/default/src/locales/ja/translation.json`
- 其他调用 `onLarkOAuthClicked` 的 default 组件（按调用点同步函数参数）

### air

- `web/air/src/components/LoginForm.js`
- `web/air/src/components/utils.js`
- `web/air/src/pages/Home/index.js`
- 其他调用 `onLarkOAuthClicked` 的 air 组件（按调用点同步函数参数）

### berry

- `web/berry/src/config.js`
- `web/berry/src/utils/common.js`
- `web/berry/src/views/Authentication/AuthForms/AuthLogin.js`
- `web/berry/src/views/Profile/index.js`
- 其他调用 `onLarkOAuthClicked` 的 berry 组件（按调用点同步函数参数）

## 测试与验收

### 状态矩阵

三个主题分别验证：

| `lark_oauth_enabled` | `lark_client_id` | 预期结果 |
| --- | --- | --- |
| `false` | 空 | 不显示飞书入口 |
| `false` | 非空 | 不显示飞书入口 |
| `true` | 空 | 不显示飞书入口 |
| `true` | 非空 | 状态加载完成后立即显示飞书入口 |

### 地址矩阵

| 当前来源 | `ServerAddress` | 预期结果 |
| --- | --- | --- |
| 完全一致 | 有效地址 | 进入飞书授权，回调地址为 `{ServerAddress}/oauth/lark` |
| IP 或域名不同 | 有效地址 | 显示地址不一致提示，不跳转 |
| 端口不同 | 有效地址 | 显示地址不一致提示，不跳转 |
| HTTP/HTTPS 不同 | 有效地址 | 显示地址不一致提示，不跳转 |
| 任意地址 | 空或无效地址 | 显示配置无效提示，不跳转 |
| 完全一致 | 带一个或多个末尾 `/` | 归一化为单一 `{origin}/oauth/lark` 后进入授权 |

### 自动化测试

- 前端回调地址辅助函数覆盖：合法 HTTP/HTTPS 来源、末尾斜杠、无效 URL、带路径、带用户信息、地址不一致和正确参数编码。
- 后端回调地址构造函数覆盖：无末尾斜杠、一个末尾斜杠和多个末尾斜杠。
- berry 的显示条件覆盖启用状态与 App ID 的四种组合；default 和 air 通过相同条件函数或组件测试覆盖对应矩阵。

### 页面验收

- default 与 air 首页显示飞书 OAuth 的已启用/未启用状态。
- default 的中、英、日文案均能正确渲染。
- 从飞书登录回调后，已有账户可以登录，新账户在允许注册时可以创建。
- 已登录用户的飞书绑定流程仍可使用。
- 错误提示中不泄露敏感信息。

### 构建验收

- 依次完成 default、berry、air 三个主题的生产构建。
- 构建产物聚合至 `web/build/<theme>`，可被 `main.go` 的 `//go:embed web/build/*` 收录。
- 运行后通过三个主题手工执行状态矩阵中的关键路径和一次完整飞书登录。

## 完成条件

满足以下全部条件才视为完成：

1. 三个主题的飞书入口只在启用且 App ID 非空时显示。
2. default 和 air 首次打开登录页时，入口能在状态响应返回后出现，无需再次刷新。
3. 地址不一致时，在跳转飞书前显示明确提示。
4. 地址一致时可以完成飞书授权、回调与登录。
5. default 和 air 首页展示飞书认证状态。
6. 三个主题生产构建成功，构建产物可被 Go 二进制嵌入。
