# 单日用量上限与总览筛选设计

日期：2026-09-20
状态：已评审（brainstorming 流程）

## 背景与目标

两个面向"日常管理 + 复盘"的需求：

1. **单日用量上限**：给所有用户加单日用量天花板，防止个别账号（或 key 泄露）在一天内把整站额度/上游配额烧穿；管理员可对个别用户豁免或设置单独数值。
2. **总览筛选**：现有总览（`/dashboard`）只能看最近 7 天，且只能看自己，无法按时间段、用户、令牌、模型复盘。

改动前先澄清了一个事实：`/api/user/dashboard` 把 `user_id` 硬编码为**当前登录会话**（`controller/user.go:262-268` → `model/log.go:225-251` 的 `AND user_id = ?`），链路上没有任何按角色分支。因此管理员在总览看到的数字是**自己账号的消耗**，不是全站。全站视图目前只存在于日志页（`/api/log/*`，`AdminAuth`），且只有明细/总额，没有"按天 × 模型"的分组图。

## 已确认的设计决议

| 议题 | 决议 |
|---|---|
| 总览数据范围 | 管理员默认看全站，页面内可切回"仅自己"；普通用户始终只看自己，不显示切换 |
| 上限口径 | token 与 quota 各设一个上限（两个独立维度） |
| 豁免/单独上限的表达 | 每个维度一个三态字段：`null`/`0` = 跟随全局，`-1` = 豁免，`>0` = 单独上限 |
| 前端主题范围 | default、berry、air 三套全改 |
| air 的坏页面 | 修好并改造为 air 的"总览"页面 |
| 用户列表响应 | 允许改动，增加"今日用量"字段 |
| 图片请求 | 不纳入 token 上限，只受额度上限约束 |

## 范围

### 包含

- 新建 `daily_usage` 按天累计表，独立于 `LogConsumeEnabled`
- `User` 两个新的三态上限字段 + 两个全局默认选项
- 四个请求模态（文本/Anthropic/Audio/Image）的准入判定与结算记账
- `/api/user/dashboard` 扩展时间段、粒度、范围与三个筛选维度
- 三个主题的总览筛选栏、用户编辑表单、运营设置项、用户列表用量展示
- 顺带修复 6 个既有缺陷（见"顺带修复的既有缺陷"）

### 排除

- 按令牌维度的单日上限（本次只按用户；`logs.token_name` 已具备，后续可加）
- 上限预警（邮件/站内信）
- 周粒度的分桶聚合
- 日志表 DISTINCT 候选值接口（筛选先用精确匹配文本）

## 第一部分：单日用量上限

### 数据模型

新表 `daily_usage`（主库 `DB`，与 `User`/`Token` 同库；**不**放 `LOG_DB`）：

```go
type DailyUsage struct {
	Id               int    `json:"id" gorm:"primaryKey"`
	UserId           int    `json:"user_id" gorm:"uniqueIndex:idx_daily_usage_user_day,priority:1"`
	Day              string `json:"day" gorm:"type:varchar(10);uniqueIndex:idx_daily_usage_user_day,priority:2"`
	PromptTokens     int64  `json:"prompt_tokens" gorm:"bigint;default:0"`
	CompletionTokens int64  `json:"completion_tokens" gorm:"bigint;default:0"`
	Quota            int64  `json:"quota" gorm:"bigint;default:0"`
}
```

- `day` 为服务器本地时间的 `YYYY-MM-DD`，长度 10，唯一索引 `(user_id, day)`。按日期字符串分桶意味着跨天重置是自然的，不需要定时任务。
- 不设 `CreatedAt`/`UpdatedAt`：这是一张纯累加表，行只在其所属日期被写。需要排查时用 `logs` 交叉验证。
- `migrateDB`（`model/main.go:140`）里加 `AutoMigrate(&DailyUsage{})`。

