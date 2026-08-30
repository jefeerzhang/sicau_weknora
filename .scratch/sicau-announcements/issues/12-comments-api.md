# 12 — 留言 API

**What to build:** 公告下方平铺留言：全员可发（纯文本），作者删自己、admin 删任何；按时间正序返回，作者名批量 hydrate。

**Blocked by:** 11 — 公告数据层（迁移已含 announcement_comments 表；留言挂在公告存在性之下）

**Status:** ready-for-agent

- [ ] GET /announcements/:id/comments：正序平铺 + 作者名 hydrate
- [ ] POST /announcements/:id/comments：{content} 纯文本；公告不存在 → 404；空内容 → 400
- [ ] DELETE /announcements/:id/comments/:cid：留言作者或 admin；他人/不存在 → 404
- [ ] handler 测试：留言 CRUD、权限三角（作者删自己 ✓ / admin 删他人 ✓ / 学生删他人 404）、公告不存在 404
