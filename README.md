<p align="center">
  <picture>
    <img src="./docs/images/logo.png" alt="WeKnora · SICAU Teaching Edition" height="120"/>
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
    <img alt="Quick Start" src="https://img.shields.io/badge/Quick_Start-Handbook-4e6b99">
  </a>
</p>

<h1 align="center">WeKnora · SICAU Teaching Edition</h1>

<p align="center">
A teaching-scoped fork built for <strong>Sichuan Agricultural University</strong>. Based on <a href="https://github.com/Tencent/WeKnora">Tencent/WeKnora</a>, it provides an integrated platform that combines RAG (Retrieval-Augmented Generation), course announcements, and student notes for instructors and learners.
</p>

<p align="center">
  <a href="./README_CN.md"><strong>简体中文</strong></a> · <strong>English</strong>
</p>

---

## What is this

**WeKnora · SICAU Teaching Edition** is a self-hosted knowledge-base system designed for university instructors and students. The core idea: instructors upload lecture slides, references, and handouts into the system; students ask questions in natural language and get grounded answers. The platform also handles course-wide announcements and lets students keep private study notes — all in one place.

The system is organized around three roles:

- **Super Admin** — the sole person responsible for platform governance and teacher appointments. Super admins automatically inherit teacher capabilities.
- **Teacher** — appointed by the Super Admin. A teacher creates and manages their own workspaces (typically one per course), uploads materials, configures models, and invites students.
- **Student** — registers through an invite link issued by a teacher. Joins authorized workspaces with read-only access by default. Students cannot upload materials or manage members.

For the precise definitions of every role and concept, see [CONTEXT.md](./CONTEXT.md). For end-user-facing operations, see [docs/超级管理员上手手册.md](./docs/超级管理员上手手册.md) (Simplified Chinese).

## Features

### Teaching toolkit

- **Course announcements** — teachers publish Markdown announcements with optional attachments (≤ 50 MB per item, ≤ 5 files). All workspace members can read and reply. Authors and workspace admins can delete (cascade-removes attachments and replies).
- **Private student notes** — every student writes Markdown notes in their own private namespace. Supports local drafts, auto-save, and image embedding (images are served through a dedicated endpoint that only the owner can read). Teachers and classmates cannot see them — this is the only writable data surface available to students.
- **Invite-only registration** — public sign-up is disabled. Teachers generate time-limited, multi-use invite links inside a course workspace. Students click the link, register, and join the course with the default `viewer` role and minimal permissions.

### Two-layer permission model

The system uses two orthogonal permission dimensions:

- **Platform identity** — Super Admin / Teacher / Student. Determines *who can teach and who can be appointed*.
- **Workspace role** — `viewer < contributor < admin < owner`. Determines *what you can do inside a specific workspace*.

The Super Admin is a "compound identity" — they hold both platform governance and teacher capabilities, but they do not automatically become an admin of other teachers' workspaces. See [docs/RBAC说明.md](./docs/RBAC说明.md) for details.

### Core RAG capabilities (inherited from upstream)

- **Knowledge bases** — CRUD, folder organization, document upload and parsing (PDF / Word / PPT / Excel / Markdown, etc.).
- **Document parsing** — built-in `docreader` service (gRPC). Supports MinerU (with formula / table / OCR), PaddleOCR-VL, and local parsers.
- **Chat and Agents** — streaming Q&A, conversation history, citation sources, Agent editor, MCP tool integration, Skill plugins.
- **Long-term memory** — optional per workspace; auto-extracts user interests and facts from conversations.
- **Retrieval augmentation** — vector search (Postgres / Qdrant / Milvus / Weaviate, configurable), reranking, optional graph RAG.

### Model configuration

Per-workspace isolation. Each teacher workspace independently binds:

- **LLM** — OpenAI-compatible protocol (OpenAI / DeepSeek / Zhipu / Ollama, etc.).
- **Embedding** — same; any OpenAI-compatible provider.
- **Rerank** — optional.
- **VLLM** — optional vision-language model.

Model credentials use `${ENV_VAR}` placeholders, are resolved at runtime, and are encrypted at rest with `SYSTEM_AES_KEY` (AES-256). See [docs/BUILTIN_MODELS.md](./docs/BUILTIN_MODELS.md) and `config/builtin_models.yaml`.

