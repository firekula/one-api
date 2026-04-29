# One API 文档

欢迎来到 One API 的官方文档。这里汇集了部署、使用、架构和开发的全部资料，帮助你快速上手并深入理解项目。

---

## 按角色导航

| 角色 | 推荐阅读 |
|------|---------|
| 运维人员 | `getting-started/` 全部 |
| API 使用者 | `getting-started/user-manual.md`、`reference/admin-api.md` |
| 开发者 | `architecture/` 全部、`development/` 全部 |

## 按场景导航

- **我要部署** → `getting-started/quick-start.md`
- **我要配置环境变量** → `getting-started/configuration.md`
- **我要使用 API** → `getting-started/user-manual.md`
- **我要了解架构** → `architecture/overview.md`
- **我要添加新渠道** → `development/adaptor-development.md`
- **我要排查问题** → `reference/faq.md`
- **我要开发 / 贡献** → `development/setup.md` → `development/contribution-guide.md`

## 文档目录树

```
docs/
  README.md                          # 文档总索引
  getting-started/
    quick-start.md                   # 快速部署指南
    configuration.md                 # 环境变量与命令行参数完整参考
    user-manual.md                   # 使用说明
  architecture/
    overview.md                      # 系统架构总览
    data-model.md                    # 数据库表结构与 ER 关系
    relay-system.md                  # 请求中继系统
    multi-node.md                    # 多机部署架构
  development/
    setup.md                         # 开发环境搭建
    adaptor-development.md           # 渠道适配器开发指南
    contribution-guide.md            # 贡献规范
  reference/
    admin-api.md                     # 管理 API 完整参考
    faq.md                           # 常见问题与故障排查
```

## 外部资源

- **GitHub 仓库**：[songquanpeng/one-api](https://github.com/songquanpeng/one-api)
- **在线演示**：[https://openai.justsong.cn](https://openai.justsong.cn)
- **Docker Hub**：[justsong/one-api](https://hub.docker.com/r/justsong/one-api)
