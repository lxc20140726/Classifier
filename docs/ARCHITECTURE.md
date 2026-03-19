# Classifier — 媒体文件整理工具 架构设计文档

> 版本：v1.0 | 日期：2026-03-19 | 适用：单人开发 · NAS Docker 部署

---

## 1. 技术栈最终确认

### 后端框架：Gin

选 **Gin** 而非 Echo/Fiber/stdlib：
- 生态最成熟，middleware 丰富（CORS、日志、recover 开箱即用）
- 路由性能足够，NAS 场景不是瓶颈
- 社区文档最多，单人开发友好
- Fiber 基于 fasthttp，与 net/http 生态不兼容，引入不必要风险

### 前端状态管理：Zustand

选 **Zustand** 而非 Redux/Jotai：
- API 极简，无 boilerplate，单人项目最合适
- 支持 slice 模式，可按模块拆分 store
- 比 Jotai 更适合管理复杂异步任务状态

### 数据持久化：SQLite（via modernc.org/sqlite）

- 纯 Go 实现，无 CGO，Docker 构建无额外依赖
- 比 JSON 文件更易查询/更新，比 PostgreSQL 轻量百倍
- 数据量极小（千级 Folder 记录），SQLite 完全够用
- 文件存于 /config/classifier.db，随 named volume 持久化

### 前端组件库：shadcn/ui + Tailwind CSS

- shadcn/ui 是"复制到项目"模式，无版本锁定风险
- Tailwind 构建产物小，适合 NAS 低配机器
- 组件质量高（Table、Dialog、Progress、Badge 都需要）

---

## 2. 系统架构图

```
┌─────────────────────────────────────────────────────────┐
│                    Docker Container                      │
│                                                          │
│  ┌──────────────┐    HTTP/SSE    ┌──────────────────┐   │
│  │  React SPA   │ ◄────────────► │  Gin HTTP Server  │   │
│  │  (静态文件)   │                │  :8080            │   │
│  └──────────────┘                └────────┬─────────┘   │
│                                           │              │
│              ┌────────────────────────────┤              │
│              │                            │              │
│    ┌─────────▼──────┐         ┌──────────▼──────────┐   │
│    │  Job Scheduler  │         │   Scanner Service    │   │
│    │  (有界并发队列)  │         │   (分类算法)         │   │
│    └─────────┬──────┘         └──────────┬──────────┘   │
│              │                            │              │
│    ┌─────────▼──────────────────────────▼──────────┐    │
│    │              Service Layer                      │    │
│    │  Rename │ Compress │ Thumbnail │ Move           │    │
│    └─────────────────────┬──────────────────────────┘    │
│                          │                               │
│              ┌───────────▼──────────┐                    │
│              │  SQLite + FS Layer   │                    │
│              │  /config/classifier  │                    │
│              └──────────────────────┘                    │
│                                                          │
│  Volume Mounts:                                          │
│  /data/source (ro) │ /data/target (rw) │ /tmp/work      │
└─────────────────────────────────────────────────────────┘
```

**分层说明：**

| 层次 | 职责 |
|------|------|
| Transport | Gin router、SSE hub、静态文件服务 |
| Handler | 请求解析、参数校验、响应序列化 |
| Service | 业务逻辑（分类、重命名、压缩、缩略图、移动） |
| Scheduler | Job 队列、并发控制（semaphore 有界 goroutine） |
| Repository | SQLite CRUD，屏蔽 SQL 细节 |
| FS Adapter | 所有文件系统操作集中此层，便于测试 mock |

---

## 3. 后端 API 设计

### 3.1 RESTful 端点

