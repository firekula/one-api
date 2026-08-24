# Changelog

## v1.2.2 - 2026-08-25

### 构建

- Dockerfile 将 tiktoken 编码文件（`cl100k_base` / `o200k_base`）内嵌进镜像并预设 `TIKTOKEN_CACHE_DIR`，镜像在无外网环境下（如国内服务器）可直接启动；此前容器会卡在 `InitTokenEncoders`，端口不监听导致服务无法访问。
- 新增 `.gitattributes` 将 tiktoken 文件标记为 binary，防止 Windows 下换行符转换损坏数据文件。
- `.dockerignore` 排除 `data/`（本机数据库）与 `web/build/` 聚合产物，减小构建上下文。

## v1.2.1 - 2026-08-21

### 修复

- 飞书登录升级为官方新版 OAuth 2.0 授权码流程：登录页改用 `accounts.feishu.cn/open-apis/authen/v1/authorize`（三个主题统一，补齐 `response_type=code`），换 token 改用 `accounts.feishu.cn/oauth/v3/token`，用户信息改用 `open.feishu.cn/open-apis/authen/v1/user_info`。旧接口（`authen/v1/index`、`authen/v2/oauth/token`、passport 用户信息接口）均已被飞书官方标注为历史版本，继续使用会导致登录报「lark id 为空」。
- 飞书登录链路增加飞书接口错误码校验，失败时返回飞书返回的真实错误信息，不再误报「lark id 为空」。

### 构建

- Dockerfile 增加 `GOPROXY=https://goproxy.cn,direct`，国内网络环境可正常拉取 Go 依赖。
- docker-compose.yml 移除对已注释 `db` 服务的 `depends_on` 引用（新版 Compose 会拒绝启动），并挂载 tiktoken 编码缓存目录（`TIKTOKEN_CACHE_DIR`），解决容器启动卡在 tokenizer 下载、端口不监听的问题。
- web/build.sh 兼容 CRLF 行尾的 THEMES 文件，修复 Windows 下 `cd` 主题目录失败导致构建静默跳过的问题。

## v1.2.0 - 2026-08-12

### 新增

- 飞书登录新增启用开关（`LarkOAuthEnabled`）：default / air 主题可在「系统设置 → 配置登录注册」勾选「允许通过飞书登录 & 注册」，未启用时登录页不显示飞书入口、OAuth 回调直接拒绝；air 主题补齐飞书配置、登录按钮、回调路由与个人绑定。
- 数据迁移导入结果展示失败明细（default / air）：有失败项时列出具体失败原因，便于排查。

### 修复

- 修复数据迁移「导入数据」按钮在 default / air 主题不可见的问题：`Form.Button` 的 `content` + `as='label'` 组合会被 Semantic UI React 渲染成无文字的空元素，改为 `Button as='label'` 标准写法。
- 修复数据导入时引用已存在用户（用户名冲突被跳过）的 token 全部报「引用的用户不存在」的问题：用户跳过时同步建立旧 id → 现有用户 id 的映射，token / redemption 正确关联。

## v1.1.0 - 2026-08-12

### 新增

- 数据导出 / 导入：支持用户、令牌、渠道、兑换码与系统设置的 JSON 备份（合并导入、判重跳过、id 映射、事务回滚）。

### 修复

- 修复令牌验证瞬态错误误报 401，sqlite 启用 WAL 消除锁竞争。
