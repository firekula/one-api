# 飞书 OAuth 界面一致性实施计划

> **供智能体执行者使用：** 必须使用子技能 `superpowers:subagent-driven-development`（推荐）或 `superpowers:executing-plans`，逐任务实施本计划。各步骤使用复选框（`- [ ]`）跟踪进度。

**目标：** 让三个主题中的飞书登录与绑定入口随运行时状态即时显示，在现有系统总览中展示飞书状态，并在授权前拒绝来源地址不匹配的 OAuth 请求。

**架构：** 继续以 `ServerAddress` 作为规范公开来源地址。由于 CRA 不允许从相邻主题源码根目录导入文件，因此在每个主题内分别增加一个小型纯函数飞书 OAuth 辅助模块；三个主题使用相同接口，同时保留各自 UI 组件库。Go 后端在交换 token 前使用同一规则归一化回调地址，default 与 air 则改为消费现有 `StatusContext`，不再依赖一次性的 `localStorage` 快照。

**技术栈：** Go 1.20+、Gin、React 18、CRA/react-scripts 5、Jest、React Context、Redux、Semantic UI、Semi UI、Material UI。

**设计文档：** `docs/superpowers/specs/2026-08-25-lark-oauth-ui-consistency-design.md`

## 全局约束

- `ServerAddress` 保持为管理员配置的规范外部来源地址；不得从 Docker、网卡、请求 Host 或代理头推断。
- 只接受不含凭据、查询参数、片段或非根路径的 HTTP/HTTPS 绝对来源地址。
- 追加 `/oauth/lark` 前，归一化一个或多个末尾 `/`。
- 请求 OAuth State 前，先比较归一化后的规范来源与 `window.location.origin`。
- 错误消息不得包含 App Secret、授权码、access token 或 Session 内容。
- 不修改后端路由名、JSON 响应结构、用户创建、账号绑定或 OAuth State 校验。
- 不移除 berry 的 Service Worker，也不新增 berry 系统状态卡片。
- 保留三个主题现有的 UI 组件库和视觉风格。

---

## 文件结构

### 后端

- 修改 `controller/auth/lark.go`：集中构造规范飞书回调 URI，并在 token 请求中使用。
- 新建 `controller/auth/lark_test.go`：用表格测试覆盖回调 URI 归一化。

### default 主题

- 新建 `web/default/src/components/larkOAuth.js`：提供可用性判断、回调校验和授权 URL 构造纯函数。
- 新建 `web/default/src/components/larkOAuth.test.js`：测试辅助函数行为和状态矩阵。
- 修改 `web/default/src/components/utils.js`：编排地址校验、State 获取和窗口跳转。
- 修改 `web/default/src/components/LoginForm.js`：消费响应式 `StatusContext` 和规范地址。
- 修改 `web/default/src/components/PersonalSetting.js`：绑定入口消费响应式 `StatusContext`。
- 修改 `web/default/src/pages/Home/index.js`：在现有系统总览中渲染飞书状态。
- 修改 `web/default/src/locales/zh/translation.json`：增加中文总览标签。
- 修改 `web/default/src/locales/en/translation.json`：增加英文总览标签。

### air 主题

- 新建 `web/air/src/components/larkOAuth.js`：提供可用性判断、回调校验和授权 URL 构造纯函数。
- 新建 `web/air/src/components/larkOAuth.test.js`：测试辅助函数行为和状态矩阵。
- 修改 `web/air/src/components/utils.js`：编排地址校验、State 获取和窗口跳转。
- 修改 `web/air/src/components/LoginForm.js`：消费响应式 `StatusContext` 和规范地址。
- 修改 `web/air/src/components/PersonalSetting.js`：绑定入口消费响应式 `StatusContext`，且状态更新时不重复获取用户信息。
- 修改 `web/air/src/pages/Home/index.js`：在现有系统总览中渲染飞书状态。

### berry 主题

- 新建 `web/berry/src/utils/larkOAuth.js`：提供可用性判断、回调校验和授权 URL 构造纯函数。
- 新建 `web/berry/src/utils/larkOAuth.test.js`：测试辅助函数行为和状态矩阵。
- 修改 `web/berry/src/utils/common.js`：编排地址校验、State 获取和窗口跳转。
- 修改 `web/berry/src/config.js`：在 Redux 站点信息初始值中声明飞书字段。
- 修改 `web/berry/src/views/Authentication/AuthForms/AuthLogin.js`：使用“已启用且 Client ID 非空”的可用性规则。
- 修改 `web/berry/src/views/Profile/index.js`：绑定入口使用相同可用性规则。

---

### 任务 1：后端规范回调 URI

**文件：**
- 修改：`controller/auth/lark.go`
- 新建：`controller/auth/lark_test.go`

**接口：**
- 输入：`config.ServerAddress string`
- 输出：`func getLarkRedirectURI(serverAddress string) string`
- 输出：作为 `redirect_uri` 发送至 `https://accounts.feishu.cn/oauth/v3/token` 的精确 `{trimmed-origin}/oauth/lark` 值