```
# Folder 管理
GET    /api/folders                    # 列表（?status=&category=&page=&limit=）
POST   /api/folders/scan               # 触发扫描 /data/source
GET    /api/folders/:id                # 单个详情
PATCH  /api/folders/:id/category       # 手动修正分类
PATCH  /api/folders/:id/status         # 标记状态（pending/done/skip）
DELETE /api/folders/:id                # 从 DB 移除记录（不删文件）

# 批量操作（提交 Job）
POST   /api/jobs/rename                # 批量重命名
POST   /api/jobs/compress              # 批量压缩
POST   /api/jobs/thumbnail             # 批量生成缩略图
POST   /api/jobs/move                  # 批量移动

# Job 管理
GET    /api/jobs                       # 任务列表（?status=running|done|failed）
GET    /api/jobs/:id                   # 任务详情
DELETE /api/jobs/:id                   # 取消/删除任务

# 重命名规则
GET    /api/rename-rules               # 规则列表
POST   /api/rename-rules               # 创建规则
PUT    /api/rename-rules/:id           # 更新规则
DELETE /api/rename-rules/:id           # 删除规则
POST   /api/rename-rules/:id/preview   # 预览重命名结果

# 配置
GET    /api/config                     # 读取配置
PUT    /api/config                     # 更新配置

# SSE
GET    /api/events                     # SSE 事件流
```

### 3.2 SSE 事件格式

```
event: job.progress
data: {"job_id":"uuid","folder_id":"uuid","type":"compress","progress":45,"message":"压缩中 23/51"}

event: job.done
data: {"job_id":"uuid","status":"done","elapsed_ms":3200}

event: job.failed
data: {"job_id":"uuid","error":"ffmpeg exit code 1"}

event: scan.progress
data: {"scanned":120,"total":300,"current_path":"/data/source/SomeFolder"}

event: scan.done
data: {"total_folders":300,"new_folders":45}
```

### 3.3 核心请求数据结构

```go
type RenameJobRequest struct {
    FolderIDs []string `json:"folder_ids"`
    RuleID    string   `json:"rule_id"`
}

type CompressJobRequest struct {
    FolderIDs []string `json:"folder_ids"`
    OutputDir string   `json:"output_dir"` // 留空则原地压缩
}

type ThumbnailJobRequest struct {
    FolderIDs  []string `json:"folder_ids"`
    SeekOffset string   `json:"seek_offset"` // 默认 "00:10:00"
}

type MoveJobRequest struct {
    FolderIDs []string `json:"folder_ids"`
    TargetDir string   `json:"target_dir"` // 留空则用全局 config.TargetDir
}

type UpdateCategoryRequest struct {
    Category string `json:"category"` // photo|video|mixed|manga
    Source   string `json:"source"`   // manual
}

// 通用响应
type Response[T any] struct {
    Data T `json:"data"`
}

type ErrorResponse struct {
    Error string `json:"error"`
    Code  string `json:"code,omitempty"`
}

type PagedResponse[T any] struct {
    Data  []T `json:"data"`
    Total int `json:"total"`
    Page  int `json:"page"`
    Limit int `json:"limit"`
}
```

---

## 4. 前端页面和组件设计

### 4.1 页面路由

```
/           → FolderListPage    主页，文件夹列表+批量操作
/jobs       → JobsPage          任务队列监控
/rules      → RulesPage         重命名规则管理
/settings   → SettingsPage      全局配置
```

### 4.2 核心组件树

```
App
├── Layout
│   ├── Sidebar（导航菜单）
│   ├── TopBar（扫描按钮、全局进度指示）
│   └── <Outlet>（页面内容区）
│
├── FolderListPage
│   ├── FolderToolbar
│   │   ├── ScanButton（触发扫描，显示 scan.progress）
│   │   ├── FilterBar（status / category / 搜索）
│   │   └── BulkActionMenu
│   │       ├── BulkRenameButton
│   │       ├── BulkCompressButton
│   │       ├── BulkThumbnailButton
│   │       └── BulkMoveButton
│   ├── FolderTable
│   │   ├── FolderRow
│   │   │   ├── Checkbox
│   │   │   ├── FolderName
│   │   │   ├── CategoryBadge（可点击修正）
│   │   │   ├── StatusBadge
│   │   │   ├── FileCountCell（图片数/视频数）
│   │   │   └── RowActions（单独操作下拉菜单）
│   │   └── Pagination
│   └── CategoryEditDialog（手动修正分类弹窗）
│
├── JobsPage
│   ├── JobFilterTabs（all / running / done / failed）
│   ├── JobList
│   │   └── JobCard
│   │       ├── JobTypeIcon
│   │       ├── JobSummary（X 个文件夹，类型）
│   │       ├── ProgressBar（running 时显示）
│   │       ├── StatusBadge
│   │       ├── ElapsedTime
│   │       └── CancelButton（running 时）
│   └── JobDetailDrawer（展开显示每个 folder 子任务状态）
│
├── RulesPage
│   ├── RuleList
│   │   └── RuleCard
│   │       ├── RuleName
│   │       ├── PatternPreview
│   │       └── RuleActions（编辑/删除/预览）
│   ├── RuleFormDialog（新建/编辑规则）
│   │   ├── NameInput
│   │   ├── TemplateInput（带变量提示）
│   │   └── VariableCheatSheet（{index} {date} {name} 说明）
│   └── RulePreviewDialog（预览重命名结果表格）
│
└── SettingsPage
    ├── PathSection（source/target 目录配置）
    ├── ConcurrencySection（最大并发数 slider）
    ├── ThumbnailSection（seek offset 默认值）
    └── DangerZone（清空 DB 记录等）
```

