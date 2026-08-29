# 08 — 前端「我的笔记」页面

**What to build:** 侧栏出现「我的笔记」入口，进入 `/platform/notes`：左侧笔记列表（首行标题 + 更新时间，倒序），右侧编辑/预览切换的 Markdown 编辑器；保存按钮 + Ctrl+S；未保存切换/离开有确认。学生与教师都能用（各自看各自的）。

**Blocked by:** 06 — 笔记数据层与 CRUD API

**Status:** ready-for-agent

**复用**：manual-knowledge-editor.vue 的工具栏（wrapSelection/heading/bullet/code/link）与安全预览管线（utils/security.ts 三件套）；保存逻辑换成笔记 API（api/me/notes.ts 新建）。菜单/路由注册参照 docs/川农笔记功能设计.md §5。

- [ ] 侧栏出现「我的笔记」菜单项（menu.ts + 四语言 menu.* 键），路由 /platform/notes 可达
- [ ] 列表：标题取首个 # 行（无则首行截断、空笔记"无标题"），按更新时间倒序，可新建/删除（删除有确认）
- [ ] 编辑/预览切换渲染正常（列表/代码块/链接均走 sanitize 管线）
- [ ] Ctrl+S 与保存按钮均落库；有未保存改动时切笔记/离开页面/关页有确认提示
- [ ] 学生账号全程可用且只能看到自己的笔记（教师登录看到的也是教师自己的空列表）
