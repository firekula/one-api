# Lark OAuth UI Consistency Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Lark/Feishu login and binding appear reactively in all three themes, show its status on existing overview cards, and reject mismatched OAuth origins before authorization.

**Architecture:** Keep `ServerAddress` as the canonical public origin. Add one small, pure Lark OAuth helper module inside each CRA theme because CRA cannot import source from sibling theme roots; components consume identical helper interfaces while retaining their existing UI libraries. Normalize the same redirect URI in Go before token exchange, and make default/air consume their existing `StatusContext` instead of one-time `localStorage` snapshots.

**Tech Stack:** Go 1.20+, Gin, React 18, CRA/react-scripts 5, Jest, React Context, Redux, Semantic UI, Semi UI, Material UI.

**Spec:** `docs/superpowers/specs/2026-08-25-lark-oauth-ui-consistency-design.md`

## Global Constraints

- `ServerAddress` remains the administrator-configured canonical external origin; do not infer it from Docker, network interfaces, request Host, or proxy headers.
- Accept only absolute HTTP/HTTPS origins with no credentials, query, fragment, or non-root path.
- Normalize one or more trailing `/` characters before appending `/oauth/lark`.
- Compare the normalized canonical origin with `window.location.origin` before requesting OAuth State.
- Never include App Secret, authorization code, access token, or Session content in an error message.
- Do not change backend route names, JSON response shapes, user creation, account binding, or OAuth State validation.
- Do not remove berry's Service Worker or add a new berry system-status card.
- Preserve the three themes' existing UI libraries and visual style.

---

## File Structure

### Backend

- Modify `controller/auth/lark.go`: own canonical Lark redirect URI construction and use it in the token request.
- Create `controller/auth/lark_test.go`: table-test redirect URI normalization.

### default theme

- Create `web/default/src/components/larkOAuth.js`: pure availability, redirect validation, and authorization URL helpers.
- Create `web/default/src/components/larkOAuth.test.js`: helper behavior and state matrix.
- Modify `web/default/src/components/utils.js`: orchestrate address validation, State retrieval, and window navigation.
- Modify `web/default/src/components/LoginForm.js`: consume reactive `StatusContext` and canonical address.
- Modify `web/default/src/components/PersonalSetting.js`: consume reactive `StatusContext` for binding.
- Modify `web/default/src/pages/Home/index.js`: render Lark status in the existing system overview.
- Modify `web/default/src/locales/zh/translation.json`: add Chinese overview label.
- Modify `web/default/src/locales/en/translation.json`: add English overview label.

### air theme

- Create `web/air/src/components/larkOAuth.js`: pure availability, redirect validation, and authorization URL helpers.
- Create `web/air/src/components/larkOAuth.test.js`: helper behavior and state matrix.
- Modify `web/air/src/components/utils.js`: orchestrate address validation, State retrieval, and window navigation.
- Modify `web/air/src/components/LoginForm.js`: consume reactive `StatusContext` and canonical address.
- Modify `web/air/src/components/PersonalSetting.js`: consume reactive `StatusContext` for binding without refetching the user on each status update.
- Modify `web/air/src/pages/Home/index.js`: render Lark status in the existing system overview.

### berry theme

- Create `web/berry/src/utils/larkOAuth.js`: pure availability, redirect validation, and authorization URL helpers.
- Create `web/berry/src/utils/larkOAuth.test.js`: helper behavior and state matrix.
- Modify `web/berry/src/utils/common.js`: orchestrate address validation, State retrieval, and window navigation.
- Modify `web/berry/src/config.js`: declare Lark fields in initial Redux site info.
- Modify `web/berry/src/views/Authentication/AuthForms/AuthLogin.js`: use enabled-plus-client-ID availability.
- Modify `web/berry/src/views/Profile/index.js`: use the same availability rule for binding.

---

### Task 1: Canonical Backend Redirect URI