- [ ] **步骤 1：编写失败的归一化测试**

新建 `controller/auth/lark_test.go`：

```go
package auth

import "testing"

func TestGetLarkRedirectURI(t *testing.T) {
	tests := []struct {
		name          string
		serverAddress string
		want          string
	}{
		{name: "no trailing slash", serverAddress: "http://192.168.0.86:3000", want: "http://192.168.0.86:3000/oauth/lark"},
		{name: "one trailing slash", serverAddress: "https://api.example.com/", want: "https://api.example.com/oauth/lark"},
		{name: "multiple trailing slashes", serverAddress: "https://api.example.com///", want: "https://api.example.com/oauth/lark"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getLarkRedirectURI(tt.serverAddress); got != tt.want {
				t.Fatalf("getLarkRedirectURI(%q) = %q, want %q", tt.serverAddress, got, tt.want)
			}
		})
	}
}
```

- [ ] **步骤 2：运行测试并确认失败**

从仓库根目录运行：

```powershell
go test ./controller/auth -run TestGetLarkRedirectURI -count=1
```

预期：编译失败并提示 `undefined: getLarkRedirectURI`。

- [ ] **步骤 3：实现最小辅助函数并接入**

在 `controller/auth/lark.go` 中导入 `strings`，并增加：

```go
func getLarkRedirectURI(serverAddress string) string {
	return strings.TrimRight(serverAddress, "/") + "/oauth/lark"
}
```

将 token 请求中的值替换为：

```go
"redirect_uri": getLarkRedirectURI(config.ServerAddress),
```

- [ ] **步骤 4：格式化并运行聚焦测试**

```powershell
gofmt -w controller/auth/lark.go controller/auth/lark_test.go
go test ./controller/auth -run TestGetLarkRedirectURI -count=1
```

预期：输出 `ok github.com/songquanpeng/one-api/controller/auth`。

- [ ] **步骤 5：运行完整 Go 测试套件**

```powershell
go test ./...
```

预期：所有包均通过测试。

- [ ] **步骤 6：提交后端单元**

```powershell
git add controller/auth/lark.go controller/auth/lark_test.go
git commit -m "fix: normalize lark oauth redirect uri"
```

---

### 任务 2：default 主题 OAuth 辅助模块

**文件：**
- 新建：`web/default/src/components/larkOAuth.js`
- 新建：`web/default/src/components/larkOAuth.test.js`
- 修改：`web/default/src/components/utils.js`

**接口：**
- 输出：`isLarkOAuthAvailable(status: object | null | undefined): boolean`
- 输出：`getLarkRedirectUri(serverAddress: string, currentOrigin: string): string`
- 输出：`buildLarkAuthorizeUrl(clientId: string, redirectUri: string, state: string): string`
- 输出：`onLarkOAuthClicked(larkClientId: string, serverAddress: string): Promise<void>`

- [ ] **步骤 1：编写失败的纯函数测试**

新建 `web/default/src/components/larkOAuth.test.js`：

```js
import {
  buildLarkAuthorizeUrl,
  getLarkRedirectUri,
  isLarkOAuthAvailable,
} from './larkOAuth';

describe('isLarkOAuthAvailable', () => {
  test.each([
    [{ lark_oauth_enabled: false, lark_client_id: '' }, false],
    [{ lark_oauth_enabled: false, lark_client_id: 'cli_test' }, false],
    [{ lark_oauth_enabled: true, lark_client_id: '' }, false],
    [{ lark_oauth_enabled: true, lark_client_id: 'cli_test' }, true],
    [undefined, false],
  ])('returns the expected availability for %p', (status, expected) => {
    expect(isLarkOAuthAvailable(status)).toBe(expected);
  });
});

describe('getLarkRedirectUri', () => {
  test('normalizes trailing slashes', () => {
    expect(
      getLarkRedirectUri(
        'http://192.168.0.86:3000///',
        'http://192.168.0.86:3000'
      )
    ).toBe('http://192.168.0.86:3000/oauth/lark');
  });

  test.each([
    ['not-a-url'],
    ['ftp://example.com'],
    ['https://user:pass@example.com'],
    ['https://example.com/base'],
    ['https://example.com/?query=1'],
    ['https://example.com/#fragment'],
  ])('rejects invalid server address %s', (serverAddress) => {
    expect(() =>
      getLarkRedirectUri(serverAddress, 'https://example.com')
    ).toThrow('系统服务器地址配置无效');
  });

  test('rejects a different browser origin', () => {
    expect(() =>
      getLarkRedirectUri('https://api.example.com', 'https://alias.example.com')
    ).toThrow('当前访问地址 https://alias.example.com 与系统服务器地址 https://api.example.com 不一致');
  });
});

test('buildLarkAuthorizeUrl encodes all OAuth parameters', () => {
  const url = new URL(
    buildLarkAuthorizeUrl(
      'cli_test',
      'https://api.example.com/oauth/lark',
      'state with spaces'
    )
  );
  expect(url.origin + url.pathname).toBe(
    'https://accounts.feishu.cn/open-apis/authen/v1/authorize'
  );
  expect(url.searchParams.get('client_id')).toBe('cli_test');
  expect(url.searchParams.get('response_type')).toBe('code');
  expect(url.searchParams.get('redirect_uri')).toBe(
    'https://api.example.com/oauth/lark'
  );
  expect(url.searchParams.get('state')).toBe('state with spaces');
});
```

