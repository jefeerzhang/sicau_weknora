<p align="center">
  <picture>
    <img src="./docs/images/logo.png" alt="WeKnora · 川农教学版" height="120"/>
  </picture>
</p>

<p align="center">
  <a href="./LICENSE">
    <img src="https://img.shields.io/badge/License-MIT-ffffff?labelColor=d4eaf7&color=2e6cc4" alt="License"/>
  </a>
  <a href="./CHANGELOG.md">
    <img alt="Version" src="https://img.shields.io/badge/version-0.8.0-2e6cc4?labelColor=d4eaf7"/>
  </a>
  <a href="./docs/超级管理员上手手册.md">
    <img alt="Quick Start" src="https://img.shields.io/badge/快速开始-上手手册-4e6b99">
  </a>
</p>

<h1 align="center">WeKnora · 川农教学版</h1>

<p align="center">
面向四川农业大学的教学场景定制 fork。基于 <a href="https://github.com/Tencent/WeKnora">Tencent/WeKnora</a> 上游，为教师授课与学生自主学习提供 RAG（检索增强生成）+ 课程公告 + 学生笔记一体化平台。
</p>

---

## 项目是什么

WeKnora · 川农教学版是一套为高校教师与学生设计的私有化知识库系统。核心思路：教师把课件、参考资料、讲义录进系统，学生用自然语言提问获得答案，同时用课程公告传达通知、用私人笔记沉淀自己的理解。

整个系统围绕三个角色展开：

- **超级管理员**：唯一负责平台治理、任命教师的人，本身自动具备教师能力。
- **教师**：被超级管理员任命后，可以创建自己的工作空间（通常一门课一个空间），上传资料、配置模型、邀请学生。
- **学生**：通过教师发出的邀请链接注册，进入被授权的工作空间，默认仅能查询与记录，不能上传资料或管理成员。

详细的领域语言（每个角色与概念的精确定义）见 [CONTEXT.md](./CONTEXT.md)；面向终端用户的运维说明见 [docs/超级管理员上手手册.md](./docs/超级管理员上手手册.md)。

## 特性

### 教学三件套

- **课程公告板**：教师发布 Markdown 公告，可附加课件压缩包（≤50 MB/单条，≤5 个附件），所有空间成员可读、可留言；作者或空间管理员可删除（级联清理附件与留言）。
- **学生私人笔记**：每个学生在自己的私有命名空间内创建 Markdown 笔记，支持本地草稿、自动保存、贴图（图片走专属端点，仅本人可读）。教师与同学均不可见——这是学生端唯一可写数据面。
- **邀请制注册**：公开注册已关闭。教师在课程空间生成多次使用的限时邀请链接，学生点击链接注册即入课，默认角色 `viewer`，最小权限。

### 双层权限模型

系统使用两层正交的权限控制：

- **平台身份标签**：超级管理员 / 教师 / 学生。决定"能不能教、能不能被任命"。
- **空间角色**：`viewer < contributor < admin < owner`。决定"在这个工作空间能做什么"。

超级管理员是"复合身份"——平台治理能力 + 教师能力都具备，但不因此自动管理其他教师的空间。详见 [docs/RBAC说明.md](./docs/RBAC说明.md)。

### 基础 RAG 能力（沿用上游）

- **知识库**：CRUD、文件夹组织、文档上传与解析（PDF / Word / PPT / Excel / Markdown 等）。
- **文档解析**：内置 docreader 服务（gRPC），支持 MinerU（含公式 / 表格 / OCR）、PaddleOCR-VL、本地解析器。
- **对话与 Agent**：流式问答、会话历史、引用来源、Agent 编辑器、MCP 工具接入、Skill 插件。
- **长期记忆**：可按工作空间启用，自动从对话中提取用户的兴趣与事实。
- **检索增强**：向量检索（Postgres / Qdrant / Milvus / Weaviate 可选）、重排序、可选图 RAG。

### 模型配置

按工作空间隔离。每个教师空间可以独立绑定：

- **LLM**：OpenAI 兼容协议（OpenAI / DeepSeek / 智谱 / Ollama 等）。
- **Embedding**：同上，OpenAI 兼容即可。
- **Rerank**：可选。
- **VLLM**：可选视觉语言模型。