**Files:**
- Modify: `controller/auth/lark.go`
- Create: `controller/auth/lark_test.go`

**Interfaces:**
- Consumes: `config.ServerAddress string`
- Produces: `func getLarkRedirectURI(serverAddress string) string`
- Produces: the exact `{trimmed-origin}/oauth/lark` value sent as `redirect_uri` to `https://accounts.feishu.cn/oauth/v3/token`

- [ ] **Step 1: Write the failing normalization test**

Create `controller/auth/lark_test.go`:

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

- [ ] **Step 2: Run the test and verify it fails**

Run from the repository root:

```powershell
go test ./controller/auth -run TestGetLarkRedirectURI -count=1
```

Expected: compilation fails with `undefined: getLarkRedirectURI`.

- [ ] **Step 3: Implement the minimal helper and use it**

In `controller/auth/lark.go`, import `strings` and add:

```go
func getLarkRedirectURI(serverAddress string) string {
	return strings.TrimRight(serverAddress, "/") + "/oauth/lark"
}
```

Replace the token request value:

```go
"redirect_uri": getLarkRedirectURI(config.ServerAddress),
```

- [ ] **Step 4: Format and run focused tests**

```powershell
gofmt -w controller/auth/lark.go controller/auth/lark_test.go
go test ./controller/auth -run TestGetLarkRedirectURI -count=1
```

Expected: `ok github.com/songquanpeng/one-api/controller/auth`.

- [ ] **Step 5: Run the full Go test suite**

```powershell
go test ./...
```

Expected: all packages pass.

- [ ] **Step 6: Commit the backend unit**

```powershell
git add controller/auth/lark.go controller/auth/lark_test.go
git commit -m "fix: normalize lark oauth redirect uri"
```

---

### Task 2: default Theme OAuth Helpers

**Files:**
- Create: `web/default/src/components/larkOAuth.js`
- Create: `web/default/src/components/larkOAuth.test.js`
- Modify: `web/default/src/components/utils.js`

**Interfaces:**
- Produces: `isLarkOAuthAvailable(status: object | null | undefined): boolean`
- Produces: `getLarkRedirectUri(serverAddress: string, currentOrigin: string): string`
- Produces: `buildLarkAuthorizeUrl(clientId: string, redirectUri: string, state: string): string`
- Produces: `onLarkOAuthClicked(larkClientId: string, serverAddress: string): Promise<void>`

- [ ] **Step 1: Write failing pure-helper tests**

Create `web/default/src/components/larkOAuth.test.js`:

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

- [ ] **Step 2: Install default theme dependencies**

```powershell
Set-Location web/default
npm install --legacy-peer-deps --no-package-lock
```

Expected: installation succeeds without creating a tracked lockfile.

- [ ] **Step 3: Run the test and verify it fails**

```powershell
Set-Location web/default
npm test -- --watchAll=false src/components/larkOAuth.test.js
```

Expected: FAIL because `./larkOAuth` does not exist.

- [ ] **Step 4: Implement the pure helper module**

Create `web/default/src/components/larkOAuth.js`:

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

- [ ] **Step 5: Run the helper tests**

```powershell
npm test -- --watchAll=false src/components/larkOAuth.test.js
```

Expected: all tests pass.

- [ ] **Step 6: Wire validation before OAuth State retrieval**

In `web/default/src/components/utils.js`, import the helpers and replace the Lark function with:

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

Keep the existing GitHub and State functions unchanged.

- [ ] **Step 7: Re-run tests and commit the helper unit**

```powershell
npm test -- --watchAll=false src/components/larkOAuth.test.js
Set-Location ../..
git add web/default/src/components/larkOAuth.js web/default/src/components/larkOAuth.test.js web/default/src/components/utils.js
git commit -m "feat: validate default lark oauth origin"
```

Expected: tests pass and the commit contains only the default helper unit.

---

### Task 3: default Theme Reactive UI and Status Overview