**为什么读这张表而不是读 `logs`**：`model.RecordConsumeLog` 被 `config.LogConsumeEnabled` 门控，关掉日志后从 `logs` 算"今日已用"会永远得到 0，上限静默失效。累加表是独立的、无条件写入的事实来源。

### 用户侧字段

```go
DailyTokenLimit *int64 `json:"daily_token_limit" gorm:"bigint;default:0"`
DailyQuotaLimit *int64 `json:"daily_quota_limit" gorm:"bigint;default:0"`
```

**为什么用指针而不是 `int64`**：`User.Update`（`model/user.go:167-182`）用的是结构体形式的 `Updates`，GORM 会跳过零值字段——非指针字段一旦从 `-1` 或正数改回 `0`（跟随全局）会**静默不生效**。非 nil 指针会被 GORM 视为非零并写入，`0` 因此能正确落库。

语义上 `null` 与 `0` 等价（都表示跟随全局），所以不需要区分"从未设置"和"显式设为 0"。但前端把三态下拉切回"跟随全局"时必须**显式发送 `0`**（发 `null`/省略会被 GORM 跳过，改不回去）。

`UpdateUser`（`controller/user.go:367`）没有字段白名单，JSON 直接解到 `model.User`，所以新字段自动可被管理 API 设置；需要在其中补校验：值必须 `>= -1`，否则按现有惯例返回 `invalid_input`。`CreateUser`/`UpdateSelf` 使用 `cleanUser` 白名单，天然不涉及；新建用户后到编辑页设置。

### 全局默认选项

`common/config/config.go` 新增：

```go
var DailyTokenLimitDefault int64 = 0 // 0 = 不限制
var DailyQuotaLimitDefault int64 = 0 // 0 = 不限制
```

`model/option.go` 的 `InitOptionMap` 用 `strconv.FormatInt` 注册默认值、`updateOptionMap` 增加 `strconv.ParseInt` 分支；`controller/option.go` 的 `UpdateOption` 增加拒绝负值的校验。沿用 `QuotaForNewUser`/`PreConsumedQuota` 的做法**不加环境变量**——运行时以数据库 options 表为准，设置页是唯一配置入口。

### 有效上限解析

```go
// 返回 0 表示"不限"
func (user *User) EffectiveDailyTokenLimit() int64 {
	if user.DailyTokenLimit == nil {
		return clampNonNegative(config.DailyTokenLimitDefault)
	}
	if v := *user.DailyTokenLimit; v < 0 {
		return 0 // 豁免
	} else if v == 0 {
		return clampNonNegative(config.DailyTokenLimitDefault)
	} else {
		return v
	}
}
```

`EffectiveDailyQuotaLimit()` 同构。全局默认若被写成负数按 0（不限）处理。

### 判定与记账的执行路径

**准入是估算，结算是精确值。** token 的真实数量只有响应结束才知道（文本流式甚至在 `text.go:86` 的 goroutine 里结算），所以准入判定用 `今日已用 + 预估`：

- token 预估 = `prompt_tokens + max_tokens`（`max_tokens` 缺省按 0 计，即低估）
- quota 预估 = `getPreConsumedQuota(...)` 的返回值，即**短路前**的原始预扣估算。注意不能复用 `preConsumeQuota` 里那个可能被置 0 的 `preConsumedQuota` 变量：当 `userQuota > 100*preConsumedQuota` 时它会被改成 0（"额度充裕免预扣"优化），拿它做日限判定会让高额度用户永远不触发上限

判定必须在**额度预扣之前**，否则被拒请求还要走退款路径。

| 模态 | 准入判定点 | 结算记账点 |
|---|---|---|
| 文本 / chat | `relay/controller/helper.go:68` `preConsumeQuota`（在 `CacheDecreaseUserQuota` 之前，且在"用户额度充裕免预扣"短路之前） | `relay/controller/helper.go:97` `postConsumeQuota` |
| Anthropic | `relay/controller/anthropic.go:100` `preConsumeQuotaAnthropic` | `relay/controller/anthropic.go:123` `postConsumeQuotaAnthropic` |
| Audio | `relay/controller/audio.go:69` | `relay/billing/billing.go:23` `PostConsumeQuota`（`audio.go:219` 调用） |
| Image | `relay/controller/image.go:174` | `relay/controller/image.go:196` 的 defer |