### One-command deployment

A single `docker compose up -d` boots the entire stack. Postgres (with vector capabilities) and Redis are bundled — no external database required.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  Frontend  (Vue 3 + TDesign + Vite)                         │
│  Nginx reverse proxy → /api/* forwarded to App              │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│  App  (Go 1.22 + Gin)                                       │
│  ├─ handler / service / types / repository                  │
│  ├─ Agent / MCP / Skill / Memory / Sandbox                  │
│  └─ streaming responses + JWT auth + dual RBAC middleware   │
└───────┬─────────────────────────────────┬───────────────────┘
        │                                 │
        ▼                                 ▼
┌──────────────────┐           ┌────────────────────────────┐
│  Postgres        │           │  Docreader (gRPC)          │
│  ├─ transactions │ ◄────────►│  MinerU / PaddleOCR-VL     │
│  ├─ vector index │           │  PDF / Office parsing      │
│  └─ ParadeDB     │           └────────────────────────────┘
└──────────────────┘
        ▲
        │
┌──────────────────┐
│  Redis           │
│  ├─ stream chans │
│  └─ cache        │
└──────────────────┘
```

**Module responsibilities**:

- **frontend** — Vue 3 SPA, TDesign component library, Vite build.
- **app** — Go backend. Single repository, three-layer architecture: handler / service / repository.
- **docreader** — standalone gRPC service. Handles document parsing and figure extraction. Scales horizontally.
- **postgres** — transactional database. ParadeDB also provides vector search and full-text search.
- **redis** — streaming channels (Server-Sent Events) + cache.

## Getting started

### Requirements

- Docker 24+
- 8 GB available RAM
- 20 GB available disk
- Access to an OpenAI-compatible LLM / Embedding service (local Ollama works too)

### Install

```bash
# 1. Clone the repository
git clone <your-fork-url> weknora-sicau && cd weknora-sicau

# 2. Copy the environment template
cp .env.example .env

# 3. Edit .env as needed (at minimum, fill in LLM and Embedding credentials)

# 4. Boot the full stack
docker compose up -d
```

After startup, visit [http://localhost:8089](http://localhost:8089).

### First-time login

The system provisions a single Super Admin account out of the box:

| Field | Default value |
|-------|---------------|
| Login method | **Email** (not username) |
| Email | `admin@admin.com` |
| Initial password | `admin123` |

First login forces a password change. New passwords must be 8–32 characters and contain at least one letter and one digit.

> The first thing you should do after logging in: open **System Settings → Teacher Management** and appoint a few teachers. Everything else (creating workspaces, configuring models, inviting students) is the teacher's job.

## Core concepts

### Platform identity vs. workspace role

| Dimension | Values | Who can change it |
|-----------|--------|-------------------|
| Platform identity | Super Admin / Teacher / Student | Only a Super Admin can appoint teachers; Super Admins are provisioned at deploy time |
| Workspace role | owner / admin / contributor / viewer | The workspace owner adjusts roles on the Members page |

A single user can hold both the "Super Admin" platform identity and a `contributor` role in some workspace — the two are independent.

### Workspace

A workspace is the smallest unit of teaching. Typically one course per workspace. Inside a workspace:

- Knowledge bases and documents
- Model bindings (LLM / Embedding / Rerank)
- Agent templates
- Member list (students join through invite links)
- Announcements and attachments

### Registration mode

`WEKNORA_AUTH_REGISTRATION_MODE=invite_only` (default). Public self-registration is disabled — strangers cannot sign up and become "owners" by themselves.

### Model isolation

Model credentials and configuration are stored per `tenant_id` (workspace). A DeepSeek API key configured by Teacher A is scoped to A's workspace and never leaks to Teacher B's workspace.

## Developer guide

### Local development

```bash
# Start dependency services (postgres / redis / docreader);
# app and frontend run with hot reload on the host
make dev-start

# Tail logs
make dev-logs

# Stop
make dev-stop
```

For finer-grained control, the helper scripts support more options:

```bash
# Start Ollama + all dependencies + app
./scripts/start_all.sh --ollama

# Environment check only
./scripts/check-env.sh
```

### Directory layout

```
weknora-sicau/
├── cmd/                    # Go entrypoints
├── internal/               # Backend core
│   ├── handler/            # HTTP / gRPC handlers
│   ├── service/            # Business logic
│   ├── repository/         # Data access
│   ├── types/              # Domain types
│   ├── middleware/         # Gin middlewares
│   ├── agent/              # Agent orchestration
│   ├── mcp/                # MCP service
│   ├── sandbox/            # Sandbox execution
│   └── bootstrap/          # Startup-time bootstrapping (Super Admin)
├── frontend/               # Vue 3 frontend
│   └── src/
│       ├── views/          # Pages (announcements / notes / knowledge / chat / ...)
│       ├── components/     # Reusable components
│       ├── stores/         # Pinia state
│       ├── router/         # Routes
│       ├── i18n/           # Internationalization (zh / en / ja / ko)
│       └── api/            # API client
├── docreader/              # gRPC document-parsing service
├── mcp-server/             # MCP service implementations
├── migrations/             # Database migrations (numbered, applied in order)
├── cli/                    # Standalone CLI tool (login / logout / status)
├── client/                 # Go SDK client
├── config/                 # Built-in models / retrieval engine config
├── docs/                   # Design docs and operations manuals
├── scripts/                # Build / deploy / ops scripts
└── docker-compose.yml      # Full-stack orchestration
```

### Common commands

```bash
# Build
make build                 # Build backend binary
make docker-build-all      # Build all Docker images

# Test
make test                  # Go unit tests
make docker-go-test        # Run Go tests inside a container

# Database migrations
make migrate-up            # Upgrade to latest version
make migrate-down          # Roll back one step
make migrate-version       # Show current version
make migrate-create NAME=xxx  # Create a new migration

# Container management
make docker-run            # docker compose up -d
make docker-stop           # docker compose stop
make clean-db              # Remove postgres-data volume (⚠️ destroys data)
make list-containers       # List all WeKnora containers

# Documentation
make docs                  # Launch Swagger UI
```

### Key code entry points

- **Platform identity & RBAC** — `internal/types/user.go` (`PlatformIdentity` enum), `internal/middleware/rbac.go`.
- **Super Admin bootstrap** — `internal/bootstrap/superadmin.go`.
- **Announcement API** — `internal/handler/announcement.go` (migration 000093).
- **Student notes API** — `internal/handler/me_note.go` (migration 000092).
- **Teacher appointment** — `internal/handler/system_teacher.go`.
- **Invite-based registration** — `internal/handler/auth_register_by_invite.go`.
- **Frontend teaching modules**:
  - Announcements: `frontend/src/views/announcements/Announcements.vue`
  - Notes: `frontend/src/views/notes/MyNotes.vue`
  - Knowledge base: `frontend/src/views/knowledge/KnowledgeBase.vue`

### Database migrations

All migrations live in `migrations/`, applied in numeric order (e.g. `000001_init.sql`, `000043_rbac.sql`). On every startup, if `AUTO_MIGRATE=true` (default), the system runs all pending migrations automatically.

To create a new migration:

```bash
make migrate-create NAME=add_some_feature
# Edit the generated file; provide both Up and Down sections.
```

## Deployment

### Default ports

| Service | Container port | Host port (default) |
|---------|----------------|--------------------|
| Frontend (Nginx) | 80 | `FRONTEND_PORT=8089` |
| App (API) | 8080 | `APP_PORT=8081` |
| docreader (gRPC) | 50051 | Container network only |
| Postgres | 5432 | No host mapping |
| Redis | 6379 | No host mapping |

Optional profiles: MinIO (9000/9001), Qdrant (6333/6334), Milvus (19530), Weaviate (9035), Neo4j (7474/7687), Doris (9030/8030/8040), Langfuse (3000), SearXNG (8888).

### Key environment variables

Grouped by category — see `.env.example` for the full list. **All real credentials belong in `.env`, which must not be committed to Git.**

- **Models (LLM / Embedding / Rerank)** — `*_BASE_URL`, `*_API_KEY`, `*_MODEL` for OpenAI-compatible providers.
- **Security keys** — `JWT_SECRET`, `SYSTEM_AES_KEY` (32-byte AES-256 master key that encrypts API keys stored in the database).
- **Startup** — `AUTO_MIGRATE=true`, `WEKNORA_BOOTSTRAP_SUPERADMIN_EMAIL`, `WEKNORA_BOOTSTRAP_SUPERADMIN_PASSWORD`.
- **Registration** — `DISABLE_REGISTRATION=true`, `WEKNORA_AUTH_REGISTRATION_MODE=invite_only`, `WEKNORA_TENANT_SELF_SERVICE_CREATION_ENABLED=false`.
- **Vector store** — `RETRIEVE_DRIVER=postgres` (default; ParadeDB serves as both transactional and vector database).
- **Storage** — `STORAGE_TYPE=local`, `LOCAL_STORAGE_BASE_DIR=/data/files`.

### Data volumes

| Volume | Purpose |
|--------|---------|
| `weknora_postgres-data` | Postgres data, including vector indexes |
| `weknora_data-files` | User uploads, parsed artifacts, attachments (`/data/files` inside the app container) |
| `weknora_docreader-tmp` | docreader temp directory (PDF rendering, embedded figure export) |

Optional volumes enabled by profile: `weknora_minio_data`, `weknora_qdrant_data`, `weknora_milvus_data`, `weknora_weaviate_data`, etc.

### Backup and restore

```bash
# Backup (pg_dump + file tarball + environment snapshot)
./scripts/deploy-backup.sh

# Artifacts
backup/weknora-db.sql         # Full SQL dump (most portable across PG versions)
backup/weknora-files.tar.gz   # File volume
backup/weknora-env.env        # .env key variables (sensitive values redacted)
```

To restore, import in reverse order. Keep `SYSTEM_AES_KEY`, `TENANT_AES_KEY`, and `JWT_SECRET` identical to the backup, otherwise encrypted model credentials and documents cannot be decrypted.

### Deployment gotchas

⚠️ **`docker compose build app/frontend | tail -N` swallows the failure exit code** (the pipe makes `exit code` always 0). This has caused "looks like a successful build but the image is stale" more than once. Always:

- **After editing the frontend, run `npm run build` first** (produces `dist`). Then `docker compose build frontend` only bakes the existing `dist` into the image — it does not rebuild `dist`.
- **Check the image `CreatedAt`** instead of trusting `tail` output.
- `scripts/rebuild_frontend.sh` combines build + recreate, but only works correctly if `npm run build` actually ran.

## Documentation

- [docs/超级管理员上手手册.md](./docs/超级管理员上手手册.md) — End-user-friendly operations handbook (Simplified Chinese)
- [CONTEXT.md](./CONTEXT.md) — Domain language and role definitions
- [docs/RBAC说明.md](./docs/RBAC说明.md) — Workspace roles and platform identity matrix
- [docs/BUILTIN_MODELS.md](./docs/BUILTIN_MODELS.md) — Built-in model configuration
- [docs/api/](./docs/api/) — Per-module API documentation
- [docs/swagger.yaml](./docs/swagger.yaml) — Full REST API definition
- [docs/migration-troubleshooting.md](./docs/migration-troubleshooting.md) — Database migration troubleshooting
- [CHANGELOG.md](./CHANGELOG.md) — Release notes

## Credits

This project is built on top of:

- [Tencent/WeKnora](https://github.com/Tencent/WeKnora) — Upstream RAG engine and document-parsing framework
- [ParadeDB](https://www.paradedb.com/) — Vector search and full-text search on Postgres
- [MinerU](https://github.com/opendatalab/MinerU) — High-quality PDF / Office document parsing
- [TDesign](https://github.com/Tencent/tdesign-vue-next) — Tencent's Vue 3 component library
- [Gin](https://github.com/gin-gonic/gin) — Go HTTP framework

## License

This project is released under the [MIT License](./LICENSE), same as upstream Tencent/WeKnora.

If you fork this project for another school, please retain the upstream copyright notice and clearly mark your fork as "based on WeKnora · SICAU Teaching Edition".