- [ ] **步骤 2：安装 default 主题依赖**

```powershell
Set-Location web/default
npm install --legacy-peer-deps --no-package-lock
```

预期：安装成功，且不产生被 Git 跟踪的锁文件。

- [ ] **步骤 3：运行测试并确认失败**

```powershell
Set-Location web/default
npm test -- --watchAll=false src/components/larkOAuth.test.js
```

预期：测试失败，因为 `./larkOAuth` 尚不存在。

- [ ] **步骤 4：实现纯函数辅助模块**

新建 `web/default/src/components/larkOAuth.js`：

```js
const LARK_AUTHORIZE_ENDPOINT =
  'https://accounts.feishu.cn/open-apis/authen/v1/authorize';

const invalidServerAddress = () =>
  new Error(
    '系统服务器地址配置无效，请在系统设置中填写仅包含协议、主机和端口的 HTTP/HTTPS 地址。'
  );

export function isLarkOAuthAvailable(status) {
  return Boolean(status?.lark_oauth_enabled && status?.lark_client_id);
}

export function getLarkRedirectUri(serverAddress, currentOrigin) {
  if (typeof serverAddress !== 'string' || serverAddress.trim() === '') {
    throw invalidServerAddress();
  }

  let parsed;
  try {
    parsed = new URL(serverAddress.trim());
  } catch {
    throw invalidServerAddress();
  }

  if (
    !['http:', 'https:'].includes(parsed.protocol) ||
    parsed.username ||
    parsed.password ||
    !/^\/*$/.test(parsed.pathname) ||
    parsed.search ||
    parsed.hash
  ) {
    throw invalidServerAddress();
  }

  const canonicalOrigin = parsed.origin;
  if (canonicalOrigin !== currentOrigin) {
    throw new Error(
      `当前访问地址 ${currentOrigin} 与系统服务器地址 ${canonicalOrigin} 不一致。请通过系统服务器地址访问，或由管理员修改 ServerAddress 后重试。`
    );
  }
  return `${canonicalOrigin}/oauth/lark`;
}

export function buildLarkAuthorizeUrl(clientId, redirectUri, state) {
  const params = new URLSearchParams({
    client_id: clientId,
    response_type: 'code',
    redirect_uri: redirectUri,
    state,
  });
  return `${LARK_AUTHORIZE_ENDPOINT}?${params.toString()}`;
}
```

- [ ] **步骤 5：运行辅助函数测试**

```powershell
npm test -- --watchAll=false src/components/larkOAuth.test.js
```

预期：所有测试通过。

- [ ] **步骤 6：在获取 OAuth State 前接入地址校验**

在 `web/default/src/components/utils.js` 中导入辅助函数，并将飞书函数替换为：

```js
import {
  buildLarkAuthorizeUrl,
  getLarkRedirectUri,
} from './larkOAuth';

export async function onLarkOAuthClicked(larkClientId, serverAddress) {
  let redirectUri;
  try {
    redirectUri = getLarkRedirectUri(serverAddress, window.location.origin);
  } catch (error) {
    showError(error.message);
    return;
  }

  const state = await getOAuthState();
  if (!state) return;
  window.open(buildLarkAuthorizeUrl(larkClientId, redirectUri, state));
}
```

保持现有 GitHub 和 State 函数不变。

- [ ] **步骤 7：重新运行测试并提交辅助模块单元**

```powershell
npm test -- --watchAll=false src/components/larkOAuth.test.js
Set-Location ../..
git add web/default/src/components/larkOAuth.js web/default/src/components/larkOAuth.test.js web/default/src/components/utils.js
git commit -m "feat: validate default lark oauth origin"
```

预期：测试通过，提交中只包含 default 辅助模块单元。

---

### 任务 3：default 主题响应式界面与状态总览

**文件：**
- 修改：`web/default/src/components/LoginForm.js`
- 修改：`web/default/src/components/PersonalSetting.js`
- 修改：`web/default/src/pages/Home/index.js`
- 修改：`web/default/src/locales/zh/translation.json`
- 修改：`web/default/src/locales/en/translation.json`

**接口：**
- 输入：结构为 `[state, dispatch]` 的 `StatusContext`，其中 `state.status` 包含 `/api/status` 数据
- 输入：任务 2 提供的 `isLarkOAuthAvailable(status)` 和 `onLarkOAuthClicked(clientId, serverAddress)`
- 输出：响应式登录/绑定入口可见性，以及本地化首页状态行

- [ ] **步骤 1：将 LoginForm 改为响应式状态**

在 `LoginForm.js` 中：

