-- Migration: 000117 down — 故意留空。
--
-- 000117 是"缺才补"的补齐迁移：同一个对象可能是它建的，也可能是上游
-- 000092-000098 正常跑出来的。在这里 DROP 会把健康库里正在使用的
-- browser_devices / mcp_metadata / memory_* / fork_snapshot_leases 一并删掉，
-- 连带丢失已配对的浏览器授权和 MCP 缓存。
--
-- 真要回滚：人工决定目标库该保留哪些对象，再单独处理；或者把版本 force 回
-- 000116（schema_migrations 只存当前版本一行），本迁移留下的对象无害。

DO $$ BEGIN RAISE NOTICE '[Migration 000117] down is a no-op by design'; END $$;
