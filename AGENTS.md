# AGENTS.md — Classifier

Reference for coding agents working in this repository.
Project status: **Phase 1 MVP implemented + v3 job/workflow foundation partially landed**.

---

## Project Overview

Classifier is a NAS-deployed media folder organizer.
Single container deployment target: Go backend + React SPA + FFmpeg runtime.

Current implemented scope includes:
- folder scanning
- media classification
- folder list / category / status management
- folder move flow
- snapshot revert flow
- audit logging
- SSE progress channel
- persisted `jobs` foundation for move tasks
- job polling endpoints (`GET /api/jobs`, `GET /api/jobs/:id`, `GET /api/jobs/:id/progress`)
- folder soft delete + restore
- local and Docker deployment assets

---

## Current Stack

- **Backend**: Go 1.26, Gin, SQLite via `modernc.org/sqlite`, SSE
- **Frontend**: React 19, TypeScript 5.9, Vite 8, Zustand 5, Tailwind CSS 3, React Router v6
- **Infra**: Docker multi-stage build, docker-compose, Alpine runtime

---

## Actual Repository Layout

```text
Classifier/
├── backend/
│   ├── cmd/server/              # main.go entrypoint + embedded frontend assets
│   ├── internal/
│   │   ├── config/              # env config loading
│   │   ├── db/                  # sqlite open + embedded migrations
│   │   ├── fs/                  # filesystem adapter layer
│   │   ├── handler/             # Gin HTTP handlers
│   │   ├── repository/          # SQLite repositories + models
│   │   ├── service/             # classifier / scanner / move / snapshot / audit
│   │   └── sse/                 # SSE broker
│   ├── migrations/              # canonical SQL migration copies
│   ├── go.mod
│   └── go.sum
├── frontend/
│   ├── src/
│   │   ├── api/
│   │   ├── components/
│   │   ├── hooks/
│   │   ├── lib/
│   │   ├── pages/
│   │   ├── store/
│   │   └── types/
│   ├── package.json
│   └── vite.config.ts
├── docs/
├── Dockerfile
├── docker-compose.yml
├── .env.example
├── .env.local.example
├── README.md
└── AGENTS.md
```

Do not assume the old planned `scheduler/` package exists. Current code still uses direct goroutines for scan/move entrypoints. The real WorkflowRunner / NodeRunner architecture is designed in the v3 docs but not fully implemented yet.

---

## Implemented Backend Endpoints

```text
GET  /health
GET  /api/events
GET  /api/folders
POST /api/folders/scan
GET  /api/folders/:id
POST /api/folders/:id/restore
PATCH /api/folders/:id/category
PATCH /api/folders/:id/status
DELETE /api/folders/:id
GET  /api/jobs
GET  /api/jobs/:id
GET  /api/jobs/:id/progress
POST /api/jobs/move
GET  /api/snapshots?folder_id=...
GET  /api/snapshots?job_id=...
POST /api/snapshots/:id/revert
GET  /api/config
PUT  /api/config
```

Frontend pages currently implemented:
- `/` → folder list page
- `/settings` → config form

---

## Build, Test, Run

### Backend

```bash
cd backend
CGO_ENABLED=0 go build ./...
CGO_ENABLED=0 go build ./cmd/server
CGO_ENABLED=0 go test ./...
go vet ./...
```

**Run a single test or test function:**

```bash
# All tests in one package
CGO_ENABLED=0 go test ./internal/handler/...

# Single function by name
CGO_ENABLED=0 go test ./internal/handler/... -run TestFolderHandler

# Single sub-test (table-driven row)
CGO_ENABLED=0 go test -v ./internal/handler/... -run TestFolderHandler/list_returns_folders

# Service layer
CGO_ENABLED=0 go test ./internal/service/... -run TestScannerService
```

### Frontend

```bash
cd frontend
npm install
npm run typecheck
npm run lint
npm run build
npm run dev
```

### Docker

```bash
docker compose --env-file .env.example build
docker compose --env-file .env.example up -d
```

### Local Development

Prepare local directories:

```bash
mkdir -p .local/source .local/target .local/config .local/delete-staging
```

Start backend:

```bash
cd backend
CONFIG_DIR="$(pwd)/../.local/config" \
SOURCE_DIR="$(pwd)/../.local/source" \
TARGET_DIR="$(pwd)/../.local/target" \
DELETE_STAGING_DIR="$(pwd)/../.local/delete-staging" \
PORT=8080 \
CGO_ENABLED=0 \
 go run ./cmd/server
```

Start frontend in another terminal:

```bash
cd frontend
npm run dev
```

---

## Go Rules

- Always build and test with `CGO_ENABLED=0`
- Never use `mattn/go-sqlite3`; use `modernc.org/sqlite` only
- All filesystem access must go through `internal/fs`
- Services and handlers must not call `os.*` directly
- Keep `context.Context` as the first argument for blocking or IO work
- Wrap errors with context using `fmt.Errorf("name: %w", err)`
- Use table-driven tests with `t.Run(name, func(t *testing.T) {...})`
- Prefer focused services and repository interfaces over cross-layer shortcuts
- Bugfix rule: fix minimally — never refactor while fixing

### Naming & Types

- DB struct tags use `db:"snake_case"`
- Handler structs accept interface dependencies (e.g. `FolderScanService`), not concrete types
- IDs are `string` (UUID) throughout — use `github.com/google/uuid`
- Pointer receiver for all structs with state or interface implementations
- Nullable DB columns → pointer types (`*time.Time`, etc.)
- JSON blobs in DB → `json.RawMessage`
- Valid enum values validated against `map[string]struct{}`, not switch statements

### Imports

Three blocks separated by blank lines: stdlib → third-party → internal.

```go
import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/liqiye/classifier/internal/repository"
	"github.com/liqiye/classifier/internal/service"
)
```

### Testing Patterns

- Interface fakes (not mocks) — minimal in-memory struct implementing the interface
- In-memory SQLite per test: `file:classifier_<pkg>_<n>?cache=shared&mode=memory`
- Unique DB names via `atomic.AddUint64` counter for parallel test safety
- `t.Helper()` on all helper functions; `t.Cleanup()` for resource teardown
- `gin.SetMode(gin.TestMode)` at top of handler test files
- `httptest.NewRecorder()` for HTTP handler tests — no live server needed
- Seed helpers (e.g. `seedFolder`) call `t.Fatalf` on setup failure

### Existing service boundaries

- `service.Classify(folderName, fileNames)` performs category detection
- `service.ScannerService` scans immediate child directories under `SOURCE_DIR`
- `service.MoveService` now handles move + snapshot + audit + SSE + persisted job progress for move tasks
- `service.SnapshotService` handles create-before / commit-after / revert
- `service.AuditService` is a thin wrapper over the audit repository

### New repository boundaries already present

- `JobRepository` is implemented and used by move/job handlers
- `FolderRepository` now includes soft delete / restore behavior

### Planned but not yet implemented

- `WorkflowRunRepository`
- `NodeRunRepository`
- `NodeSnapshotRepository`
- `WorkflowDefinitionRepository`
- `WorkflowRunner`
- `NodeExecutor` registry

Use repository interfaces instead of reaching into SQL from handlers or services that already have an abstraction available.

---

## Frontend Rules

### Hard Rules

- Strict TypeScript only — no `any`, no `@ts-ignore`, no `@ts-expect-error`
- Tailwind utility classes only — no inline styles, no CSS modules
- All user-facing copy is hardcoded **Chinese** unless explicitly told otherwise
- Use `cn()` from `src/lib/utils.ts` for conditional/merged class strings
- ESLint must pass with zero warnings: `npm run lint`
- `ApiRequestError` (from `src/api/client.ts`) extends `Error` — never throw plain strings

### Imports

- Use `@/` alias for all internal imports (maps to `src/`)
- Use `import type { ... }` for type-only imports
- Group order: external libs → `@/api/` → `@/components/` → `@/hooks/` → `@/lib/` → `@/store/` → `@/types`

```ts
import { useState, useEffect } from 'react'
import { AlertTriangle } from 'lucide-react'

import { revertSnapshot } from '@/api/snapshots'
import { ApiRequestError } from '@/api/client'
import { cn } from '@/lib/utils'
import { useSnapshotStore } from '@/store/snapshotStore'
import type { Snapshot } from '@/types'
```

### Architecture