要点：

- 文本与 Anthropic 传真实 `prompt_tokens`/`completion_tokens`；**Audio 与 Image 的 token 字段目前不可信**（`billing.go` 把 quota 写进 `PromptTokens`，`image.go` 写 0），因此这两个模态调用 `RecordDailyUsage(userId, 0, 0, quota)`，只受 quota 上限约束。
- `RecordDailyUsage` 与 `model.RecordConsumeLog` 并列调用，但**不受 `config.LogConsumeEnabled` 门控**。
- 失败/退款路径（`relay/billing/billing.go:11` `ReturnPreConsumedQuota`）不记账，与现有日志行为一致。
- 记账用 `clause.OnConflict` 原子累加（MySQL `ON DUPLICATE KEY UPDATE`、SQLite/PG `ON CONFLICT DO UPDATE`），每个请求一次点查 + 一次 upsert，与现有"每请求写一条 log"同量级，因此不引入 Redis 缓存。

model 层新增（`model/daily_usage.go`）：

- `Today() string` — 本地当日 `YYYY-MM-DD`
- `GetDailyUsage(userId int, day string) (*DailyUsage, error)` — 无记录时返回零值且不报错
- `RecordDailyUsage(userId int, promptTokens, completionTokens, quota int64) error` — 原子 upsert 累加
- `CheckUserDailyLimit(user *User, addTokens, addQuota int64) error` — 超限时返回 `*DailyLimitError`
- `GetDailyUsages(userIds []int, day string) (map[int]*DailyUsage, error)` — 供管理列表批量取

### 错误码与消息

```go
type DailyLimitError struct {
	Dimension string // "token" | "quota"
	Used      int64
	Limit     int64
	Estimated int64
}
```

`Error()` 组装可执行的中文消息（带上具体数字）：

- token：`今日 token 用量已达上限（已用 1234567 / 上限 1000000），请明日再试或联系管理员调整`
- quota：同结构，数值用 `common.LogQuota` 格式化

relay 侧映射为 `openai.ErrorWrapper(err, code, http.StatusForbidden)`：

| code | HTTP | 触发 |
|---|---|---|
| `insufficient_user_daily_tokens` | 403 | 今日 token 已用 + 预估 > 上限 |
| `insufficient_user_daily_quota` | 403 | 今日 quota 已用 + 预估 > 上限 |

与现有 `insufficient_user_quota`（403，`helper.go:77`）风格一致。relay 错误不走 i18n（现有代码即硬编码消息），本设计沿用。

### 管理界面

- **设置 → 运营设置**：两个数字输入——"单日 Token 上限（全局默认）"、"单日额度上限（全局默认）"，提示"0 表示不限制"。default/berry 走 i18n（`setting.operation.quota.*` 命名空间），air 沿用该文件现状的硬编码中文。
- **用户 → 编辑**：每个维度一行，三态下拉（跟随全局 / 豁免 / 自定义）+ 仅"自定义"时出现的数字框；前端校验自定义值必须 `> 0`。选"跟随全局"发 `0`，选"豁免"发 `-1`。
- **用户列表"统计信息"列**：追加"今日 token / 上限"与"今日额度 / 上限"，上限为 0 显示"不限"。

## 第二部分：总览时间段与筛选

### 接口契约

不新增接口，扩展 `GET /api/user/dashboard`（`router/api.go:43`，`selfRoute` + `UserAuth`）：