```js
import { StatusContext } from '../context/Status';
import { isLarkOAuthAvailable } from './larkOAuth';
```

将本地状态和一次性 `localStorage` 读取替换为：

```js
const [statusState] = useContext(StatusContext);
const status = statusState?.status || {};
```

保留登录过期处理 effect，但移除其中的 `localStorage.getItem('status')`、JSON 解析和 `setStatus` 调用。第三方登录区域和飞书按钮都改用 `isLarkOAuthAvailable(status)` 判断，并调用：

```js
onLarkOAuthClicked(status.lark_client_id, status.server_address)
```

- [ ] **步骤 2：将 PersonalSetting 改为响应式状态**

导入 `StatusContext` 和 `isLarkOAuthAvailable`，并将 `const [status, setStatus] = useState({})` 替换为：

```js
const [statusState] = useContext(StatusContext);
const status = statusState?.status || {};
```

将一次性状态加载 effect 替换为：

```js
useEffect(() => {
  setTurnstileEnabled(Boolean(status.turnstile_check));
  setTurnstileSiteKey(status.turnstile_site_key || '');
}, [status.turnstile_check, status.turnstile_site_key]);
```

绑定入口使用 `isLarkOAuthAvailable(status)` 判断可见性，并把 `status.server_address` 传给 `onLarkOAuthClicked`。

- [ ] **步骤 3：增加本地化首页状态标签**

在 `home.system_status.config` 下增加以下条目：

`web/default/src/locales/zh/translation.json`:

```json
"lark_oauth": "飞书身份验证："
```

`web/default/src/locales/en/translation.json`:

```json
"lark_oauth": "Lark authentication:"
```

在 `web/default/src/pages/Home/index.js` 的现有 GitHub 和微信状态行旁新增飞书状态行。使用 `t('home.system_status.config.lark_oauth')`，读取 `statusState?.status?.lark_oauth_enabled`，并复用现有 `enabled`/`disabled` 翻译及绿/红样式。

- [ ] **步骤 4：校验翻译文件并运行辅助函数测试**

```powershell
Get-Content -Raw web/default/src/locales/zh/translation.json | ConvertFrom-Json | Out-Null
Get-Content -Raw web/default/src/locales/en/translation.json | ConvertFrom-Json | Out-Null
Set-Location web/default
npm test -- --watchAll=false src/components/larkOAuth.test.js
```

预期：两个 JSON 文件均可解析，辅助函数测试通过。

- [ ] **步骤 5：构建 default 主题**

```powershell
$env:DISABLE_ESLINT_PLUGIN='true'
npm run build
Remove-Item Env:DISABLE_ESLINT_PLUGIN
```

预期：CRA 报告生产构建成功，并将产物移动到 `web/build/default`。

- [ ] **步骤 6：提交 default 界面单元**

```powershell
Set-Location ../..
git add web/default/src/components/LoginForm.js web/default/src/components/PersonalSetting.js web/default/src/pages/Home/index.js web/default/src/locales/zh/translation.json web/default/src/locales/en/translation.json
git commit -m "fix: refresh default lark oauth status"
```

---

### 任务 4：air 主题 OAuth 辅助模块

**文件：**
- 新建：`web/air/src/components/larkOAuth.js`
- 新建：`web/air/src/components/larkOAuth.test.js`
- 修改：`web/air/src/components/utils.js`

**接口：**
- 输出：`isLarkOAuthAvailable(status: object | null | undefined): boolean`
- 输出：`getLarkRedirectUri(serverAddress: string, currentOrigin: string): string`
- 输出：`buildLarkAuthorizeUrl(clientId: string, redirectUri: string, state: string): string`
- 输出：`onLarkOAuthClicked(larkClientId: string, serverAddress: string): Promise<void>`

- [ ] **步骤 1：增加失败的 air 辅助函数测试**

新建 `web/air/src/components/larkOAuth.test.js`：

