# 07 — 笔记图片上传、访问与 GC

**What to build:** 学生在笔记里能贴图：上传（≤2MB/张、≤200 张/人、magic bytes 白名单），存 per-user 图片表；图片仅 owner 可读（非本人 404）；支持主动删图与删笔记时的 GC，配额可释放；响应带缓存头。

**Blocked by:** 06 — 笔记数据层与 CRUD API（迁移 000092 已含 tenant_note_images 表；GC 钩子挂在 06 的删除端点上）

**Status:** ready-for-agent

**设计**：docs/川农笔记功能设计.md §3.1/§4/§7。capability URL 折衷已废除——读取一律 owner 校验，前端 authed fetch hydration 不受影响。

- [ ] 图片逻辑接入 000092 已建的 tenant_note_images 表（不新建迁移）
- [ ] POST /me/notes/images（multipart）校验：≤2MB、**嗅探 magic bytes**（不信任 Content-Type 头）+ 白名单 image/png|jpeg|gif|webp、每人 ≤200 张；成功返回 {url, id}
- [ ] GET /me/notes/images/:id：**仅 owner，非本人 404**；响应带 `Cache-Control: private, max-age=31536000, immutable` + `ETag`
- [ ] DELETE /me/notes/images/:id：owner 主动删图释放配额
- [ ] 删笔记 GC：删除笔记后扫描该用户剩余笔记的图片引用集合，回收仅被被删笔记引用的图片（best-effort，GC 失败不影响笔记删除）
- [ ] handler 测试：上传形状、超 2MB 400、白名单外/伪造 Content-Type 400（magic sniff 生效）、B 读 A 的图 404、删图后 404、GC 回收
