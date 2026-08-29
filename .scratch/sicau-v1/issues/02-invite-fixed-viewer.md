# 02 — 邀请链接固定 viewer

**What to build:** 教师生成课程分享邀请链接时，链接角色固定为 viewer：前端不再展示角色选择，后端对任何试图创建高于 viewer 角色分享链接的请求直接拒绝。课程协作者如需更高权限，走"成员管理页添加成员"的正规流程（Owner+ 专属），不走邀请链接。落实 ADR-009-4。

**Blocked by:** None — can start immediately

**Status:** ready-for-agent

- [ ] 教师在成员管理页生成分享链接，界面无角色选择控件（或控件固定显示 viewer 且不可改）
- [ ] 用该链接注册的新学生进入空间后角色为 viewer（只读）
- [ ] 直接调用邀请创建 API 并传入 owner/admin/contributor 角色时，后端拒绝（400/403），不产生高权限邀请
- [ ] 通过"添加成员"正规流程（Owner+）仍可授予 contributor 及以上角色
- [ ] invite_only 模式下链接注册全流程回归通过（含过期链接被拒的既有行为）