```js
import {
  buildLarkAuthorizeUrl,
  getLarkRedirectUri,
  isLarkOAuthAvailable,
} from './larkOAuth';

describe('isLarkOAuthAvailable', () => {
  test.each([
    [{ lark_oauth_enabled: false, lark_client_id: '' }, false],
    [{ lark_oauth_enabled: false, lark_client_id: 'cli_test' }, false],
    [{ lark_oauth_enabled: true, lark_client_id: '' }, false],
    [{ lark_oauth_enabled: true, lark_client_id: 'cli_test' }, true],
    [undefined, false],
  ])('returns the expected availability for %p', (status, expected) => {
    expect(isLarkOAuthAvailable(status)).toBe(expected);
  });
});

describe('getLarkRedirectUri', () => {
  test('normalizes trailing slashes', () => {
    expect(
      getLarkRedirectUri(
        'http://192.168.0.86:3000///',
        'http://192.168.0.86:3000'
      )
    ).toBe('http://192.168.0.86:3000/oauth/lark');
  });

  test.each([
    ['not-a-url'],
    ['ftp://example.com'],
    ['https://user:pass@example.com'],
    ['https://example.com/base'],
    ['https://example.com/?query=1'],
    ['https://example.com/#fragment'],
  ])('rejects invalid server address %s', (serverAddress) => {
    expect(() =>
      getLarkRedirectUri(serverAddress, 'https://example.com')
    ).toThrow('系统服务器地址配置无效');
  });

  test('rejects a different browser origin', () => {
    expect(() =>
      getLarkRedirectUri('https://api.example.com', 'https://alias.example.com')
    ).toThrow('当前访问地址 https://alias.example.com 与系统服务器地址 https://api.example.com 不一致');
  });
});

test('buildLarkAuthorizeUrl encodes all OAuth parameters', () => {
  const url = new URL(
    buildLarkAuthorizeUrl(
      'cli_test',
      'https://api.example.com/oauth/lark',
      'state with spaces'
    )
  );
  expect(url.origin + url.pathname).toBe(
    'https://accounts.feishu.cn/open-apis/authen/v1/authorize'
  );
  expect(url.searchParams.get('client_id')).toBe('cli_test');
  expect(url.searchParams.get('response_type')).toBe('code');
  expect(url.searchParams.get('redirect_uri')).toBe(
    'https://api.example.com/oauth/lark'
  );
  expect(url.searchParams.get('state')).toBe('state with spaces');
});
```

- [ ] **步骤 2：安装 air 主题依赖**

```powershell
Set-Location web/air
npm install --legacy-peer-deps --no-package-lock
```

预期：安装成功，且不产生被 Git 跟踪的锁文件。

- [ ] **步骤 3：运行测试并确认失败**

```powershell
Set-Location web/air
npm test -- --watchAll=false src/components/larkOAuth.test.js
```

预期：测试失败，因为 `./larkOAuth` 尚不存在。

- [ ] **步骤 4：实现 air 纯函数辅助模块**

新建 `web/air/src/components/larkOAuth.js`：

```js
const LARK_AUTHORIZE_ENDPOINT =
  'https://accounts.feishu.cn/open-apis/authen/v1/authorize';

const invalidServerAddress = () =>
  new Error(
    '系统服务器地址配置无效，请在系统设置中填写仅包含协议、主机和端口的 HTTP/HTTPS 地址。'
  );

export function isLarkOAuthAvailable(status) {
  return Boolean(status?.lark_oauth_enabled && status?.lark_client_id);
}

export function getLarkRedirectUri(serverAddress, currentOrigin) {
  if (typeof serverAddress !== 'string' || serverAddress.trim() === '') {
    throw invalidServerAddress();
  }

  let parsed;
  try {
    parsed = new URL(serverAddress.trim());
  } catch {
    throw invalidServerAddress();
  }

  if (
    !['http:', 'https:'].includes(parsed.protocol) ||
    parsed.username ||
    parsed.password ||
    !/^\/*$/.test(parsed.pathname) ||
    parsed.search ||
    parsed.hash
  ) {
    throw invalidServerAddress();
  }

  const canonicalOrigin = parsed.origin;
  if (canonicalOrigin !== currentOrigin) {
    throw new Error(
      `当前访问地址 ${currentOrigin} 与系统服务器地址 ${canonicalOrigin} 不一致。请通过系统服务器地址访问，或由管理员修改 ServerAddress 后重试。`
    );
  }
  return `${canonicalOrigin}/oauth/lark`;
}

export function buildLarkAuthorizeUrl(clientId, redirectUri, state) {
  const params = new URLSearchParams({
    client_id: clientId,
    response_type: 'code',
    redirect_uri: redirectUri,
    state,
  });
  return `${LARK_AUTHORIZE_ENDPOINT}?${params.toString()}`;
}
```

- [ ] **步骤 5：接入 air 编排函数**

在 `web/air/src/components/utils.js` 中增加：

```js
import {
  buildLarkAuthorizeUrl,
  getLarkRedirectUri,
} from './larkOAuth';
```

将飞书函数替换为：

```js
export async function onLarkOAuthClicked(larkClientId, serverAddress) {
  let redirectUri;
  try {
    redirectUri = getLarkRedirectUri(serverAddress, window.location.origin);
  } catch (error) {
    showError(error.message);
    return;
  }

  const state = await getOAuthState();
  if (!state) return;
  window.open(buildLarkAuthorizeUrl(larkClientId, redirectUri, state));
}
```

保持现有 GitHub 和 State 函数不变。

- [ ] **步骤 6：运行测试并提交**

```powershell
npm test -- --watchAll=false src/components/larkOAuth.test.js
Set-Location ../..
git add web/air/src/components/larkOAuth.js web/air/src/components/larkOAuth.test.js web/air/src/components/utils.js
git commit -m "feat: validate air lark oauth origin"
```

预期：测试通过。

---

### 任务 5：air 主题响应式界面与状态总览

**文件：**
- 修改：`web/air/src/components/LoginForm.js`
- 修改：`web/air/src/components/PersonalSetting.js`
- 修改：`web/air/src/pages/Home/index.js`

