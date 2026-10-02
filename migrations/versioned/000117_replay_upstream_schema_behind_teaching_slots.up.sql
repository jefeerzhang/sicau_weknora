-- Migration: 000117_replay_upstream_schema_behind_teaching_slots
--
-- 川农教学迁移曾经占用 versioned 000091-000098 号位，和上游 v0.8.2 的
-- 000092_mcp_metadata / 000093_browser_authorization / 000094_memory_consistency /
-- 000095_memory_vector_search / 000096_dingtalk_stream_only / 000097_session_fork /
-- 000098_fork_snapshot_lease 完全同号。重号让 golang-migrate 在加载目录时就拒绝
-- (source.ErrDuplicateMigration)，AUTO_MIGRATE 整轮失败，库停在教学号位上。
--
-- 000111-000116 把教学迁移顺延之后，重号解除了，但只解决了一半问题：
-- golang-migrate 只从库里记录的当前版本往上跑。一个已经按教学号位推进过的库
-- (backup/weknora-db.sql 记的是 version=93) 换基后只会跑 >93 的迁移，
-- 号位 <= 当前版本的这批上游 DDL 对它永远不会再执行。实测缺失：
-- browser_devices / browser_pairings / browser_task_interruptions /
-- mcp_metadata / mcp_services.usage_instructions（该库里全部不存在）。
--
-- 本迁移把这些对象按"缺才补"的形状重放一遍：健康库上是 no-op，
-- 停在 92-98 号位的库会补齐。
--
-- 只重放 schema，不重放 000094 末尾那两条 memory_items 数据修复 —— 第二条会把
-- superseded 记录改回 active，对已经修过的库盲跑会改写数据。真需要补那一步的库
-- 请人工单独执行 000094 的 UPDATE。

DO $$ BEGIN RAISE NOTICE '[Migration 000117] Replaying upstream schema hidden by teaching version slots'; END $$;

-- ── 000092_mcp_metadata ────────────────────────────────────────────────────
DO $$
BEGIN
    IF to_regclass('public.mcp_services') IS NULL THEN
        RAISE NOTICE '[Migration 000117] mcp_services absent · skipping mcp metadata';
        RETURN;
    END IF;

    ALTER TABLE mcp_services ADD COLUMN IF NOT EXISTS usage_instructions TEXT NOT NULL DEFAULT '';

    CREATE TABLE IF NOT EXISTS mcp_metadata (
        tenant_id BIGINT NOT NULL,
        service_id VARCHAR(36) NOT NULL REFERENCES mcp_services(id) ON DELETE CASCADE,
        principal VARCHAR(255) NOT NULL DEFAULT '',
        config_fingerprint VARCHAR(64) NOT NULL,
        tools JSONB NOT NULL,
        instructions TEXT NOT NULL DEFAULT '',
        server_name TEXT NOT NULL DEFAULT '',
        server_version TEXT NOT NULL DEFAULT '',
        server_description TEXT NOT NULL DEFAULT '',
        synced_at TIMESTAMPTZ NOT NULL,
        PRIMARY KEY (tenant_id, service_id, principal)
    );
    RAISE NOTICE '[Migration 000117] mcp metadata ready';
END $$;