### 4.3 Zustand Store 设计

```typescript
// stores/folderStore.ts
interface FolderStore {
  folders: Folder[]
  total: number
  page: number
  limit: number
  filters: { status?: string; category?: string; search?: string }
  selectedIds: Set<string>
  isScanning: boolean
  scanProgress: ScanProgress | null

  fetchFolders: () => Promise<void>
  setFilters: (f: Partial<FolderStore['filters']>) => void
  setPage: (p: number) => void
  toggleSelect: (id: string) => void
  selectAll: () => void
  clearSelection: () => void
  updateCategory: (id: string, category: Category) => Promise<void>
  updateStatus: (id: string, status: FolderStatus) => Promise<void>
  setScanProgress: (p: ScanProgress | null) => void
}

// stores/jobStore.ts
interface JobStore {
  jobs: Job[]
  activeJobIds: Set<string>
  jobProgress: Record<string, JobProgress>

  fetchJobs: () => Promise<void>
  submitJob: (type: JobType, req: JobRequest) => Promise<Job>
  cancelJob: (id: string) => Promise<void>
  updateProgress: (jobId: string, p: JobProgress) => void
  markDone: (jobId: string) => void
}

// stores/ruleStore.ts
interface RuleStore {
  rules: RenameRule[]
  fetchRules: () => Promise<void>
  createRule: (r: CreateRuleRequest) => Promise<void>
  updateRule: (id: string, r: UpdateRuleRequest) => Promise<void>
  deleteRule: (id: string) => Promise<void>
  previewRule: (id: string, folderIds: string[]) => Promise<PreviewResult[]>
}

// hooks/useSSE.ts — 独立 hook
// 连接 /api/events，根据 event type 分发到对应 store action
```

---

## 5. 数据模型设计

### Go Struct 定义