| 参数 | 类型 | 默认 | 说明 |
|---|---|---|---|
| `start_timestamp` | int64（unix 秒） | 本地今天往前 6 天的 00:00:00 | 起始，含 |
| `end_timestamp` | int64（unix 秒） | 本地今天 23:59:59 | 结束，含 |
| `granularity` | `day` \| `hour` | `day` | 分桶粒度 |
| `scope` | `self` \| `all` | `self` | `all` 需 `Role >= RoleAdminUser` |
| `username` | string | 空 | 精确匹配；非管理员传了也忽略（只能看自己） |
| `token_name` | string | 空 | 精确匹配 |
| `model_name` | string | 空 | 精确匹配 |

响应结构保持不变（`{success, message, data: [LogStatistic]}`），`LogStatistic.Day` 承载桶标签：day 粒度是 `2026-09-20`，hour 粒度是 `2026-09-20 14:00`。复用 `Day` 字段名可以让三个主题的前端分桶/补零逻辑改动最小。

校验规则：

| 情形 | 行为 |
|---|---|
| `start_timestamp > end_timestamp` | `success:false`，HTTP 400，`无效的时间范围` |
| `granularity` 非法值 | 退回 `day`（不报错） |
| `scope=all` 且非管理员 | HTTP 403，`无权查看全站统计` |
| `hour` 粒度且区间 > 90 天 | HTTP 400，`小时粒度最多查询 90 天` |
| `day` 粒度区间 > 366 天 | HTTP 400，`天粒度最多查询 366 天` |
| 非管理员传 `username` | 忽略该参数，按自己统计 |

### 模型层

```go
type LogStatisticQuery struct {
	UserId         int    // 0 = 不限用户（调用方已完成权限校验）
	Username       string
	TokenName      string
	ModelName      string
	StartTimestamp int64
	EndTimestamp   int64
	Granularity    string // "day" | "hour"
}

func SearchLogsByDayAndModel(query LogStatisticQuery) ([]*LogStatistic, error)
```

替换现有三参版本——它只有一个调用点（`controller/user.go:268`）。查询始终限定 `type = LogTypeConsume`（值 2），`GROUP BY 桶, model_name`。

### 分桶与时区修复

Go 侧用**本地时区**计算边界，替换现在的 `now.Truncate(24*time.Hour)`（那是 UTC 零点）：

```go
now := time.Now()
dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
```

SQL 侧三个方言都按**服务器本地时间**分桶：

| 方言 | day | hour |
|---|---|---|
| MySQL | `DATE_FORMAT(FROM_UNIXTIME(created_at), '%Y-%m-%d')` | `DATE_FORMAT(FROM_UNIXTIME(created_at), '%Y-%m-%d %H:00')` |
| PostgreSQL | `TO_CHAR(date_trunc('day', to_timestamp(created_at)), 'YYYY-MM-DD')` | `TO_CHAR(date_trunc('hour', to_timestamp(created_at)), 'YYYY-MM-DD HH24:00')` |
| SQLite | `strftime('%Y-%m-%d', datetime(created_at, 'unixepoch', 'localtime'))` | `strftime('%Y-%m-%d %H:00', datetime(created_at, 'unixepoch', 'localtime'))` |

**明确假设**：应用进程的本地时区 == 数据库会话时区。MySQL 分支今天已隐含该假设（`FROM_UNIXTIME` 走会话时区）；PostgreSQL 的 `date_trunc` 同样走会话 `TimeZone`；SQLite 原来按 UTC 分桶，这次补 `'localtime'` 修饰符与其他两者对齐。该假设会写进代码注释与用户手册。

### 各主题前端改动

三套主题互不复用代码，各自实现筛选栏；三个筛选维度统一用**精确匹配文本输入**，与各主题日志页现有做法一致（带候选值的下拉需要新增按日志 DISTINCT 的接口，本次不做）。

**default**（recharts + semantic-ui-react，无日期库，日志页的做法是原生输入）：

