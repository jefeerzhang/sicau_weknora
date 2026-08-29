# 06 — 笔记数据层与 CRUD API

**What to build:** 学生（任何登录成员）拥有完全私有的 Markdown 笔记：能新建、读取、编辑、删除，且只能看到自己的。教师与其他学生对任何人笔记均不可见——不存在"列全空间笔记"的端点。限额（200 篇/人、单篇 1MB）生效。

**Blocked by:** None — can start immediately

**Status:** ready-for-agent

**模仿对象**：user-favorites 链路（000047）+ env-vars 的 /me 路由前缀（000089）。设计见 docs/川农笔记功能设计.md §3/§4。

- [ ] 迁移 000092 建 tenant_notes 表 + 用户索引（up/down 成对）
- [ ] GET /me/notes 返回 id/preview/updated_at 列表（不含 content），按 updated_at 倒序，preview 为首行截 50 字
- [ ] POST /me/notes 创建；第 201 篇返回 400；content 超 1MB（UTF-8 字节）返回 400
- [ ] GET/PUT/DELETE /me/notes/:id 均校验 owner（非本人 404，不泄露存在性）
- [ ] 所有端点用户身份取自 auth ctx（前端不传 user_id）；未登录 401
- [ ] handler 测试：owner 隔离（B 读不到 A 的）、限额两条、正常 CRUD 全链
