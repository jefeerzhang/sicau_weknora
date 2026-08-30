# 06 — 笔记数据层与 CRUD API

**What to build:** 学生（任何登录成员）拥有完全私有的 Markdown 笔记：能新建、读取、编辑、删除，且只能看到自己的。教师与其他学生对任何人笔记均不可见——不存在"列全空间笔记"的端点。限额（200 篇/人、单篇 1MB）生效。

**Blocked by:** None — can start immediately

**Status:** ready-for-agent

**模仿对象**：user-favorites 链路（000047）+ env-vars 的 /me 路由前缀（000089）。设计见 docs/川农笔记功能设计.md §3/§4/§7。

**重要**：迁移 000092 归本票，**一次建齐两张表**（tenant_notes + tenant_note_images）——ticket 07 只写图片逻辑，不再建表。/me/notes 仅注册到 web JWT 认证路径（IM 合成账号共享 user_id，绝不挂 IM 认证，见设计文档 §7）。

- [ ] 迁移 000092：两张表 + 用户索引（up/down 成对）
- [ ] GET /me/notes 返回 id/**title**/updated_at 列表（不含 content）；title 由服务端按 N-6 规则派生：首个 `#` 标题 → 无则首个非空行截 50 字 → 空笔记为空串（前端显示"无标题"）
- [ ] POST /me/notes 创建；第 201 篇返回 400；content 超 1MB（UTF-8 字节）返回 400
- [ ] GET/PUT/DELETE /me/notes/:id 均校验 owner（非本人 404，不泄露存在性）
- [ ] DELETE /me/notes/:id 删除后调用图片 GC 钩子（若 ticket 07 已合入则 GC 生效，未合入为 no-op）
- [ ] 限额实现于事务内（count-then-insert，接受并发近似，见设计文档 §7）；并发用例：两并行 POST 至多成功一篇在第 200/201 边界（可接受 flaky 则标注）
- [ ] 所有端点用户身份取自 auth ctx（前端不传 user_id）；未登录 401；**仅注册 web JWT 路径**
- [ ] handler 测试：owner 隔离（B 读不到 A 的）、限额两条、title 派生三态（#标题/首行/空）、正常 CRUD 全链