```go
// models/folder.go
type Category string
const (
    CategoryPhoto   Category = "photo"
    CategoryVideo   Category = "video"
    CategoryMixed   Category = "mixed"
    CategoryManga   Category = "manga"
    CategoryUnknown Category = "unknown"
)

type FolderStatus string
const (
    StatusPending FolderStatus = "pending"
    StatusDone    FolderStatus = "done"
    StatusSkip    FolderStatus = "skip"
)

type Folder struct {
    ID               string       `json:"id" db:"id"`
    Path             string       `json:"path" db:"path"`
    Name             string       `json:"name" db:"name"`
    Category         Category     `json:"category" db:"category"`
    CategorySource   string       `json:"category_source" db:"category_source"` // auto|manual
    Status           FolderStatus `json:"status" db:"status"`
    ImageCount       int          `json:"image_count" db:"image_count"`
    VideoCount       int          `json:"video_count" db:"video_count"`
    TotalFiles       int          `json:"total_files" db:"total_files"`
    TotalSize        int64        `json:"total_size" db:"total_size"`
    MarkedForMove    bool         `json:"marked_for_move" db:"marked_for_move"`
    ScannedAt        time.Time    `json:"scanned_at" db:"scanned_at"`
    UpdatedAt        time.Time    `json:"updated_at" db:"updated_at"`
}

// models/job.go
type JobType string
const (
    JobTypeRename    JobType = "rename"
    JobTypeCompress  JobType = "compress"
    JobTypeThumbnail JobType = "thumbnail"
    JobTypeMove      JobType = "move"
)

type JobStatus string
const (
    JobStatusQueued    JobStatus = "queued"
    JobStatusRunning   JobStatus = "running"
    JobStatusDone      JobStatus = "done"
    JobStatusFailed    JobStatus = "failed"
    JobStatusCancelled JobStatus = "cancelled"
)

type Job struct {
    ID          string     `json:"id" db:"id"`
    Type        JobType    `json:"type" db:"type"`
    Status      JobStatus  `json:"status" db:"status"`
    FolderIDs   []string   `json:"folder_ids" db:"-"`   // 存 JSON 字符串
    FolderIDsRaw string    `json:"-" db:"folder_ids"`
    Params      string     `json:"params" db:"params"`  // JSON blob
    Progress    int        `json:"progress" db:"progress"`
    ErrorMsg    string     `json:"error,omitempty" db:"error_msg"`
    CreatedAt   time.Time  `json:"created_at" db:"created_at"`
    StartedAt   *time.Time `json:"started_at,omitempty" db:"started_at"`
    FinishedAt  *time.Time `json:"finished_at,omitempty" db:"finished_at"`
}

// models/rule.go
type RenameRule struct {
    ID        string    `json:"id" db:"id"`
    Name      string    `json:"name" db:"name"`
    Pattern   string    `json:"pattern" db:"pattern"` // e.g. "{name}_{index:03d}"
    IsPreset  bool      `json:"is_preset" db:"is_preset"`
    CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// models/config.go
type AppConfig struct {
    SourceDir       string `json:"source_dir" db:"source_dir"`
    TargetDir       string `json:"target_dir" db:"target_dir"`
    MaxConcurrency  int    `json:"max_concurrency" db:"max_concurrency"`
    ThumbnailSeek   string `json:"thumbnail_seek" db:"thumbnail_seek"` // "00:10:00"
    ZipCompression  int    `json:"zip_compression" db:"zip_compression"` // 0=store,1=fast
    AutoScanOnStart bool   `json:"auto_scan_on_start" db:"auto_scan_on_start"`
}
```

### SQLite Schema

```sql
CREATE TABLE IF NOT EXISTS folders (
    id               TEXT PRIMARY KEY,
    path             TEXT NOT NULL UNIQUE,
    name             TEXT NOT NULL,
    category         TEXT NOT NULL DEFAULT 'unknown',
    category_source  TEXT NOT NULL DEFAULT 'auto',
    status           TEXT NOT NULL DEFAULT 'pending',
    image_count      INTEGER NOT NULL DEFAULT 0,
    video_count      INTEGER NOT NULL DEFAULT 0,
    total_files      INTEGER NOT NULL DEFAULT 0,
    total_size       INTEGER NOT NULL DEFAULT 0,
    marked_for_move  INTEGER NOT NULL DEFAULT 0,
    scanned_at       DATETIME NOT NULL,
    updated_at       DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS jobs (
    id           TEXT PRIMARY KEY,
    type         TEXT NOT NULL,
    status       TEXT NOT NULL DEFAULT 'queued',
    folder_ids   TEXT NOT NULL DEFAULT '[]',
    params       TEXT NOT NULL DEFAULT '{}',
    progress     INTEGER NOT NULL DEFAULT 0,
    error_msg    TEXT NOT NULL DEFAULT '',
    created_at   DATETIME NOT NULL,
    started_at   DATETIME,
    finished_at  DATETIME
);

CREATE TABLE IF NOT EXISTS rename_rules (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    pattern    TEXT NOT NULL,
    is_preset  INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS config (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_folders_status   ON folders(status);
CREATE INDEX IF NOT EXISTS idx_folders_category ON folders(category);
CREATE INDEX IF NOT EXISTS idx_jobs_status      ON jobs(status);
```

---

## 6. 文件分类算法设计

### 6.1 扩展名映射表

```go
var imageExts = map[string]bool{
    ".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
    ".webp": true, ".bmp": true, ".tiff": true, ".tif": true,
    ".heic": true, ".heif": true, ".avif": true, ".raw": true,
}

var videoExts = map[string]bool{
    ".mp4": true, ".mkv": true, ".avi": true, ".mov": true,
    ".wmv": true, ".flv": true, ".m4v": true, ".ts": true,
    ".rmvb": true, ".rm": true, ".webm": true, ".3gp": true,
}

var mangaExts = map[string]bool{
    ".cbz": true, ".cbr": true, ".cb7": true, ".cbt": true,
}
```