-- ── 000093_browser_authorization ───────────────────────────────────────────
-- 自建表，无前置依赖。列定义与上游 000093 保持一致，只补 IF NOT EXISTS。
CREATE TABLE IF NOT EXISTS browser_devices (
 scope_key VARCHAR(32) PRIMARY KEY,
 id VARCHAR(32) NOT NULL UNIQUE,
 tenant BIGINT NOT NULL,
 "user" VARCHAR(36) NOT NULL,
 label VARCHAR(100) NOT NULL,
 token_hash VARCHAR(64) NOT NULL UNIQUE,
 previous_hash VARCHAR(64) NOT NULL DEFAULT '',
 previous_until TIMESTAMPTZ NOT NULL,
 expires_at TIMESTAMPTZ NOT NULL,
 renew_after TIMESTAMPTZ NOT NULL,
 created_at TIMESTAMPTZ NOT NULL,
 last_seen_at TIMESTAMPTZ NOT NULL,
 revoked_at TIMESTAMPTZ,
 owner VARCHAR(32) NOT NULL DEFAULT '',
 owner_url VARCHAR(500) NOT NULL DEFAULT '',
 lease_key VARCHAR(32) NOT NULL DEFAULT '',
 lease_until TIMESTAMPTZ NOT NULL
);
CREATE TABLE IF NOT EXISTS browser_pairings (
 scope_key VARCHAR(32) PRIMARY KEY,
 token_hash VARCHAR(64) NOT NULL UNIQUE,
 tenant BIGINT NOT NULL,
 "user" VARCHAR(36) NOT NULL,
 expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS browser_pairings_expiry ON browser_pairings(expires_at);
CREATE TABLE IF NOT EXISTS browser_task_interruptions (
 scope_key VARCHAR(32) NOT NULL,
 session VARCHAR(36) NOT NULL,
 PRIMARY KEY (scope_key, session)
);

-- ── 000094_memory_consistency（仅 schema）─────────────────────────────────
DO $$
BEGIN
    IF to_regclass('public.memory_subjects') IS NULL OR to_regclass('public.memory_items') IS NULL THEN
        RAISE NOTICE '[Migration 000117] memory tables absent · skipping memory consistency schema';
        RETURN;
    END IF;

    ALTER TABLE memory_subjects ADD COLUMN IF NOT EXISTS extraction_state JSONB;
    ALTER TABLE memory_items ADD COLUMN IF NOT EXISTS replaces_id VARCHAR(36) NOT NULL DEFAULT '';

    CREATE TABLE IF NOT EXISTS memory_extraction_sessions (
        tenant_id BIGINT NOT NULL,
        subject_id VARCHAR(512) NOT NULL,
        session_id VARCHAR(36) NOT NULL,
        revision BIGINT NOT NULL DEFAULT 0,
        cursor_at TIMESTAMP WITH TIME ZONE, cursor_id VARCHAR(36) NOT NULL DEFAULT '',
        pending BOOLEAN NOT NULL DEFAULT false,
        failure_count INTEGER NOT NULL DEFAULT 0,
        failure_code VARCHAR(64) NOT NULL DEFAULT '',
        failed_from_at TIMESTAMP WITH TIME ZONE, failed_from_id VARCHAR(36) NOT NULL DEFAULT '',
        failed_to_at TIMESTAMP WITH TIME ZONE, failed_to_id VARCHAR(36) NOT NULL DEFAULT '',
        failed_at TIMESTAMP WITH TIME ZONE, updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
        PRIMARY KEY (tenant_id, subject_id, session_id)
    );
    CREATE INDEX IF NOT EXISTS idx_memory_extraction_pending
        ON memory_extraction_sessions (tenant_id, subject_id, pending, updated_at, session_id);
    CREATE INDEX IF NOT EXISTS idx_memory_replaces
        ON memory_items (tenant_id, subject_id, replaces_id, status);
    RAISE NOTICE '[Migration 000117] memory consistency schema ready';
END $$;

-- ── 000095_memory_vector_search ───────────────────────────────────────────
-- 上游原文已经带存在性判断，照搬，只改 NOTICE 号。
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.tables WHERE table_name = 'memory_item_embeddings'
    ) THEN
        RAISE NOTICE '[Migration 000117] memory_item_embeddings absent · skipping';
        RETURN;
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'vector') THEN
        RAISE NOTICE '[Migration 000117] vector extension absent · memory keeps scoring in process';
    ELSE
        ALTER TABLE memory_item_embeddings ADD COLUMN IF NOT EXISTS embedding halfvec;
        RAISE NOTICE '[Migration 000117] memory_item_embeddings.embedding ready';
    END IF;

    CREATE INDEX IF NOT EXISTS idx_mem_emb_search
        ON memory_item_embeddings (tenant_id, subject_id, model_id, dims);
END $$;

-- ── 000096_dingtalk_stream_only ───────────────────────────────────────────
DO $$
BEGIN
    IF to_regclass('public.im_channels') IS NULL THEN
        RAISE NOTICE '[Migration 000117] im_channels absent · skipping dingtalk stream enforcement';
        RETURN;
    END IF;

    UPDATE im_channels SET mode = 'websocket', updated_at = CURRENT_TIMESTAMP
    WHERE platform = 'dingtalk' AND mode = 'webhook';
END $$;

-- ── 000097_session_fork ───────────────────────────────────────────────────
-- 上游原文已经全带 IF NOT EXISTS；sessions / messages 由 000000_init 建立。
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS parent_session_id VARCHAR(36);
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS forked_from_message_id VARCHAR(36);
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS fork_bootstrap JSONB;

CREATE INDEX IF NOT EXISTS idx_sessions_parent_session_id
    ON sessions (parent_session_id)
    WHERE parent_session_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_sessions_unconsumed_fork
    ON sessions ((fork_bootstrap ->> 'snapshot_id'))
    WHERE fork_bootstrap IS NOT NULL
      AND fork_bootstrap ->> 'consumed_at' IS NULL;

ALTER TABLE messages ADD COLUMN IF NOT EXISTS sandbox_checkpoint JSONB;

-- ── 000098_fork_snapshot_lease ────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS fork_snapshot_leases (
    snapshot_id VARCHAR(128) PRIMARY KEY,
    tenant_id BIGINT NOT NULL DEFAULT 0,
    sandbox_config_id VARCHAR(36) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_fork_snapshot_leases_created_at
    ON fork_snapshot_leases (created_at);

DO $$ BEGIN RAISE NOTICE '[Migration 000117] Upstream schema replay finished'; END $$;