- 在 `dashboard-container` 内、图表之上插入筛选栏：快捷区间按钮（今天 / 近 7 天 / 近 30 天）+ 两个 `<input type='date'>` + 管理员"全站 / 仅自己"切换 + 用户/令牌/模型文本输入 + 查询按钮。
- 筛选状态进入 `useEffect` 依赖，触发重新拉取。
- 删除 `processTimeSeriesData`/`processModelData` 里写死的 `sevenDaysAgo` 逻辑，改为按请求区间与粒度生成桶、对缺失桶补零。
- `formatDate` 支持 `YYYY-MM-DD HH:00` 形式的标签。

**berry**（MUI + ApexCharts）：

- `views/Dashboard/index.js` 外层 Grid 的第一个子项加 `<Grid item xs={12}>` 筛选栏，用 `@mui/x-date-pickers` 的 `DatePicker`/`DateTimePicker`（日志页 `TableToolBar.js` 已在用）。
- `utils/chart.js` 的 `getLastSevenDays()` 换成按区间生成桶的函数；`getBarDataGroup` 里 `lastSevenDays.indexOf(item.Day)` 的定位方式改为按返回桶直接映射（现在桶不在数组里会丢数据）。

**air**（Semi UI + 命令式 `@visactor/vchart`，当前无可用总览）：

- 把死页面 `pages/Detail` 改造为 `pages/Dashboard`（路由 `/dashboard`），`App.js` 注册 `<Route>`，`SiderBar.js` 加"总览"菜单项并更新 `routerMap`，**去掉永不成立的 `enable_data_export` 门控**。
- 筛选栏用已有的 `Form.DatePicker` + 时间粒度选择（小时 / 天，去掉原来无效的"周"）+ 管理员用户名/令牌名/模型名输入 + 全站/仅自己切换。
- 图表从"仅首次 mount 用 `new VChart(spec, {dom})` 建图"改为筛选变化时 `updateSpec` + `reLayout`。

### 用户列表增强

不改 `model.User`（新增字段会改变备份导出格式，可能影响 `model/backup_test.go` 的往返比较）。在 `controller/user.go` 定义响应 DTO：

```go
type UserListItem struct {
	*model.User
	TodayTokens              int64 `json:"today_tokens"`
	TodayQuota               int64 `json:"today_quota"`
	EffectiveDailyTokenLimit int64 `json:"effective_daily_token_limit"`
	EffectiveDailyQuotaLimit int64 `json:"effective_daily_quota_limit"`
}
```

`GetAllUsers` 取到当页用户后，用一次 `GetDailyUsages(ids, Today())` 填这四个字段（每页一次查询，不是每行一次）。`effective_*` 为 0 表示不限。三套主题的用户表格在"统计信息"列展示"今日 1.2M / 5M"。

## 顺带修复的既有缺陷

改动范围内顺带修掉，都是这次功能会踩到的：

1. **总览日期边界用 UTC** — `controller/user.go:265` 的 `now.Truncate(24*time.Hour)` 是 UTC 零点，与 SQL 的本地时区分桶口径不一致。改为本地时区边界（本次设计已包含）。
2. **SQLite 分桶按 UTC** — `model/log.go` 的 SQLite 分支缺 `'localtime'`，与 MySQL/PostgreSQL 分支行为不一致。补上。
3. **default 总览越界崩溃** — `web/default/src/pages/Dashboard/index.js:145` 直接 `dailyData[item.Day].requests += ...`，加了自定义时间段后必然触发。改为判空后按需创建桶。
4. **default 总览"今天"判断用 UTC** — `calculateSummary` 里 `new Date().toISOString().split('T')[0]` 改为本地日期。
5. **default 总览绕过错误拦截器** — 该页用裸 `axios`（`index.js:16,71`）而非 `helpers/api` 的 `API`，请求失败只在 console 留一行、界面无提示。换用 `API`。
6. **air"数据看板"是死页面** — 它请求的 `/api/data/`、`/api/data/self/`（`web/air/src/pages/Detail/index.js:177-179`）在本仓库不存在（只剩 `/api/data/export`、`/api/data/import`），且菜单项被 `localStorage.enable_data_export === 'true'` 门控，而 `/api/status` 从不返回该字段，所以永远隐藏。按上文改造为可用的"总览"。