**接口：**
- 输入：结构为 `[state, dispatch]`、由 `SiderBar.loadStatus()` 填充的 `StatusContext`
- 输入：任务 4 提供的 `isLarkOAuthAvailable(status)` 和 `onLarkOAuthClicked(clientId, serverAddress)`
- 输出：响应式 air 登录/绑定入口可见性，以及首页总览状态行

- [ ] **步骤 1：将 air LoginForm 改为使用 StatusContext**

导入 `StatusContext` 和 `isLarkOAuthAvailable`，将本地状态替换为：

```js
const [statusState] = useContext(StatusContext);
const status = statusState?.status || {};
```

在挂载 effect 中保留登录过期处理，但移除 `localStorage.status` 读取。另加一个响应式 Turnstile effect：

```js
useEffect(() => {
  setTurnstileEnabled(Boolean(status.turnstile_check));
  setTurnstileSiteKey(status.turnstile_site_key || '');
}, [status.turnstile_check, status.turnstile_site_key]);
```

第三方登录区域与飞书按钮条件均使用 `isLarkOAuthAvailable(status)`，并调用 `onLarkOAuthClicked(status.lark_client_id, status.server_address)`。

- [ ] **步骤 2：改造 air PersonalSetting，避免重复请求用户信息**

导入 `StatusContext` 和 `isLarkOAuthAvailable`，并将本地状态替换为：

```js
const [statusState] = useContext(StatusContext);
const status = statusState?.status || {};
```

将现有挂载 effect 拆分为以下职责：

```js
useEffect(() => {
  getUserData().then();
  loadModels().then();
  getAffLink().then();
  setTransferAmount(getQuotaPerUnit());
}, []);

useEffect(() => {
  setTurnstileEnabled(Boolean(status.turnstile_check));
  setTurnstileSiteKey(status.turnstile_site_key || '');
}, [status.turnstile_check, status.turnstile_site_key]);
```

保持现有 `getUserData()` 函数不变；它已在成功时分发 `res.data`，失败时展示 API 消息。绑定按钮启用状态使用 `isLarkOAuthAvailable(status)`，并把 `status.server_address` 传给点击处理函数。

- [ ] **步骤 3：增加 air 首页状态行**

在 `web/air/src/pages/Home/index.js` 的 GitHub 与微信状态行之间插入：

```jsx
<p>
  飞书身份验证：
  {statusState?.status?.lark_oauth_enabled === true ? '已启用' : '未启用'}
</p>
```

- [ ] **步骤 4：运行测试并构建 air**

```powershell
Set-Location web/air
npm test -- --watchAll=false src/components/larkOAuth.test.js
$env:DISABLE_ESLINT_PLUGIN='true'
npm run build
Remove-Item Env:DISABLE_ESLINT_PLUGIN
```

预期：测试与生产构建通过，产物移动到 `web/build/air`。

- [ ] **步骤 5：提交 air 界面单元**

```powershell
Set-Location ../..
git add web/air/src/components/LoginForm.js web/air/src/components/PersonalSetting.js web/air/src/pages/Home/index.js
git commit -m "fix: refresh air lark oauth status"
```

---

### 任务 6：berry 主题 OAuth 辅助模块

**文件：**
- 新建：`web/berry/src/utils/larkOAuth.js`
- 新建：`web/berry/src/utils/larkOAuth.test.js`
- 修改：`web/berry/src/utils/common.js`

**接口：**
- 输出：`isLarkOAuthAvailable(status: object | null | undefined): boolean`
- 输出：`getLarkRedirectUri(serverAddress: string, currentOrigin: string): string`
- 输出：`buildLarkAuthorizeUrl(clientId: string, redirectUri: string, state: string): string`
- 输出：`onLarkOAuthClicked(larkClientId: string, serverAddress: string): Promise<void>`

- [ ] **步骤 1：增加失败的 berry 辅助函数测试**

新建 `web/berry/src/utils/larkOAuth.test.js`：