| Layer | Location | Notes |
|---|---|---|
| HTTP client + error | `src/api/client.ts` | `request<T>()` helper; 204 returns `undefined as T` |
| Domain API functions | `src/api/<domain>.ts` | Pure async functions returning typed data |
| Global state | `src/store/<name>Store.ts` | Zustand; owns fetching and mutations |
| SSE | `src/hooks/useSSE.ts` | Single hook for all SSE events |
| Page components | `src/pages/` | Thin — delegate data work to stores/API |
| Shared UI | `src/components/` | Avoid direct store access unless necessary |
| Types | `src/types/index.ts` | All shared types; no `any` |

### Component Conventions

- Export props interface: `export interface MyComponentProps { ... }`
- Constant label/class maps declared outside the component body
- Format dates: `new Date(value).toLocaleString('zh-CN')`
- Icons: `lucide-react` only
- Available UI primitives: `@radix-ui/react-slot`, `class-variance-authority`, `clsx`, `tailwind-merge`

### Existing Key Files

- `src/pages/FolderListPage.tsx` — uses `useFolderStore`
- `src/pages/SettingsPage.tsx` — reads/writes `/api/config`
- `src/components/SnapshotDrawer.tsx` — snapshot state via `useSnapshotStore`
- `src/store/folderStore.ts` — folder list + scan progress
- `src/store/snapshotStore.ts` — snapshot list state
- `src/store/jobStore.ts` — job polling state

---

## Architecture Constraints

- SQLite only
- SSE instead of WebSocket for push events
- Snapshot record must exist before mutating moves
- Job progress must be queryable over HTTP, not SSE-only
- Folder deletion is soft delete by default; do not reintroduce hard delete semantics in handlers
- Backend binary serves embedded frontend assets from `backend/cmd/server/web/dist`
- Dockerfile must copy built frontend assets into that embed path before building the Go server
- For NAS deployment, keep the compose default compatible with 极空间 bind mounts

---

## 极空间 / NAS Notes

- Default Docker compose runtime uses `user: "0:0"` for compatibility with 极空间 shared-folder permissions
- Do not enable `privileged: true` unless explicitly required
- Real shared-folder paths on 极空间 commonly look like:

```text
/tmp/zfsv3/.../data/...
```

- Local developer guidance is in `README.md`
- NAS deployment guidance is in `docs/部署/极空间部署指南.md`

---

## Documentation Index

- `docs/文档目录.md` — categorized documentation index
- `docs/功能/接口设计.md` — legacy/high-level API notes
- `docs/功能/接口设计（版本3）.md` — latest API direction
- `docs/架构/架构概览.md` — legacy/high-level system architecture
- `docs/架构/架构概览（版本3）.md` — latest architecture direction
- `docs/功能/审计日志.md` — audit model
- `docs/功能/分类规则.md` — legacy classification rules
- `docs/功能/分类规则（版本3）.md` — classifier-node direction
- `docs/架构/数据模型.md` — legacy SQLite/data model reference
- `docs/架构/数据模型（版本3）.md` — latest data model direction
- `docs/部署/Docker部署指南.md` — Docker deployment reference
- `docs/功能/前端设计.md` — legacy frontend design
- `docs/功能/前端设计（版本3）.md` — latest frontend direction
- `docs/架构/配置系统.md` — v3 config system design
- `docs/规划/技术研究.md` — technical research notes
- `docs/规划/开发路线图.md` — actual implementation roadmap/status
- `docs/规划/开发路线图（版本3）.md` — v3 execution plan
- `docs/功能/快照系统.md` — legacy snapshot / revert design
- `docs/功能/快照系统（版本3）.md` — node-level snapshot direction
- `docs/功能/工作流设计.md` — legacy workflow planning
- `docs/功能/工作流设计（版本3）.md` — latest workflow design
- `docs/功能/重命名编辑器（版本3）.md` — v3 rename design
- `docs/部署/极空间部署指南.md` — 极空间 deployment runbook

---

## What Is Not Implemented Yet

Do not assume these are already available:
- workflow editor
- rename editor
- compression pipeline
- thumbnail generation pipeline
- scheduler/workflow runner package
- workflow_runs / node_runs / node_snapshots persistence
- structured `app_config` configuration system
- full audit log UI
- authentication/authorization

When extending the project:
- match the current implemented code first
- use the v3 docs as the intended direction
- do not claim WorkflowRunner/NodeRunner already exists unless you implement it in this task
