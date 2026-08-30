# 10 — API 文档（docs/api/notes.md）

**What to build:** 按 repo 惯例为笔记功能补 API 文档：docs/api/notes.md 覆盖 /me/notes 与 /me/notes/images 全部端点——方法、路径、请求/响应形状、错误码、限额表、图片 capability 语义（owner-only）与缓存头。

**Blocked by:** 06、07（文档需覆盖最终实现的全部端点）

**Status:** ready-for-agent

- [ ] docs/api/notes.md 存在且覆盖 7 个端点（5 CRUD + 上传 + 读图/删图）
- [ ] 每个端点含：鉴权要求、请求/响应 JSON 形状、错误码表（400/401/403/404）
- [ ] 限额表：200 篇/人、1MB/篇、2MB/图、200 图/人，及对应错误信息原文
- [ ] 图片访问语义与缓存策略一段说明（owner-only、private immutable、GC 行为）