### 6.2 分类决策树

```
扫描文件夹内所有文件
        │
        ▼
┌─────────────────────────────┐
│ 是否包含 .cbz/.cbr 文件？    │
│ 或文件夹名含"漫画/comic/manga"│
└──────────┬──────────────────┘
           │ YES → Category = manga
           │ NO
           ▼
┌─────────────────────────────┐
│ 统计 imageCount / videoCount │
│ totalMedia = img + vid       │
└──────────┬──────────────────┘
           │
           ▼
┌──────────────────────────────────────────┐
│ imageRatio = imageCount / totalMedia      │
│ videoRatio = videoCount / totalMedia      │
├──────────────────────────────────────────┤
│ imageRatio >= 0.85  → photo              │
│ videoRatio >= 0.85  → video              │
│ imageRatio >= 0.15                        │
│   AND videoRatio >= 0.15  → mixed        │
│ totalMedia == 0    → unknown             │
└──────────────────────────────────────────┘
```

### 6.3 Magic Bytes 验证（扩展名不可信时）

```go
// 仅对扩展名缺失或异常的文件做 magic bytes 检测
var magicBytes = []struct {
    mime   string
    offset int
    magic  []byte
}{
    {"image/jpeg",   0, []byte{0xFF, 0xD8, 0xFF}},
    {"image/png",    0, []byte{0x89, 0x50, 0x4E, 0x47}},
    {"image/gif",    0, []byte{0x47, 0x49, 0x46, 0x38}},
    {"image/webp",   8, []byte{0x57, 0x45, 0x42, 0x50}},
    {"video/mp4",    4, []byte{0x66, 0x74, 0x79, 0x70}}, // ftyp
    {"video/x-matroska", 0, []byte{0x1A, 0x45, 0xDF, 0xA3}},
}
```

### 6.4 预设重命名模板

```
{name}              原文件夹名
{index}             序号（从1开始）
{index:03d}         补零序号（001, 002...）
{date}              今日日期 YYYYMMDD
{year}              年份
{month}             月份
{category}          分类名（photo/video/mixed/manga）
```

**预设脚本示例：**

| 名称 | 模板 | 示例结果 |
|------|------|---------|
| 序号前缀 | `{index:03d}_{name}` | `001_SomeFolder` |
| 日期前缀 | `{date}_{name}` | `20260319_SomeFolder` |
| 分类+序号 | `{category}_{index:03d}` | `photo_001` |
| 原名不变 | `{name}` | `SomeFolder` |

---

## 7. 项目目录结构