模型凭据用 `${ENV_VAR}` 占位，运行时由系统解析并用 `SYSTEM_AES_KEY`（AES-256）加密存储。详见 [docs/BUILTIN_MODELS.md](./docs/BUILTIN_MODELS.md) 与 `config/builtin_models.yaml`。

### 一键部署

单一 `docker compose up -d` 即可起全栈。Postgres（含向量能力）、Redis 内置，无需额外部署数据库。

## 架构

```
┌─────────────────────────────────────────────────────────────┐
│  Frontend  (Vue 3 + TDesign + Vite)                         │
│  Nginx 代理 → /api/* 反代到 App                             │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│  App  (Go 1.22 + Gin)                                       │
│  ├─ handler / service / types / repository                  │
│  ├─ Agent / MCP / Skill / Memory / Sandbox                  │
│  └─ 流式响应 + JWT 鉴权 + 双层 RBAC 中间件                  │
└───────┬─────────────────────────────────┬───────────────────┘
        │                                 │
        ▼                                 ▼
┌──────────────────┐           ┌────────────────────────────┐
│  Postgres        │           │  Docreader (gRPC)          │
│  ├─ 事务          │ ◄────────►│  MinerU / PaddleOCR-VL     │
│  ├─ 向量检索       │           │  PDF / Office 解析          │
│  └─ ParadeDB     │           └────────────────────────────┘
└──────────────────┘
        ▲
        │
┌──────────────────┐
│  Redis           │
│  ├─ 流式通道      │
│  └─ 缓存          │
└──────────────────┘
```

**核心模块职责**：

- **frontend**：Vue 3 单页应用，TDesign 组件库，Vite 构建。
- **app**：Go 后端，单体仓库内含 handler / service / repository 三层。
- **docreader**：独立 gRPC 服务，负责文档解析与图表抽取，可横向扩展。
- **postgres**：事务数据库（ParadeDB 同时提供向量检索与全文检索）。
- **redis**：流式通道（Server-Sent Events）+ 缓存。

## 快速开始

### 环境要求

- Docker 24+
- 8 GB 可用内存
- 20 GB 可用磁盘
- 可访问 OpenAI 兼容 LLM / Embedding 服务（本地 Ollama 也可）

### 安装

```bash
# 1. 克隆仓库
git clone <your-fork-url> weknora-sicau && cd weknora-sicau

# 2. 复制环境变量模板
cp .env.example .env

# 3. 按需修改 .env（至少填写 LLM 与 Embedding 凭据）

# 4. 启动全栈
docker compose up -d
```