**Files:**
- Modify: `web/default/src/components/LoginForm.js`
- Modify: `web/default/src/components/PersonalSetting.js`
- Modify: `web/default/src/pages/Home/index.js`
- Modify: `web/default/src/locales/zh/translation.json`
- Modify: `web/default/src/locales/en/translation.json`

**Interfaces:**
- Consumes: `StatusContext` shaped as `[state, dispatch]`, with `state.status` containing `/api/status` data
- Consumes: `isLarkOAuthAvailable(status)` and `onLarkOAuthClicked(clientId, serverAddress)` from Task 2
- Produces: reactive login/binding visibility and a localized home status row

- [ ] **Step 1: Convert LoginForm to reactive status**

In `LoginForm.js`:

```js
import { StatusContext } from '../context/Status';
import { isLarkOAuthAvailable } from './larkOAuth';
```

Replace the local status state and one-time `localStorage` load with:

```js
const [statusState] = useContext(StatusContext);
const status = statusState?.status || {};
```

Keep the expired-session effect, but remove its `localStorage.getItem('status')`, JSON parsing, and `setStatus` calls. Replace the third-party group and Lark button conditions with `isLarkOAuthAvailable(status)`, and call:

```js
onLarkOAuthClicked(status.lark_client_id, status.server_address)
```

- [ ] **Step 2: Convert PersonalSetting to reactive status**

Import `StatusContext` and `isLarkOAuthAvailable`, then replace `const [status, setStatus] = useState({})` with:

```js
const [statusState] = useContext(StatusContext);
const status = statusState?.status || {};
```

Replace the one-time status-loading effect with:

```js
useEffect(() => {
  setTurnstileEnabled(Boolean(status.turnstile_check));
  setTurnstileSiteKey(status.turnstile_site_key || '');
}, [status.turnstile_check, status.turnstile_site_key]);
```

Use `isLarkOAuthAvailable(status)` for binding visibility and pass `status.server_address` to `onLarkOAuthClicked`.

- [ ] **Step 3: Add localized home status labels**

Under `home.system_status.config`, add these entries:

`web/default/src/locales/zh/translation.json`:

```json
"lark_oauth": "飞书身份验证："
```

`web/default/src/locales/en/translation.json`:

```json
"lark_oauth": "Lark authentication:"
```

In `web/default/src/pages/Home/index.js`, add a status row beside the existing GitHub and WeChat rows. Use `t('home.system_status.config.lark_oauth')`, read `statusState?.status?.lark_oauth_enabled`, and reuse the existing `enabled`/`disabled` translations and green/red styles.

- [ ] **Step 4: Validate translations and helper tests**

```powershell
Get-Content -Raw web/default/src/locales/zh/translation.json | ConvertFrom-Json | Out-Null
Get-Content -Raw web/default/src/locales/en/translation.json | ConvertFrom-Json | Out-Null
Set-Location web/default
npm test -- --watchAll=false src/components/larkOAuth.test.js
```

Expected: both JSON files parse and helper tests pass.

- [ ] **Step 5: Build default theme**

```powershell
$env:DISABLE_ESLINT_PLUGIN='true'
npm run build
Remove-Item Env:DISABLE_ESLINT_PLUGIN
```

Expected: CRA reports a successful production build and moves it to `web/build/default`.

- [ ] **Step 6: Commit the default UI unit**

```powershell
Set-Location ../..
git add web/default/src/components/LoginForm.js web/default/src/components/PersonalSetting.js web/default/src/pages/Home/index.js web/default/src/locales/zh/translation.json web/default/src/locales/en/translation.json
git commit -m "fix: refresh default lark oauth status"
```

---

### Task 4: air Theme OAuth Helpers

**Files:**
- Create: `web/air/src/components/larkOAuth.js`
- Create: `web/air/src/components/larkOAuth.test.js`
- Modify: `web/air/src/components/utils.js`

