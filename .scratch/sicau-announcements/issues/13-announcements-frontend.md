# 13 — 前端「公告板」页面 + API 文档

**What to build:** 侧栏「公告板」入口 → /platform/announcements 单列公告流：顶部发布表单（contributor+：标题/内容/多选文件），公告卡片（标题、作者、时间、Markdown 渲染正文、附件下载、留言展开与发送）。另产出 docs/api/announcements.md。

**Blocked by:** 11、12

**Status:** ready-for-agent

**复用**：security.ts 渲染管线；username/time 格式照页面惯例本地定义；发布用 multipart FormData（参照 api/knowledge-base uploadKnowledgeFile）。菜单/路由注册同 notes 模式——**注意 components/menu.vue 的 topMenuItems/bottomMenuItems 白名单过滤器要放行 'announcements'**（ticket 08 的教训）。设计 docs/川农公告板设计.md §4。

- [ ] 侧栏出现「公告板」+ 路由可达 + 四语言 menu/页面文案
- [ ] 发布表单仅 contributor+ 可见；选文件显示文件名列表，提交走 multipart；成功后列表刷新
- [ ] 公告卡片：标题/作者/时间/正文渲染（sanitize 管线）/附件下载链接/留言展开
- [ ] 留言：展开加载正序列表、发送、删除（作者/admin 可见删除钮）
- [ ] docs/api/announcements.md 覆盖全部端点（鉴权/形状/错误码/限额），README 索引若有格式则同步