启动完成后访问 [http://localhost:8089](http://localhost:8089)。

### 首次登录

系统已预设唯一超级管理员账号：

| 项目 | 预设值 |
|------|--------|
| 登录方式 | **邮箱**（不是用户名） |
| 邮箱 | `admin@admin.com` |
| 初始密码 | `admin123` |

首次登录会被强制要求修改密码。新密码要求 8-32 位，至少同时包含字母与数字。

> 登录后第一件事：在「系统设置 → 教师管理」里任命若干教师。其他事项（建工作空间、配模型、邀请学生）由教师完成。

## 核心概念

### 平台身份 vs 空间角色

| 维度 | 取值 | 谁能改 |
|------|------|--------|
| 平台身份 | 超级管理员 / 教师 / 学生 | 仅超级管理员可任命教师；超级管理员由部署期 bootstrap |
| 空间角色 | owner / admin / contributor / viewer | 由工作空间 owner 在成员页调整 |

一个用户可以同时拥有"超级管理员"平台身份和某个工作空间的 `contributor` 角色——两者独立。

### 工作空间

工作空间是教学的最小单位，通常一门课对应一个工作空间。空间内包含：

- 知识库与文档
- 模型绑定（LLM / Embedding / Rerank）
- Agent 模板
- 成员名单（学生通过邀请链接进入）
- 公告与附件

### 注册模式

`WEKNORA_AUTH_REGISTRATION_MODE=invite_only`（默认）。公开自助注册已关闭，陌生人无法自己注册成"业主"。

### 模型隔离

模型凭据与配置按 `tenant_id`（工作空间）维度存储。教师 A 配的 DeepSeek Key 仅在自己的工作空间生效，不会泄漏给教师 B 的工作空间。

## 开发者指南

### 本地开发

```bash
# 启动依赖服务（postgres / redis / docreader），app 与 frontend 走本地 hot reload
make dev-start

# 看日志
make dev-logs

# 停止
make dev-stop
```

或者使用更细粒度的脚本：

```bash
# 启动 Ollama + 全部依赖 + app
./scripts/start_all.sh --ollama

# 仅检查环境
./scripts/check-env.sh
```

### 目录结构

```
weknora-sicau/
├── cmd/                    # Go 入口
├── internal/               # 后端核心
│   ├── handler/            # HTTP / gRPC handler
│   ├── service/            # 业务逻辑
│   ├── repository/         # 数据访问
│   ├── types/              # 领域类型
│   ├── middleware/         # Gin 中间件
│   ├── agent/              # Agent 编排
│   ├── mcp/                # MCP 服务
│   ├── sandbox/            # 沙箱执行
│   └── bootstrap/          # 启动期自举（超级管理员）
├── frontend/               # Vue 3 前端
│   └── src/
│       ├── views/          # 页面（announcements/notes/knowledge/chat/...）
│       ├── components/     # 复用组件
│       ├── stores/         # Pinia 状态
│       ├── router/         # 路由
│       ├── i18n/           # 国际化（zh / en / ja / ko）
│       └── api/            # API 客户端
├── docreader/              # 文档解析 gRPC 服务
├── mcp-server/             # MCP 服务实现
├── migrations/             # 数据库迁移（按编号顺序）
├── cli/                    # 独立命令行工具（登录 / 注销 / 状态查询）
├── client/                 # Go SDK 客户端
├── config/                 # 内置模型 / 检索引擎配置
├── docs/                   # 设计文档与运维手册
├── scripts/                # 构建 / 部署 / 运维脚本
└── docker-compose.yml      # 全栈编排
```

### 常用命令

```bash
# 构建
make build                 # 构建后端二进制
make docker-build-all      # 构建所有镜像

# 测试
make test                  # Go 单元测试
make docker-go-test        # 在容器内跑 Go 测试

# 数据库迁移
make migrate-up            # 升级到最新版本
make migrate-down          # 回滚一次
make migrate-version       # 查看当前版本
make migrate-create NAME=xxx  # 新建迁移

# 镜像管理
make docker-run            # docker compose up -d
make docker-stop           # docker compose stop
make clean-db              # 删除 postgres-data 卷（⚠️ 数据丢失）
make list-containers       # 列出所有 WeKnora 容器

# 文档
make docs                  # 启动 Swagger UI
```

### 关键代码入口

- **平台身份与 RBAC**：`internal/types/user.go`（`PlatformIdentity` 枚举）、`internal/middleware/rbac.go`。
- **超级管理员自举**：`internal/bootstrap/superadmin.go`。
- **公告 API**：`internal/handler/announcement.go`（对应迁移 000093）。
- **学生笔记 API**：`internal/handler/me_note.go`（对应迁移 000092）。
- **教师任命**：`internal/handler/system_teacher.go`。
- **邀请注册**：`internal/handler/auth_register_by_invite.go`。
- **前端三大模块**：
  - 公告：`frontend/src/views/announcements/Announcements.vue`
  - 笔记：`frontend/src/views/notes/MyNotes.vue`
  - 知识库：`frontend/src/views/knowledge/KnowledgeBase.vue`

### 数据库迁移

所有迁移文件位于 `migrations/`，按编号顺序执行（如 `000001_init.sql`、`000043_rbac.sql`）。每次启动时若 `AUTO_MIGRATE=true`（默认）会自动跑所有未执行的迁移。

新增迁移：

```bash
make migrate-create NAME=add_some_feature
# 编辑生成的文件，按 Up / Down 两段写 SQL
```

## 部署

### 默认端口

| 服务 | 容器端口 | 宿主机端口（默认） |
|------|----------|--------------------|
| 前端（Nginx） | 80 | `FRONTEND_PORT=8089` |
| App（API） | 8080 | `APP_PORT=8081` |
| docreader（gRPC） | 50051 | 仅容器网络内 |
| Postgres | 5432 | 无 host 映射 |
| Redis | 6379 | 无 host 映射 |

可选 profile：MinIO（9000/9001）、Qdrant（6333/6334）、Milvus（19530）、Weaviate（9035）、Neo4j（7474/7687）、Doris（9030/8030/8040）、Langfuse（3000）、SearXNG（8888）。

### 关键环境变量

按 `.env.example` 分块说明（**所有真实凭据写在 `.env` 文件中，不要提交到 Git**）：

- **模型（LLM / Embedding / Rerank）**：OpenAI 兼容协议的 `*_BASE_URL`、`*_API_KEY`、`*_MODEL`。
- **安全密钥**：`JWT_SECRET`、`SYSTEM_AES_KEY`（32 字节 AES-256 主密钥，加密数据库中的 API Key）。
- **启动**：`AUTO_MIGRATE=true`、`WEKNORA_BOOTSTRAP_SUPERADMIN_EMAIL`、`WEKNORA_BOOTSTRAP_SUPERADMIN_PASSWORD`。
- **注册**：`DISABLE_REGISTRATION=true`、`WEKNORA_AUTH_REGISTRATION_MODE=invite_only`、`WEKNORA_TENANT_SELF_SERVICE_CREATION_ENABLED=false`。
- **向量库**：`RETRIEVE_DRIVER=postgres`（默认，ParadeDB 同时做事务库与向量库）。
- **存储**：`STORAGE_TYPE=local`、`LOCAL_STORAGE_BASE_DIR=/data/files`。

### 数据卷

| 卷名 | 用途 |
|------|------|
| `weknora_postgres-data` | Postgres 数据（含向量索引） |
| `weknora_data-files` | 用户上传文件、解析产物、附件（app 容器内 `/data/files`） |
| `weknora_docreader-tmp` | docreader 临时目录（PDF 渲染、嵌入图导出） |

按需启用：`weknora_minio_data`、`weknora_qdrant_data`、`weknora_milvus_data`、`weknora_weaviate_data` 等。

### 备份与恢复

```bash
# 备份（导出 pg_dump + 文件 tar + 环境变量快照）
./scripts/deploy-backup.sh

# 产物
backup/weknora-db.sql         # SQL 全文（跨 PG 版本最稳）
backup/weknora-files.tar.gz   # 文件卷
backup/weknora-env.env        # .env 关键变量（敏感值脱敏）
```

恢复时按相反顺序导入；保持 `SYSTEM_AES_KEY` / `TENANT_AES_KEY` / `JWT_SECRET` 与备份时一致，否则已加密的模型凭据和文档解不开。

### 部署常见坑

⚠️ **`docker compose build app/frontend | tail -N` 会吞掉失败退出码**（管道使 `exit code` 恒为 0），多次导致"以为构建成功实则旧镜像"。务必：

- **改前端后必须先 `npm run build`**（产出 `dist`），再 `docker compose build frontend`。`docker compose build frontend` 不会重新编 `dist`，只把 `dist` 烤进镜像。
- **查镜像 `CreatedAt`** 而非只看 tail 输出确认是否真的更新。
- 使用 `./scripts/rebuild_frontend.sh` 合并 build + recreate，但要先确认 `npm run build` 真的执行过。

## 文档

- [docs/超级管理员上手手册.md](./docs/超级管理员上手手册.md) — 终端用户友好的运维入门
- [CONTEXT.md](./CONTEXT.md) — 领域语言与角色定义
- [docs/RBAC说明.md](./docs/RBAC说明.md) — 空间角色与平台身份矩阵
- [docs/BUILTIN_MODELS.md](./docs/BUILTIN_MODELS.md) — 内置模型配置
- [docs/api/](./docs/api/) — 模块 API 文档
- [docs/swagger.yaml](./docs/swagger.yaml) — 完整 REST API 定义
- [docs/migration-troubleshooting.md](./docs/migration-troubleshooting.md) — 数据库迁移故障排查
- [CHANGELOG.md](./CHANGELOG.md) — 版本变更记录

## 致谢

本项目基于以下开源项目构建：

- [Tencent/WeKnora](https://github.com/Tencent/WeKnora) — 上游 RAG 引擎与文档解析框架
- [ParadeDB](https://www.paradedb.com/) — Postgres 上的向量检索与全文检索
- [MinerU](https://github.com/opendatalab/MinerU) — 高质量 PDF / Office 文档解析
- [TDesign](https://github.com/Tencent/tdesign-vue-next) — 腾讯前端组件库
- [Gin](https://github.com/gin-gonic/gin) — Go HTTP 框架

## 许可证

本项目采用 [MIT 协议](./LICENSE)，与上游 Tencent/WeKnora 保持一致。

如需在其他学校复用本 fork，请保留上游版权声明，并在显著位置标注本项目基于 WeKnora 川农教学版。