```
classifier/
├── Dockerfile                      # 3-stage 构建
├── docker-compose.yml              # NAS 部署编排
├── .env.example                    # 环境变量模板
├── .gitignore
│
├── backend/                        # Go 后端
│   ├── cmd/
│   │   └── server/
│   │       └── main.go             # 入口：初始化 DB、路由、启动服务
│   ├── internal/
│   │   ├── api/
│   │   │   ├── router.go           # Gin 路由注册
│   │   │   ├── middleware/
│   │   │   │   ├── cors.go
│   │   │   │   └── logger.go
│   │   │   └── handler/
│   │   │       ├── folder.go       # Folder CRUD handler
│   │   │       ├── job.go          # Job 提交/查询 handler
│   │   │       ├── rule.go         # RenameRule handler
│   │   │       ├── config.go       # Config handler
│   │   │       └── events.go       # SSE handler
│   │   ├── service/
│   │   │   ├── scanner.go          # 扫描目录，写入 Folder 记录
│   │   │   ├── classifier.go       # 分类算法
│   │   │   ├── renamer.go          # 重命名逻辑 + 模板解析
│   │   │   ├── compressor.go       # ZIP 压缩（archive/zip）
│   │   │   ├── thumbnail.go        # FFmpeg 缩略图生成
│   │   │   └── mover.go            # 文件夹移动
│   │   ├── worker/
│   │   │   ├── pool.go             # 有界 goroutine 池（semaphore）
│   │   │   └── scheduler.go        # Job 队列调度，分发到 pool
│   │   ├── store/
│   │   │   ├── db.go               # SQLite 初始化、migration
│   │   │   ├── folder.go           # Folder CRUD
│   │   │   ├── job.go              # Job CRUD
│   │   │   ├── rule.go             # RenameRule CRUD
│   │   │   └── config.go           # Config KV 读写
│   │   ├── model/
│   │   │   ├── folder.go           # Folder struct + 常量
│   │   │   ├── job.go              # Job struct + 常量
│   │   │   ├── rule.go             # RenameRule struct
│   │   │   └── config.go           # AppConfig struct
│   │   └── sse/
│   │       └── broker.go           # SSE 事件广播（channel-based）
│   ├── go.mod
│   └── go.sum
│
├── frontend/                       # React + TypeScript 前端
│   ├── index.html
│   ├── vite.config.ts
│   ├── tsconfig.json
│   ├── package.json
│   ├── tailwind.config.ts
│   └── src/
│       ├── main.tsx                # 入口
│       ├── App.tsx                 # 路由配置
│       ├── types/
│       │   └── index.ts            # 所有 TS 类型定义（Folder/Job/Rule/Config）
│       ├── api/
│       │   ├── client.ts           # axios 实例 + 拦截器
│       │   ├── folders.ts          # Folder API 调用
│       │   ├── jobs.ts             # Job API 调用
│       │   ├── rules.ts            # Rule API 调用
│       │   └── config.ts           # Config API 调用
│       ├── stores/
│       │   ├── folderStore.ts      # Zustand folder store
│       │   ├── jobStore.ts         # Zustand job store
│       │   ├── ruleStore.ts        # Zustand rule store
│       │   └── configStore.ts      # Zustand config store
│       ├── hooks/
│       │   └── useSSE.ts           # SSE 连接 + 事件分发
│       ├── pages/
│       │   ├── FolderListPage.tsx
│       │   ├── JobsPage.tsx
│       │   ├── RulesPage.tsx
│       │   └── SettingsPage.tsx
│       └── components/
│           ├── layout/
│           │   ├── Layout.tsx
│           │   ├── Sidebar.tsx
│           │   └── TopBar.tsx
│           ├── folder/
│           │   ├── FolderTable.tsx
│           │   ├── FolderRow.tsx
│           │   ├── FolderToolbar.tsx
│           │   ├── CategoryBadge.tsx
│           │   └── CategoryEditDialog.tsx
│           ├── job/
│           │   ├── JobCard.tsx
│           │   ├── JobList.tsx
│           │   └── JobDetailDrawer.tsx
│           └── rule/
│               ├── RuleCard.tsx
│               ├── RuleFormDialog.tsx
│               └── RulePreviewDialog.tsx
│
└── docs/
    ├── REQUIREMENTS.md
    ├── RESEARCH.md
    ├── EMBY_THUMBNAILS.md
    ├── TECH_STACK.md
    └── ARCHITECTURE.md             # 本文档
```

---

## 8. Docker 配置

### Dockerfile

```dockerfile
# syntax=docker/dockerfile:1

# Stage 1: 前端构建
FROM node:22-alpine AS frontend-build
WORKDIR /web
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ .
RUN npm run build

# Stage 2: 后端构建
FROM golang:1.23-alpine AS backend-build
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ .
COPY --from=frontend-build /web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /out/classifier \
    ./cmd/server

# Stage 3: 运行时镜像
FROM alpine:3.20
RUN apk add --no-cache ffmpeg ca-certificates tzdata
WORKDIR /app
COPY --from=backend-build /out/classifier /app/classifier
VOLUME ["/config"]
EXPOSE 8080
ENTRYPOINT ["/app/classifier"]
```

### docker-compose.yml

```yaml
services:
  classifier:
    build:
      context: .
      dockerfile: Dockerfile
    image: classifier:latest
    container_name: classifier
    restart: unless-stopped
    ports:
      - "${APP_PORT:-8080}:8080"
    environment:
      CONFIG_DIR: /config
      SOURCE_DIR: /data/source
      TARGET_DIR: /data/target
      MAX_CONCURRENCY: ${MAX_CONCURRENCY:-3}
      TZ: ${TZ:-Asia/Shanghai}
    volumes:
      - type: bind
        source: ${SOURCE_DIR:?SOURCE_DIR is required}
        target: /data/source
        read_only: true
        bind:
          create_host_path: false
      - type: bind
        source: ${TARGET_DIR:?TARGET_DIR is required}
        target: /data/target
        bind:
          create_host_path: false
      - type: volume
        source: classifier-config
        target: /config
        volume:
          nocopy: true
      - type: tmpfs
        target: /tmp/work
        tmpfs:
          size: 1073741824  # 1GB FFmpeg 临时空间
    deploy:
      resources:
        limits:
          cpus: "${CPU_LIMIT:-2.0}"
          memory: ${MEMORY_LIMIT:-1G}
        reservations:
          cpus: "0.25"
          memory: 256M

volumes:
  classifier-config:
```