**Interfaces:**
- Produces: `isLarkOAuthAvailable(status: object | null | undefined): boolean`
- Produces: `getLarkRedirectUri(serverAddress: string, currentOrigin: string): string`
- Produces: `buildLarkAuthorizeUrl(clientId: string, redirectUri: string, state: string): string`
- Produces: `onLarkOAuthClicked(larkClientId: string, serverAddress: string): Promise<void>`

- [ ] **Step 1: Add the failing air helper test**

Create `web/air/src/components/larkOAuth.test.js`:

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

- [ ] **Step 2: Install air theme dependencies**

```powershell
Set-Location web/air
npm install --legacy-peer-deps --no-package-lock
```

Expected: installation succeeds without creating a tracked lockfile.

- [ ] **Step 3: Run the test and verify it fails**

```powershell
Set-Location web/air
npm test -- --watchAll=false src/components/larkOAuth.test.js
```

Expected: FAIL because `./larkOAuth` does not exist.

- [ ] **Step 4: Add the air pure helper implementation**

Create `web/air/src/components/larkOAuth.js`:

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

- [ ] **Step 5: Wire the air orchestration function**

In `web/air/src/components/utils.js`, add:

```js
import {
  buildLarkAuthorizeUrl,
  getLarkRedirectUri,
} from './larkOAuth';
```

Replace the Lark function with:

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

Keep the existing GitHub and State functions unchanged.

- [ ] **Step 6: Run tests and commit**

```powershell
npm test -- --watchAll=false src/components/larkOAuth.test.js
Set-Location ../..
git add web/air/src/components/larkOAuth.js web/air/src/components/larkOAuth.test.js web/air/src/components/utils.js
git commit -m "feat: validate air lark oauth origin"
```

Expected: tests pass.

---

### Task 5: air Theme Reactive UI and Status Overview

**Files:**
- Modify: `web/air/src/components/LoginForm.js`
- Modify: `web/air/src/components/PersonalSetting.js`
- Modify: `web/air/src/pages/Home/index.js`

**Interfaces:**
- Consumes: `StatusContext` shaped as `[state, dispatch]`, populated by `SiderBar.loadStatus()`
- Consumes: `isLarkOAuthAvailable(status)` and `onLarkOAuthClicked(clientId, serverAddress)` from Task 4
- Produces: reactive air login/binding visibility and a home overview row

- [ ] **Step 1: Convert air LoginForm to StatusContext**

Import `StatusContext` and `isLarkOAuthAvailable`. Replace local status state with:

```js
const [statusState] = useContext(StatusContext);
const status = statusState?.status || {};
```

Keep expired-session handling in its mount effect, but remove the `localStorage.status` read. Add a separate reactive Turnstile effect:

```js
useEffect(() => {
  setTurnstileEnabled(Boolean(status.turnstile_check));
  setTurnstileSiteKey(status.turnstile_site_key || '');
}, [status.turnstile_check, status.turnstile_site_key]);
```

Use `isLarkOAuthAvailable(status)` in both the third-party group condition and the Lark button condition. Call `onLarkOAuthClicked(status.lark_client_id, status.server_address)`.

- [ ] **Step 2: Convert air PersonalSetting without duplicate user requests**

Import `StatusContext` and `isLarkOAuthAvailable`, then replace local status state with:

```js
const [statusState] = useContext(StatusContext);
const status = statusState?.status || {};
```

Split the existing mount effect into these responsibilities:

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

Keep the existing `getUserData()` function unchanged; it already dispatches `res.data` on success and shows the API message on failure. Use `isLarkOAuthAvailable(status)` for the bind button's enabled state and pass `status.server_address` to the click handler.

- [ ] **Step 3: Add the air home status row**

In `web/air/src/pages/Home/index.js`, insert between GitHub and WeChat:

```jsx
<p>
  飞书身份验证：
  {statusState?.status?.lark_oauth_enabled === true ? '已启用' : '未启用'}
</p>
```

- [ ] **Step 4: Run tests and build air**

```powershell
Set-Location web/air
npm test -- --watchAll=false src/components/larkOAuth.test.js
$env:DISABLE_ESLINT_PLUGIN='true'
npm run build
Remove-Item Env:DISABLE_ESLINT_PLUGIN
```

Expected: tests and production build pass; output moves to `web/build/air`.

- [ ] **Step 5: Commit the air UI unit**

```powershell
Set-Location ../..
git add web/air/src/components/LoginForm.js web/air/src/components/PersonalSetting.js web/air/src/pages/Home/index.js
git commit -m "fix: refresh air lark oauth status"
```

---

### Task 6: berry Theme OAuth Helpers

**Files:**
- Create: `web/berry/src/utils/larkOAuth.js`
- Create: `web/berry/src/utils/larkOAuth.test.js`
- Modify: `web/berry/src/utils/common.js`

**Interfaces:**
- Produces: `isLarkOAuthAvailable(status: object | null | undefined): boolean`
- Produces: `getLarkRedirectUri(serverAddress: string, currentOrigin: string): string`
- Produces: `buildLarkAuthorizeUrl(clientId: string, redirectUri: string, state: string): string`
- Produces: `onLarkOAuthClicked(larkClientId: string, serverAddress: string): Promise<void>`

- [ ] **Step 1: Add the failing berry helper test**

Create `web/berry/src/utils/larkOAuth.test.js`:

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

- [ ] **Step 2: Install berry theme dependencies**

```powershell
Set-Location web/berry
npm install --legacy-peer-deps --no-package-lock
```

Expected: installation succeeds without creating a tracked lockfile.

- [ ] **Step 3: Run the test and verify it fails**

```powershell
Set-Location web/berry
npm test -- --watchAll=false src/utils/larkOAuth.test.js
```

Expected: FAIL because `./larkOAuth` does not exist.

- [ ] **Step 4: Add the berry pure helper implementation**

Create `web/berry/src/utils/larkOAuth.js`:

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

- [ ] **Step 5: Wire the berry orchestration function**

In `web/berry/src/utils/common.js`, add:

```js
import {
  buildLarkAuthorizeUrl,
  getLarkRedirectUri,
} from './larkOAuth';
```

Replace the Lark function with:

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

Keep the existing `showError`, `getOAuthState`, GitHub, and OIDC behavior unchanged.

- [ ] **Step 6: Run tests and commit**

```powershell
npm test -- --watchAll=false src/utils/larkOAuth.test.js
Set-Location ../..
git add web/berry/src/utils/larkOAuth.js web/berry/src/utils/larkOAuth.test.js web/berry/src/utils/common.js
git commit -m "feat: validate berry lark oauth origin"
```

Expected: tests pass.

---

### Task 7: berry Theme Availability and Call Sites

**Files:**
- Modify: `web/berry/src/config.js`
- Modify: `web/berry/src/views/Authentication/AuthForms/AuthLogin.js`
- Modify: `web/berry/src/views/Profile/index.js`

**Interfaces:**
- Consumes: Redux `siteInfo` and `isLarkOAuthAvailable(status)` from Task 6
- Consumes: `onLarkOAuthClicked(clientId, serverAddress)` from Task 6
- Produces: consistent login and binding availability in berry

- [ ] **Step 1: Declare the initial Lark state**

Add beside the existing GitHub fields in `web/berry/src/config.js`:

```js
lark_client_id: '',
lark_oauth_enabled: false,
```

- [ ] **Step 2: Update AuthLogin availability**

Import `isLarkOAuthAvailable` from `utils/larkOAuth`. Replace both `siteInfo.lark_client_id` checks used for third-party availability and Lark rendering with `isLarkOAuthAvailable(siteInfo)`. Call:

```js
onLarkOAuthClicked(siteInfo.lark_client_id, siteInfo.server_address)
```

- [ ] **Step 3: Update Profile binding availability**

Import `isLarkOAuthAvailable` from `utils/larkOAuth`. Render the bind button only when:

```js
isLarkOAuthAvailable(status) && !inputs.lark_id
```

Call:

```js
onLarkOAuthClicked(status.lark_client_id, status.server_address)
```

- [ ] **Step 4: Run tests and build berry**

```powershell
Set-Location web/berry
npm test -- --watchAll=false src/utils/larkOAuth.test.js
$env:DISABLE_ESLINT_PLUGIN='true'
npm run build
Remove-Item Env:DISABLE_ESLINT_PLUGIN
```

Expected: tests and production build pass; output moves to `web/build/berry`.

- [ ] **Step 5: Commit the berry UI unit**

```powershell
Set-Location ../..
git add web/berry/src/config.js web/berry/src/views/Authentication/AuthForms/AuthLogin.js web/berry/src/views/Profile/index.js
git commit -m "fix: enforce berry lark oauth availability"
```

---

### Task 8: Cross-Theme Build and Runtime Verification

**Files:**
- Verify generated, ignored outputs: `web/build/default`, `web/build/berry`, `web/build/air`
- No source files should be created in this task

**Interfaces:**
- Consumes: all previous tasks
- Produces: evidence that all themes build and the Go binary can embed them

- [ ] **Step 1: Run all focused frontend tests from clean theme roots**

```powershell
Set-Location web/default
npm test -- --watchAll=false src/components/larkOAuth.test.js
Set-Location ../air
npm test -- --watchAll=false src/components/larkOAuth.test.js
Set-Location ../berry
npm test -- --watchAll=false src/utils/larkOAuth.test.js
Set-Location ../..
```

Expected: three passing Jest runs.

- [ ] **Step 2: Run all Go tests**

```powershell
go test ./...
```

Expected: all packages pass.

- [ ] **Step 3: Build all themes through the project entry point**

```powershell
Set-Location web
sh build.sh
Set-Location ..
```

Expected: default, berry, and air each report a successful production build, with directories present under `web/build`.

- [ ] **Step 4: Verify embed-compatible output and compile the backend**

```powershell
Get-ChildItem web/build | Select-Object Name
go build -o one-api.exe .
```

Expected: output lists `default`, `berry`, and `air`; Go compilation succeeds. Keep `one-api.exe` untracked as required by `AGENTS.md`.

- [ ] **Step 5: Perform the configured air-theme smoke test**

Using the configured deployment at `http://192.168.0.86:3000` after rebuilding/redeploying:

1. Confirm `/api/status` returns `lark_oauth_enabled: true`, a non-empty `lark_client_id`, and `server_address: "http://192.168.0.86:3000"`.
2. Clear site storage once, open `/login`, and verify the Lark button appears after the status response without a second refresh.
3. Click Lark login and verify the authorization request contains `redirect_uri=http://192.168.0.86:3000/oauth/lark` after URL decoding.
4. Complete authorization and verify login succeeds.
5. Open the same deployment through a different origin such as `http://127.0.0.1:3000`; verify clicking Lark shows the address mismatch message and does not open Feishu.
6. Log in as root, open the air home page, and verify “飞书身份验证：已启用” appears.

- [ ] **Step 6: Confirm the worktree contains only intended source changes**

```powershell
git status --short
git diff --check
```

Expected: generated `web/build/*`, `one-api.exe`, and local runtime residue remain ignored; no whitespace errors or unrelated tracked changes appear.

- [ ] **Step 7: Stop if verification changed tracked files**

Expected: Step 6 reports no unexpected tracked changes. If a formatter or test changed a tracked file, do not stage it blindly; inspect the diff, return to the task that owns that file, repeat that task's test cycle, and use that task's commit message.