## 受影响文件清单

### 后端

| 文件 | 改动 |
|---|---|
| `common/config/config.go` | 新增 `DailyTokenLimitDefault`、`DailyQuotaLimitDefault` |
| `model/option.go` | `InitOptionMap` 注册 + `updateOptionMap` 两个 `ParseInt` 分支 |
| `controller/option.go` | 两个新键拒绝负值 |
| `model/user.go` | 两个 `*int64` 字段 + `EffectiveDailyTokenLimit`/`EffectiveDailyQuotaLimit` |
| `model/daily_usage.go` | **新增**：模型、`Today`、`GetDailyUsage`、`RecordDailyUsage`、`CheckUserDailyLimit`、`GetDailyUsages`、`DailyLimitError` |
| `model/main.go` | `migrateDB` 加 `AutoMigrate(&DailyUsage{})` |
| `model/log.go` | `LogStatisticQuery` + 重写 `SearchLogsByDayAndModel`（区间/粒度/三维筛选/本地时区分桶） |
| `controller/user.go` | `GetUserDashboard` 参数解析与权限校验；`UpdateUser` 的新字段校验；`UserListItem` DTO + `GetAllUsers` 填充今日用量 |
| `relay/controller/helper.go` | 准入判定 + 结算记账（文本/chat） |
| `relay/controller/anthropic.go` | 准入判定 + 结算记账 |
| `relay/controller/audio.go` | 准入判定 + 结算记账（仅 quota） |
| `relay/controller/image.go` | 准入判定 + 结算记账（仅 quota） |
| `relay/billing/billing.go` | Audio 结算路径的记账调用 |

### 前端

| 主题 | 文件 |
|---|---|
| default | `src/pages/Dashboard/index.js`、`Dashboard.css`、`src/pages/User/EditUser.js`、`src/components/UsersTable.js`、`src/components/OperationSetting.js`、`src/locales/zh|en/translation.json` |
| berry | `src/views/Dashboard/index.js`、`src/utils/chart.js`、`src/views/User/component/EditModal.js`、`src/views/User/TableRow.js`、`src/views/Setting/component/OperationSetting.js` |
| air | `src/pages/Detail/*` → `src/pages/Dashboard/*`、`src/App.js`、`src/components/SiderBar.js`、`src/pages/User/EditUser.js`、`src/components/UsersTable.js`、`src/components/OperationSetting.js` |

文案：default/berry 的总览筛选栏与用户编辑表单走 i18n（新增 `dashboard.filters.*`、`user.edit.daily_*`、`setting.operation.quota.daily_*`、`user.table.today_usage`，`zh`/`en` 两份同步）；air 沿用该主题现状的硬编码中文。

### 文档

- `docs/getting-started/user-manual.md`："额度规则"下新增"单日用量上限"小节（三态语义、0 表示不限、豁免、仅约束 `/v1` 转发流量、跨天重置）。
- `docs/getting-started/configuration.md`：若该文件列举运营设置项则补充两个新选项（目前该文件是环境变量参考，本次不新增环境变量）。

## 实施顺序

两个功能彼此独立，但都依赖后端先行。按四阶段推进，每阶段结束都能独立验证：

1. **后端：日限**（`daily_usage` 表 + User 字段 + 全局选项 + 四个模态的判定与记账）。验证：model 层测试通过，手工设小上限确认拦截生效。
2. **后端：总览查询**（`LogStatisticQuery` + 重写查询 + `/api/user/dashboard` 参数与权限 + `UserListItem`）。验证：分桶/筛选测试通过，`curl` 带参数确认返回正确。
3. **前端：default 主题**（筛选栏 + 用户编辑 + 运营设置 + 用户列表用量）。default 是你当前使用的主题，先把交互跑通。
4. **前端：berry 与 air 主题**（各自实现同样的交互，air 含死页面改造）。

