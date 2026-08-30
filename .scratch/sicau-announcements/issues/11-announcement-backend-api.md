# 11 — 公告数据层 + CRUD + 附件 API

**What to build:** 教师发公告（multipart：title + content + files[]），全员可读可下载附件。附件走现成 FileService 抽象（SaveFile/GetFile/DeleteFile），元数据存公告行 jsonb——随公告一次性提交，无编辑即无孤儿文件；删公告连带删附件文件与留言。

**Blocked by:** None — can start immediately

**Status:** ready-for-agent

**设计**：docs/川农公告板设计.md（迁移 000093 双表建齐：announcements + announcement_comments，后者供 ticket 12 用）。**路由仅注册 web JWT 路径**（IM 合成账号共享 user_id，同笔记 §7）。username 批量 hydrate 照抄 handler/tenant_member.go 的 GetUsersByIDs 模式。

- [ ] 迁移 000093：两张表（up/down 成对）
- [ ] GET /announcements 列表（不含正文与留言）；POST multipart 创建（contributor+ 守卫）：≤5 附件、单个 ≤50MB、扩展名白名单（pdf/doc/docx/ppt/pptx/xls/xlsx/zip/rar/7z/txt/md），文件走 storageResolver 解析的 FileService SaveFile，path 存 jsonb
- [ ] GET /:id 详情（全文 + 附件元数据 + 作者名）；DELETE /:id（作者或 admin）连带 FileService DeleteFile 每个附件 + 级联删留言
- [ ] GET /announcements/:id/attachments/:index：FileService GetFile 流式返回（登录即可），Content-Disposition 带原文件名
- [ ] 超限/白名单外 → 400；非作者且非 admin 删公告 → 403/404
- [ ] handler 测试：创建含附件全链（local FileService）、门禁（viewer 创建 403）、白名单与数量校验、作者/admin 删除语义、username hydrate
