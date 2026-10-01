# sync/upstream-v0.8.2 · Ticket 0 基线

> 日期：2026-10-01  
> 分支：`sync/upstream-v0.8.2` @ worktree `.worktrees/sync-upstream-v0.8.2`  
> 基座：`v0.8.2`（`3e8b0bfc8`）

## 已完成

1. 从 `upstream/v0.8.2` 建分支与 worktree。  
2. 川农 Postgres 迁移顺延为 **`000111`–`000116`**；SQLite 为 **`000031`–`000036`**。  
3. Route 2 桥接脚本 + 克隆脚本 + 回滚剧本：`docs/上游同步-v0.8.2-迁移与回滚.md`。  
4. 空库迁移验证（ParadeDB `v0.22.2-pg17` + `migrate/migrate:v4.18.1`）：**`version=116`，`dirty=false`**；教学表/列与上游 `mcp_metadata` 均存在。  
5. 模拟升级路径：在 `version=98` 且已有教学 schema 的库上桥接上游 `091`–`098`（version 仍为 98）→ `migrate up` → **`116|f`**；`111`–`116` 幂等通过。

## 未完成（后续 Ticket）

- A–G 川农行为重放（邀请 / 教师 / 学生封闭 / 笔记公告 UI / 品牌 / 对话 / 部署默认含 anydoc 0.2.4）  
- 真实克隆卷 Route 2（`weknora_postgres-data` 只读克隆）冒烟  
- Go / 前端教学测试 RED→GREEN  

## 回滚要点

失败默认丢弃克隆 volume；禁止对源卷 `down -v` / `force`。详见迁移与回滚文档。