第 1、2 阶段无共同改动面，可并行；第 3、4 阶段依赖各自阶段确定下来的接口契约（本文件已冻结）。

## 测试与验证

1. **model 层：上限解析与记账**（新增 `model/daily_usage_test.go`，临时 SQLite，仿 `model/token_test.go`）
   - `EffectiveDailyTokenLimit`/`EffectiveDailyQuotaLimit`：`nil`、`0`、正数、`-1`、全局默认非零的组合
   - `RecordDailyUsage` 累加正确，且不同 `day` 落在不同行
   - `CheckUserDailyLimit`：未超限通过、恰好等于上限通过、超出返回 `DailyLimitError`（维度与数字正确）、豁免用户在上限很小时仍通过
2. **model 层：分桶与筛选**（内存 SQLite 作为 `LOG_DB`）
   - 在本地时区 23:59 与次日 00:00 插入日志，断言落在不同 day 桶（时区正确性的核心断言）
   - hour 粒度分桶标签格式
   - `username`/`token_name`/`model_name` 过滤生效
   - `UserId = 0` 时返回全部用户（全站视图的基础）
3. **controller 层**（`httptest`，仿 `controller/data_test.go`）
   - `scope=all` 非管理员 → 403
   - `start > end` → 400
   - hour 粒度超长区间 → 400
4. **构建**：`go test ./...` 与 `go build -o one-api .` 通过；三套主题分别 `npm run build` 通过（`web/build/` 被 gitignore，产物不入库）
5. **手工验收**（`CGO_ENABLED=1 go run .`，SQLite）
   - 把某用户 `daily_token_limit` 设为 100，第一次请求成功、第二次被拒，错误码与消息正确，`daily_usage` 行数值与实际用量一致
   - 管理员总览默认全站，切"仅自己"数字变化正确；切时间段/粒度、按用户/令牌/模型筛选，结果与日志页对得上
6. 全量 `go test ./...` 无回归——特别留意备份导出/导入往返测试（`model/backup_test.go`），本次不改 `model.User` 正是为了避开这个风险

## 已知取舍与风险

- **准入是估算**：无 `max_tokens` 的请求按 0 估输出，因此单次请求最多能超出上限"一个响应"的量。这是"准入可拦、结算精确"两害相权的结果。
- **并发在途请求**可轻微超限（多个请求同时通过准入）。超出量以在途请求数为界。
- **时区耦合**：按天分桶要求应用进程时区与数据库会话时区一致。多节点部署若各节点时区不一致，分桶会偏移。已在文档与注释中写明该前提。
- **每请求额外两次 DB 访问**（一次点查 + 一次 upsert），不做 Redis 缓存。先保证正确性；若量级成为问题，可在 `CheckUserDailyLimit` 前加 Redis 计数器并把 DB 作为事实来源。
- **用户/令牌/模型筛选是精确匹配文本**，没有候选值下拉，输入需要与日志里记录的值完全一致。
- **豁免只针对本功能**：豁免用户仍受账户余额与令牌额度限制，上限是额外的一层约束。
- **仅约束 `/v1` 转发流量**：管理后台操作、导出等不消耗 token，因此不受上限影响（也意味着管理员不会把自己锁在后台之外）。

## 明确不做（YAGNI）

- 周粒度分桶聚合
- 日志表 DISTINCT 候选值接口与下拉筛选
- 上限预警（邮件/站内信）与总览上的"今日用量/上限"进度条（`daily_usage` 已让这两项随时可加）
- 按令牌维度的单日上限
- 总览数据导出
- 图片请求的 token 估算规则（本次图片只受额度上限约束）
- 顺手重构 air/berry 表格里硬编码中文等既有问题