```js
import {
  buildLarkAuthorizeUrl,
  getLarkRedirectUri,
  isLarkOAuthAvailable,
} from './larkOAuth';

describe('isLarkOAuthAvailable', () => {
  test.each([
    [{ lark_oauth_enabled: false, lark_client_id: '' }, false],
    [{ lark_oauth_enabled: false, lark_client_id: 'cli_test' }, false],
    [{ lark_oauth_enabled: true, lark_client_id: '' }, false],
    [{ lark_oauth_enabled: true, lark_client_id: 'cli_test' }, true],
    [undefined, false],
  ])('returns the expected availability for %p', (status, expected) => {
    expect(isLarkOAuthAvailable(status)).toBe(expected);
  });
});

describe('getLarkRedirectUri', () => {
  test('normalizes trailing slashes', () => {
    expect(
      getLarkRedirectUri(
        'http://192.168.0.86:3000///',
        'http://192.168.0.86:3000'
      )
    ).toBe('http://192.168.0.86:3000/oauth/lark');
  });

  test.each([
    ['not-a-url'],
    ['ftp://example.com'],
    ['https://user:pass@example.com'],
    ['https://example.com/base'],
    ['https://example.com/?query=1'],
    ['https://example.com/#fragment'],
  ])('rejects invalid server address %s', (serverAddress) => {
    expect(() =>
      getLarkRedirectUri(serverAddress, 'https://example.com')
    ).toThrow('系统服务器地址配置无效');
  });

  test('rejects a different browser origin', () => {
    expect(() =>
      getLarkRedirectUri('https://api.example.com', 'https://alias.example.com')
    ).toThrow('当前访问地址 https://alias.example.com 与系统服务器地址 https://api.example.com 不一致');
  });
});

test('buildLarkAuthorizeUrl encodes all OAuth parameters', () => {
  const url = new URL(
    buildLarkAuthorizeUrl(
      'cli_test',
      'https://api.example.com/oauth/lark',
      'state with spaces'
    )
  );
  expect(url.origin + url.pathname).toBe(
    'https://accounts.feishu.cn/open-apis/authen/v1/authorize'
  );
  expect(url.searchParams.get('client_id')).toBe('cli_test');
  expect(url.searchParams.get('response_type')).toBe('code');
  expect(url.searchParams.get('redirect_uri')).toBe(
    'https://api.example.com/oauth/lark'
  );
  expect(url.searchParams.get('state')).toBe('state with spaces');
});
```

- [ ] **步骤 2：安装 berry 主题依赖**

```powershell
Set-Location web/berry
npm install --legacy-peer-deps --no-package-lock
```

预期：安装成功，且不产生被 Git 跟踪的锁文件。

- [ ] **步骤 3：运行测试并确认失败**

```powershell
Set-Location web/berry
npm test -- --watchAll=false src/utils/larkOAuth.test.js
```

预期：测试失败，因为 `./larkOAuth` 尚不存在。

- [ ] **步骤 4：实现 berry 纯函数辅助模块**

新建 `web/berry/src/utils/larkOAuth.js`：

```js
const LARK_AUTHORIZE_ENDPOINT =
  'https://accounts.feishu.cn/open-apis/authen/v1/authorize';

const invalidServerAddress = () =>
  new Error(
    '系统服务器地址配置无效，请在系统设置中填写仅包含协议、主机和端口的 HTTP/HTTPS 地址。'
  );

export function isLarkOAuthAvailable(status) {
  return Boolean(status?.lark_oauth_enabled && status?.lark_client_id);
}

export function getLarkRedirectUri(serverAddress, currentOrigin) {
  if (typeof serverAddress !== 'string' || serverAddress.trim() === '') {
    throw invalidServerAddress();
  }

  let parsed;
  try {
    parsed = new URL(serverAddress.trim());
  } catch {
    throw invalidServerAddress();
  }

  if (
    !['http:', 'https:'].includes(parsed.protocol) ||
    parsed.username ||
    parsed.password ||
    !/^\/*$/.test(parsed.pathname) ||
    parsed.search ||
    parsed.hash
  ) {
    throw invalidServerAddress();
  }

  const canonicalOrigin = parsed.origin;
  if (canonicalOrigin !== currentOrigin) {
    throw new Error(
      `当前访问地址 ${currentOrigin} 与系统服务器地址 ${canonicalOrigin} 不一致。请通过系统服务器地址访问，或由管理员修改 ServerAddress 后重试。`
    );
  }
  return `${canonicalOrigin}/oauth/lark`;
}

export function buildLarkAuthorizeUrl(clientId, redirectUri, state) {
  const params = new URLSearchParams({
    client_id: clientId,
    response_type: 'code',
    redirect_uri: redirectUri,
    state,
  });
  return `${LARK_AUTHORIZE_ENDPOINT}?${params.toString()}`;
}
```

- [ ] **步骤 5：接入 berry 编排函数**

在 `web/berry/src/utils/common.js` 中增加：

```js
import {
  buildLarkAuthorizeUrl,
  getLarkRedirectUri,
} from './larkOAuth';
```

将飞书函数替换为：

```js
export async function onLarkOAuthClicked(larkClientId, serverAddress) {
  let redirectUri;
  try {
    redirectUri = getLarkRedirectUri(serverAddress, window.location.origin);
  } catch (error) {
    showError(error.message);
    return;
  }

  const state = await getOAuthState();
  if (!state) return;
  window.open(buildLarkAuthorizeUrl(larkClientId, redirectUri, state));
}
```

保持现有 `showError`、`getOAuthState`、GitHub 和 OIDC 行为不变。

- [ ] **步骤 6：运行测试并提交**

```powershell
npm test -- --watchAll=false src/utils/larkOAuth.test.js
Set-Location ../..
git add web/berry/src/utils/larkOAuth.js web/berry/src/utils/larkOAuth.test.js web/berry/src/utils/common.js
git commit -m "feat: validate berry lark oauth origin"
```

预期：测试通过。

---

### 任务 7：berry 主题可用性判断与调用点

**文件：**
- 修改：`web/berry/src/config.js`
- 修改：`web/berry/src/views/Authentication/AuthForms/AuthLogin.js`
- 修改：`web/berry/src/views/Profile/index.js`

