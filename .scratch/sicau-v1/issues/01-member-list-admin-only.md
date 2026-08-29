# 01 — 成员名单仅教师可见

**What to build:** 教师打开成员管理页可以看到课程空间完整名单；学生（viewer）端不再有任何入口看到"这个课有哪些人"。直接调用成员列表接口的学生请求被拒绝。目的是落实 ADR-009-5：学生之间互不可见，防止名单/学号信息在学生端泄露。

**Blocked by:** None — can start immediately

**Status:** done (commit 待填：成员/邀请列表守卫 Admin+)

- [x] 学生账号（viewer）打开前端设置后，看不到任何成员管理/成员列表入口
- [x] 学生账号直接请求成员列表 API 返回 403（权限不足），而非脱敏数据
- [x] 教师（owner/admin）成员管理页功能不变：仍可查看名单、添加成员、改角色、移除成员
- [x] 排查其他可能向 viewer 泄露成员身份信息的接口（用户名/邮箱/学号字段），确认均已封闭或脱敏
  - GET /invitations（含被邀请人邮箱）一并收紧为 Admin+
  - TenantInfo 退出空间门禁仅 owner 拉名单（学生路径本来就不触接口），无需改
  - MyInvitationsDialog / GlobalInvitationBell 只含本人数据；TenantSelector 只含空间名
- [x] 前端对 403 场景不弹原始报错（入口已隐藏，深链落入 role-denied 兜底页）
