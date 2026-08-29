# 07 — 笔记图片上传与访问

**What to build:** 学生在笔记里能贴图：上传接口收图（≤2MB/张、≤200 张/人），存 per-user 图片表；读取走 capability URL（36 位 UUID 即凭证，需登录）。预览渲染时可经认证取回字节。

**Blocked by:** None — can start immediately（与 06 无依赖，同迁移文件需协调：两张表放同一个 000092，或本票用 000093）

**Status:** ready-for-agent

**注意**：若 06 已落 000092（仅 tenant_notes 表），本票迁移用 000093 建 tenant_note_images；若先行，可合并。设计见 docs/川农笔记功能设计.md §3/§4。

- [ ] 迁移建 tenant_note_images 表（含 BYTEA bytes 列）+ 用户索引
- [ ] POST /me/notes/images（multipart）校验：≤2MB、白名单 mime（image/png jpeg gif webp）、每人 ≤200 张；成功返回 {url:"/api/v1/me/notes/images/<id>", id}
- [ ] GET /me/notes/images/:id 返回字节 + 正确 Content-Type；登录即可（capability URL 语义，文档已注明折衷）
- [ ] 超限/超类型返回 400，错误信息可读
- [ ] handler 测试：上传成功形状、超 2MB 400、白名单外 mime 400