### .env.example

```env
# 端口
APP_PORT=8080

# NAS 上的实际路径（必填）
SOURCE_DIR=/volume1/media/incoming
TARGET_DIR=/volume1/media/processed

# 并发处理数量（建议 NAS 上设为 2-3）
MAX_CONCURRENCY=3

# 时区
TZ=Asia/Shanghai

# 资源限制
CPU_LIMIT=2.0
MEMORY_LIMIT=1G
```

---

## 9. 开发路线图

### Phase 1 — MVP（核心扫描 + 分类 + 移动）

目标：能扫描目录、自动分类、手动修正、移动文件夹

- [ ] 后端项目初始化（Go module、Gin、SQLite）
- [ ] Docker 多阶段构建配置
- [ ] 数据库 schema + migration
- [ ] Scanner Service：递归扫描 /data/source
- [ ] Classifier Service：扩展名 + 比例判断
- [ ] Folder CRUD API（GET/PATCH/DELETE）
- [ ] SSE Broker 基础实现
- [ ] Move Service：移动文件夹到 /data/target
- [ ] 前端项目初始化（Vite + React + TypeScript + Tailwind + shadcn/ui）
- [ ] FolderListPage：列表、筛选、分类修正
- [ ] SettingsPage：source/target 目录配置
- [ ] SSE hook：连接 /api/events，更新 store

**验收标准：** 能扫描 → 看到分类结果 → 手动修正 → 移动到目标目录

---

### Phase 2 — 批量操作（重命名 + 压缩 + 缩略图）

目标：完整的批量处理能力

- [ ] Job Scheduler：队列 + 有界并发 goroutine pool
- [ ] Rename Service：模板解析 + 批量重命名
- [ ] 预设重命名规则（4个内置模板）
- [ ] Compress Service：archive/zip 快速压缩图片目录
- [ ] Thumbnail Service：FFmpeg 生成 Emby 规范缩略图
- [ ] Job API（POST/GET/DELETE）
- [ ] RenameRule API（CRUD + preview）
- [ ] JobsPage：任务列表 + 实时进度
- [ ] RulesPage：规则管理 + 预览
- [ ] FolderToolbar：批量操作按钮

**验收标准：** 能批量重命名、压缩图片目录、生成视频缩略图，任务进度实时可见

---

### Phase 3 — 并发优化 + 体验打磨

目标：生产可用，流畅体验

- [ ] 并发扫描（多目录同时扫描）
- [ ] 扫描增量更新（只扫描新增/变更文件夹）
- [ ] Magic bytes 检测（处理无扩展名文件）
- [ ] 任务取消（cancel running job）
- [ ] 错误重试机制
- [ ] 前端虚拟列表（大量文件夹时性能优化）
- [ ] 操作历史记录
- [ ] 配置导入/导出
- [ ] 健康检查端点（/health）

**验收标准：** 处理 1000+ 文件夹流畅，任务可取消，错误可恢复

---

## 附录：关键技术决策记录

| 决策 | 选择 | 放弃 | 理由 |
|------|------|------|------|
| 后端框架 | Gin | Echo, Fiber | 生态最成熟，单人开发友好 |
| 数据库 | SQLite | JSON文件, PostgreSQL | 无 CGO，轻量，够用 |
| 前端状态 | Zustand | Redux, Jotai | 极简 API，无 boilerplate |
| 实时推送 | SSE | WebSocket | 单向推送，实现更简单 |
| 压缩 | archive/zip | zstd, 7z | 标准库，兼容性最好 |
| 视频处理 | FFmpeg exec | ffmpeg-go | 无 CGO 依赖，更易 Docker 化 |
| 组件库 | shadcn/ui | Ant Design, MUI | 无运行时，构建产物小 |