**接口：**
- 输入：Redux `siteInfo` 和任务 6 提供的 `isLarkOAuthAvailable(status)`
- 输入：任务 6 提供的 `onLarkOAuthClicked(clientId, serverAddress)`
- 输出：berry 中一致的登录与绑定入口可用性

- [ ] **步骤 1：声明飞书初始状态**

在 `web/berry/src/config.js` 现有 GitHub 字段旁增加：

```js
lark_client_id: '',
lark_oauth_enabled: false,
```

- [ ] **步骤 2：更新 AuthLogin 可用性判断**

从 `utils/larkOAuth` 导入 `isLarkOAuthAvailable`。第三方登录可用性和飞书渲染处的两个 `siteInfo.lark_client_id` 判断均替换为 `isLarkOAuthAvailable(siteInfo)`，并调用：

```js
onLarkOAuthClicked(siteInfo.lark_client_id, siteInfo.server_address)
```

- [ ] **步骤 3：更新 Profile 绑定入口可用性**

从 `utils/larkOAuth` 导入 `isLarkOAuthAvailable`。仅在以下条件成立时渲染绑定按钮：

```js
isLarkOAuthAvailable(status) && !inputs.lark_id
```

调用：

```js
onLarkOAuthClicked(status.lark_client_id, status.server_address)
```

- [ ] **步骤 4：运行测试并构建 berry**

```powershell
Set-Location web/berry
npm test -- --watchAll=false src/utils/larkOAuth.test.js
$env:DISABLE_ESLINT_PLUGIN='true'
npm run build
Remove-Item Env:DISABLE_ESLINT_PLUGIN
```

预期：测试与生产构建通过，产物移动到 `web/build/berry`。

- [ ] **步骤 5：提交 berry 界面单元**

```powershell
Set-Location ../..
git add web/berry/src/config.js web/berry/src/views/Authentication/AuthForms/AuthLogin.js web/berry/src/views/Profile/index.js
git commit -m "fix: enforce berry lark oauth availability"
```

---

### 任务 8：跨主题构建与运行时验证

**文件：**
- 验证生成且被忽略的产物：`web/build/default`、`web/build/berry`、`web/build/air`
- 本任务不应创建源文件

**接口：**
- 输入：此前所有任务的产物
- 输出：三个主题均可构建且 Go 二进制能够嵌入它们的证据

- [ ] **步骤 1：从各主题根目录运行全部聚焦前端测试**

```powershell
Set-Location web/default
npm test -- --watchAll=false src/components/larkOAuth.test.js
Set-Location ../air
npm test -- --watchAll=false src/components/larkOAuth.test.js
Set-Location ../berry
npm test -- --watchAll=false src/utils/larkOAuth.test.js
Set-Location ../..
```

预期：三次 Jest 测试均通过。

- [ ] **步骤 2：运行全部 Go 测试**

```powershell
go test ./...
```

预期：所有包均通过测试。

- [ ] **步骤 3：通过项目统一入口构建全部主题**

```powershell
Set-Location web
sh build.sh
Set-Location ..
```

预期：default、berry、air 均报告生产构建成功，且 `web/build` 下存在对应目录。

- [ ] **步骤 4：验证可嵌入产物并编译后端**

```powershell
Get-ChildItem web/build | Select-Object Name
go build -o one-api.exe .
```

预期：输出列出 `default`、`berry`、`air`，Go 编译成功。按 `AGENTS.md` 要求保持 `one-api.exe` 不被跟踪。

- [ ] **步骤 5：执行已配置的 air 主题冒烟测试**

重新构建并部署后，使用已配置的 `http://192.168.0.86:3000`：

1. 确认 `/api/status` 返回 `lark_oauth_enabled: true`、非空 `lark_client_id` 和 `server_address: "http://192.168.0.86:3000"`。
2. 清理一次站点存储，打开 `/login`，确认状态响应返回后飞书按钮立即出现，无需再次刷新。
3. 点击飞书登录，确认授权请求经 URL 解码后包含 `redirect_uri=http://192.168.0.86:3000/oauth/lark`。
4. 完成授权并确认登录成功。
5. 通过 `http://127.0.0.1:3000` 等不同来源打开同一部署，确认点击飞书后展示地址不匹配提示，且不打开飞书。
6. 使用 root 登录并打开 air 首页，确认出现“飞书身份验证：已启用”。

- [ ] **步骤 6：确认工作树仅包含预期源码变更**

```powershell
git status --short
git diff --check
```

预期：生成的 `web/build/*`、`one-api.exe` 和本地运行残留仍被忽略；不存在空白错误或无关的已跟踪变更。

- [ ] **步骤 7：若验证过程修改了已跟踪文件则停止**

预期：步骤 6 不报告意外的已跟踪变更。若格式化工具或测试修改了已跟踪文件，不要直接暂存；先检查差异，返回负责该文件的任务，重新执行该任务的测试循环，并使用该任务的提交信息。
