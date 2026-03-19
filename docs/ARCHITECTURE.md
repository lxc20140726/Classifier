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
GET    /api/folders                        # 列表（?status=&category=&page=&limit=）
POST   /api/folders/scan                   # 触发扫描 /data/source
GET    /api/folders/:id                    # 单个详情
PATCH  /api/folders/:id/category           # 手动修正分类
PATCH  /api/folders/:id/status             # 标记状态（pending/done/skip）
DELETE /api/folders/:id                    # 从 DB 移除记录（不删文件）

# 批量操作（提交 Job）
POST   /api/jobs/rename                    # 批量重命名
POST   /api/jobs/compress                  # 批量压缩
POST   /api/jobs/thumbnail                 # 批量生成缩略图
POST   /api/jobs/move                      # 批量移动
POST   /api/jobs/run-workflow              # 运行工作流（指定 folder_ids + workflow_id）

# Job 管理
GET    /api/jobs                           # 任务列表（?status=running|done|failed）
GET    /api/jobs/:id                       # 任务详情
DELETE /api/jobs/:id                       # 取消/删除任务

# 重命名规则
GET    /api/rename-rules                   # 规则列表
POST   /api/rename-rules                   # 创建规则
PUT    /api/rename-rules/:id               # 更新规则
DELETE /api/rename-rules/:id               # 删除规则
POST   /api/rules/preview                  # 预览重命名结果（rule_id + folder_id）
GET    /api/rename-rules/tokens            # 获取可用 Token 列表

# 工作流
GET    /api/workflows                      # 所有工作流列表
GET    /api/workflows/:id                  # 单个工作流（含 steps）
POST   /api/workflows                      # 创建工作流
PUT    /api/workflows/:id                  # 更新工作流（steps 全量替换）
DELETE /api/workflows/:id                  # 删除工作流

# 快照 / 回退
GET    /api/snapshots                      # 快照列表（?folder_id=&job_id=）
GET    /api/snapshots/:id                  # 快照详情（before/after diff）
POST   /api/snapshots/:id/revert           # 执行回退操作

# 审计日志
GET    /api/logs                           # 日志列表（?action=&result=&folder_id=&from=&to=）
GET    /api/logs/:id                       # 单条日志详情
GET    /api/logs/export                    # 导出日志（?format=json|csv&from=&to=）

# 配置
GET    /api/config                         # 读取配置
PUT    /api/config                         # 更新配置

# SSE
GET    /api/events                         # SSE 事件流

# 健康检查
GET    /health                             # 健康检查端点
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
/workflows  → WorkflowsPage     可视化工作流配置 [NEW]
/logs       → LogsPage          审计日志 [NEW]
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
│   │   │   ├── router.go           # Gin 路由注册（包含所有端点）
│   │   │   ├── middleware/
│   │   │   │   ├── cors.go
│   │   │   │   └── logger.go
│   │   │   └── handler/
│   │   │       ├── folder.go       # Folder CRUD handler
│   │   │       ├── job.go          # Job 提交/查询 handler
│   │   │       ├── rule.go         # RenameRule handler
│   │   │       ├── config.go       # Config handler
│   │   │       ├── events.go       # SSE handler
│   │   │       ├── workflow.go     # 工作流 CRUD + 执行 handler  [NEW]
│   │   │       ├── snapshot.go     # Snapshot 查询 + 回退 handler  [NEW]
│   │   │       └── auditlog.go     # 审计日志查询 + 导出 handler  [NEW]
│   │   ├── service/
│   │   │   ├── scanner.go          # 扫描目录，写入 Folder 记录
│   │   │   ├── classifier.go       # 分类算法
│   │   │   ├── renamer.go          # 重命名逻辑 + Token 模板解析
│   │   │   ├── compressor.go       # ZIP 压缩（archive/zip）
│   │   │   ├── thumbnail.go        # FFmpeg 缩略图生成
│   │   │   ├── mover.go            # 文件夹移动
│   │   │   ├── workflow.go         # 工作流执行引擎（按步骤调度各 service）  [NEW]
│   │   │   ├── snapshot.go         # 操作前快照 + 回退逻辑  [NEW]
│   │   │   └── auditlog.go         # 审计日志写入逻辑  [NEW]
│   │   ├── worker/
│   │   │   ├── pool.go             # 有界 goroutine 池（semaphore）
│   │   │   └── scheduler.go        # Job 队列调度，分发到 pool
│   │   ├── store/
│   │   │   ├── db.go               # SQLite 初始化、migration
│   │   │   ├── folder.go           # Folder CRUD
│   │   │   ├── job.go              # Job CRUD
│   │   │   ├── rule.go             # RenameRule CRUD
│   │   │   ├── config.go           # Config KV 读写
│   │   │   ├── workflow.go         # Workflow CRUD  [NEW]
│   │   │   ├── snapshot.go         # Snapshot CRUD  [NEW]
│   │   │   └── auditlog.go         # AuditLog append + 查询  [NEW]
│   │   ├── model/
│   │   │   ├── folder.go           # Folder struct + 常量
│   │   │   ├── job.go              # Job struct + 常量
│   │   │   ├── rule.go             # RenameRule struct
│   │   │   ├── config.go           # AppConfig struct
│   │   │   ├── workflow.go         # Workflow + WorkflowStep struct  [NEW]
│   │   │   ├── snapshot.go         # Snapshot struct  [NEW]
│   │   │   └── auditlog.go         # AuditLog struct  [NEW]
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
│       │   ├── WorkflowsPage.tsx    # 工作流管理 [NEW]
│       │   ├── LogsPage.tsx         # 审计日志 [NEW]
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
│           │   ├── JobDetailDrawer.tsx
│           │   └── UndoButton.tsx       # 快照回退 [NEW]
│           ├── workflow/              # 工作流组件 [NEW]
│           │   ├── WorkflowList.tsx
│           │   ├── WorkflowCard.tsx
│           │   ├── WorkflowEditor.tsx
│           │   ├── StepCard.tsx
│           │   └── StepConfigPanel.tsx
│           ├── rename-editor/         # Token 重命名编辑器 [NEW]
│           │   ├── RenameEditor.tsx
│           │   ├── TokenBadge.tsx
│           │   ├── TokenPicker.tsx
│           │   └── RenamePreview.tsx
│           ├── log/                   # 日志组件 [NEW]
│           │   ├── LogTable.tsx
│           │   ├── LogDetailDrawer.tsx
│           │   └── LogExportButton.tsx
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

### Phase 1 — MVP（核心扫描 + 分类 + 移动 + 安全基础）

目标：能扫描目录、自动分类、手动修正、移动文件夹，并建立安全操作基础

**后端：**
- [ ] 后端项目初始化（Go module、Gin、SQLite）
- [ ] Docker 多阶段构建配置
- [ ] 数据库 schema + migration（folders / snapshots / audit_logs）
- [ ] Scanner Service：递归扫描 /data/source
- [ ] Classifier Service：扩展名 + 比例判断分类
- [ ] Folder CRUD API（GET/PATCH/DELETE）
- [ ] SSE Broker 基础实现
- [ ] Move Service：移动文件夹到 /data/target
- [ ] Snapshot Service：移动前自动创建快照（记录原始路径）
- [ ] Snapshot API（GET /api/snapshots、POST /api/snapshots/:id/revert）
- [ ] AuditLog Service：写入扫描、分类、移动操作日志
- [ ] 健康检查端点（/health）

**前端：**
- [ ] 前端项目初始化（Vite + React + TypeScript + Tailwind + shadcn/ui）
- [ ] FolderListPage：列表、筛选、分类修正
- [ ] SettingsPage：source/target 目录配置
- [ ] SSE hook：连接 /api/events，更新 Zustand store
- [ ] SnapshotDrawer：查看操作快照、一键回退

**验收标准：** 能扫描 → 看到分类结果 → 手动修正 → 移动到目标目录 → 出错可回退

---

### Phase 2 — 批量操作（重命名 + 压缩 + 缩略图 + 工作流）

目标：完整的批量处理能力 + 可视化工作流配置

**后端：**
- [ ] Job Scheduler：队列 + 有界并发 goroutine pool
- [ ] Rename Service：Token 模板解析 + 批量重命名
- [ ] Token 系统：{index} {date} {name} {category} {ext} 等内置变量
- [ ] 预设重命名规则（4 个内置模板）
- [ ] Compress Service：archive/zip 快速压缩图片目录
- [ ] Thumbnail Service：FFmpeg 生成 Emby 规范缩略图
- [ ] Job API（POST/GET/DELETE /api/jobs/*）
- [ ] RenameRule API（CRUD + POST /api/rename-rules/:id/preview）
- [ ] Workflow Service：按分类加载并执行步骤链
- [ ] Workflow API（GET/POST/PUT/DELETE /api/workflows）
- [ ] Snapshot：重命名/压缩前自动快照
- [ ] AuditLog：写入重命名、压缩、缩略图操作

**前端：**
- [ ] JobsPage：任务列表 + 实时进度条
- [ ] RulesPage：Token 重命名编辑器（拖拽 Token、实时预览）
- [ ] WorkflowsPage：每种分类配置步骤链（拖拽排序）
- [ ] LogsPage：审计日志列表 + 筛选 + CSV 导出
- [ ] FolderToolbar：批量操作按钮（重命名/压缩/缩略图/移动）

**验收标准：** 能配置工作流 → 批量重命名（预览正确）→ 压缩图片目录 → 生成 Emby 缩略图 → 审计日志可查

---

### Phase 3 — 并发优化 + 体验打磨 + 高级功能

目标：生产可用，流畅体验，支持大规模文件夹处理

**性能：**
- [ ] 并发扫描（多目录同时扫描）
- [ ] 扫描增量更新（只扫描新增/变更文件夹）
- [ ] Magic bytes 检测（处理无扩展名文件）
- [ ] 前端虚拟列表（react-window，大量文件夹时性能优化）
- [ ] 任务取消（cancel running job）
- [ ] 错误重试机制（失败 Job 自动重试 3 次）

**工作流增强：**
- [ ] 工作流条件步骤（如：仅当文件数 > N 时执行压缩）
- [ ] 工作流执行历史（每次执行记录到审计日志）
- [ ] 自定义分类规则（用户可添加新的扩展名映射）

**运维：**
- [ ] 审计日志自动清理（保留最近 N 天）
- [ ] 配置导入/导出（JSON 格式）
- [ ] 快照自动过期清理（保留最近 30 天）
- [ ] Docker 健康检查完善

**验收标准：** 处理 1000+ 文件夹流畅，任务可取消，错误可恢复，审计日志完整

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

---

## 10. 可视化工作流系统设计

### 10.1 核心概念

每种文件夹分类（photo/video/mixed/manga）对应一个独立的**工作流管道**，由有序的处理步骤组成。用户可通过拖拽界面自由增删、排序步骤。

```
工作流 = 分类触发器 + 有序步骤列表

示例：photo 工作流
  Step 1: 重命名（规则 A）
  Step 2: 压缩为 ZIP
  Step 3: 移动到 /data/target/photos

示例：video 工作流
  Step 1: 生成 Emby 缩略图
  Step 2: 重命名（规则 B）
  Step 3: 移动到 /data/target/videos
```

### 10.2 数据模型

```go
// 工作流定义
type Workflow struct {
    ID        string         `json:"id" db:"id"`
    Category  Category       `json:"category" db:"category"` // photo|video|mixed|manga|custom
    Name      string         `json:"name" db:"name"`
    Steps     []WorkflowStep `json:"steps" db:"-"`           // 从 workflow_steps 表 JOIN
    CreatedAt time.Time      `json:"created_at" db:"created_at"`
    UpdatedAt time.Time      `json:"updated_at" db:"updated_at"`
}

// 工作流步骤
type WorkflowStep struct {
    ID         string          `json:"id" db:"id"`
    WorkflowID string          `json:"workflow_id" db:"workflow_id"`
    Order      int             `json:"order" db:"order"`
    StepType   WorkflowStepType `json:"step_type" db:"step_type"`
    Config     json.RawMessage `json:"config" db:"config"` // 步骤专属参数 JSON
    Enabled    bool            `json:"enabled" db:"enabled"`
}

type WorkflowStepType string
const (
    StepRename    WorkflowStepType = "rename"
    StepCompress  WorkflowStepType = "compress"
    StepThumbnail WorkflowStepType = "thumbnail"
    StepMove      WorkflowStepType = "move"
    StepWait      WorkflowStepType = "wait"   // 暂停等待用户确认
)

// 各步骤 Config 结构示例
type RenameStepConfig struct {
    RuleID string `json:"rule_id"`
}
type CompressStepConfig struct {
    OutputDir   string `json:"output_dir"`   // 留空=原地
    DeleteAfter bool   `json:"delete_after"` // 压缩后删除源文件
}
type MoveStepConfig struct {
    TargetDir string `json:"target_dir"` // 留空=全局配置
}
type ThumbnailStepConfig struct {
    SeekOffset string `json:"seek_offset"` // 默认 "00:10:00"
    Overwrite  bool   `json:"overwrite"`
}
```

### 10.3 SQLite Schema

```sql
CREATE TABLE workflows (
    id         TEXT PRIMARY KEY,
    category   TEXT NOT NULL,
    name       TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE workflow_steps (
    id          TEXT PRIMARY KEY,
    workflow_id TEXT NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    "order"     INTEGER NOT NULL,
    step_type   TEXT NOT NULL,
    config      TEXT NOT NULL DEFAULT '{}',
    enabled     INTEGER NOT NULL DEFAULT 1
);

CREATE INDEX idx_workflow_steps_workflow_id ON workflow_steps(workflow_id, "order");
```

### 10.4 API 设计

```
GET    /api/workflows                     # 所有工作流列表
GET    /api/workflows/:id                 # 单个工作流（含 steps）
POST   /api/workflows                     # 创建工作流
PUT    /api/workflows/:id                 # 更新工作流（含 steps 全量替换）
DELETE /api/workflows/:id                 # 删除工作流

POST   /api/jobs/run-workflow             # 对指定 folder_ids 运行工作流
```

### 10.5 前端组件设计

```
RulesPage（/rules 路由扩展为 /workflows）
├── WorkflowList                    # 左侧：按分类列出工作流卡片
│   └── WorkflowCard（category badge + step count + 编辑按钮）
│
└── WorkflowEditor（右侧面板）
    ├── CategorySelector            # 关联分类（photo/video/mixed/manga）
    ├── StepList                    # 拖拽排序（dnd-kit）
    │   └── StepCard
    │       ├── StepTypeIcon
    │       ├── StepConfigSummary   # 一行摘要："重命名: {规则名}"
    │       ├── EnableToggle
    │       └── ConfigExpandPanel   # 展开详细配置
    ├── AddStepButton               # 下拉选择步骤类型
    └── SaveButton
```

**拖拽排序**：使用 `@dnd-kit/core` + `@dnd-kit/sortable`，无需额外依赖。

---

## 11. 可回退操作（Undo / Snapshot）设计

### 11.1 设计原则

- **操作前快照**：所有破坏性操作（重命名、移动、删除）执行前，记录原始状态
- **快照粒度**：以单个文件夹为单位，记录操作前的文件列表和路径
- **回退范围**：支持回退到任意历史操作节点
- **不可回退操作**：压缩（ZIP 生成是追加操作，原文件不变，无需回退）

### 11.2 Snapshot 数据模型

```go
type SnapshotStatus string
const (
    SnapshotPending   SnapshotStatus = "pending"   // 操作进行中
    SnapshotCommitted SnapshotStatus = "committed" // 操作完成，可回退
    SnapshotReverted  SnapshotStatus = "reverted"  // 已回退
)

type Snapshot struct {
    ID          string         `json:"id" db:"id"`
    JobID       string         `json:"job_id" db:"job_id"`
    FolderID    string         `json:"folder_id" db:"folder_id"`
    OperationType string       `json:"operation_type" db:"operation_type"` // rename|move
    Before      json.RawMessage `json:"before" db:"before"` // 操作前文件路径列表
    After       json.RawMessage `json:"after" db:"after"`   // 操作后文件路径列表
    Status      SnapshotStatus `json:"status" db:"status"`
    CreatedAt   time.Time      `json:"created_at" db:"created_at"`
}

// Before / After 结构
type FileSnapshot struct {
    Files []FileRecord `json:"files"`
}

type FileRecord struct {
    OriginalPath string `json:"original_path"`
    CurrentPath  string `json:"current_path"`
}
```

### 11.3 SQLite Schema

```sql
CREATE TABLE snapshots (
    id             TEXT PRIMARY KEY,
    job_id         TEXT NOT NULL,
    folder_id      TEXT NOT NULL,
    operation_type TEXT NOT NULL,
    before         TEXT NOT NULL, -- JSON
    after          TEXT,          -- NULL 直到操作完成
    status         TEXT NOT NULL DEFAULT 'pending',
    created_at     DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_snapshots_job_id ON snapshots(job_id);
CREATE INDEX idx_snapshots_folder_id ON snapshots(folder_id);
```

### 11.4 回退流程

```
用户触发 Revert(snapshot_id)
  ├── 查询 snapshot.after（当前路径列表）
  ├── 对每个文件：os.Rename(current_path, original_path)
  ├── 全部成功 → 更新 snapshot.status = "reverted"
  │            → 更新 folder 记录恢复原始状态
  └── 任意失败 → 回滚已重命名的文件，返回错误
```

### 11.5 API 设计

```
GET    /api/snapshots?folder_id=&job_id=   # 查询快照列表
GET    /api/snapshots/:id                  # 快照详情（before/after diff）
POST   /api/snapshots/:id/revert           # 执行回退
```

### 11.6 前端设计

- **JobsPage** 每个已完成任务旁显示「撤销」按钮（仅 rename/move 类型）
- 点击撤销 → 弹出确认 Dialog，展示 before/after 文件路径对比
- 确认后调用 `/api/snapshots/:id/revert`，实时显示回退进度
- 已撤销的任务显示「已撤销」badge，不可重复撤销

---

## 12. 审计日志系统设计

### 12.1 设计原则

- **全量记录**：所有文件操作（分类、重命名、压缩、移动）均写入日志
- **不可篡改**：日志只追加，不修改，不删除
- **结构化存储**：JSON 格式，方便查询和导出
- **轻量实现**：写入 SQLite，同时可导出为 CSV/JSON 文件

### 12.2 日志数据模型

```go
type AuditLevel string
const (
    AuditInfo  AuditLevel = "info"
    AuditWarn  AuditLevel = "warn"
    AuditError AuditLevel = "error"
)

type AuditAction string
const (
    ActionScan      AuditAction = "scan"      // 扫描目录
    ActionClassify  AuditAction = "classify"  // 分类文件夹
    ActionRename    AuditAction = "rename"    // 重命名文件
    ActionCompress  AuditAction = "compress"  // 压缩为 ZIP
    ActionThumbnail AuditAction = "thumbnail" // 生成缩略图
    ActionMove      AuditAction = "move"      // 移动文件夹
    ActionRevert    AuditAction = "revert"    // 回退操作
)

type AuditLog struct {
    ID         string          `json:"id" db:"id"`
    JobID      string          `json:"job_id" db:"job_id"`
    FolderID   string          `json:"folder_id" db:"folder_id"`
    FolderPath string          `json:"folder_path" db:"folder_path"`
    Action     AuditAction     `json:"action" db:"action"`
    Level      AuditLevel      `json:"level" db:"level"`
    Detail     json.RawMessage `json:"detail" db:"detail"` // 操作具体参数
    Result     string          `json:"result" db:"result"` // success|failure
    Error      string          `json:"error,omitempty" db:"error"`
    DurationMs int64           `json:"duration_ms" db:"duration_ms"`
    CreatedAt  time.Time       `json:"created_at" db:"created_at"`
}

// Detail 示例（rename）
type RenameDetail struct {
    RuleID   string            `json:"rule_id"`
    RuleName string            `json:"rule_name"`
    Files    []RenameFileLog   `json:"files"`
}
type RenameFileLog struct {
    From string `json:"from"`
    To   string `json:"to"`
}

// Detail 示例（move）
type MoveDetail struct {
    From string `json:"from"`
    To   string `json:"to"`
}

// Detail 示例（compress）
type CompressDetail struct {
    OutputFile   string `json:"output_file"`
    FilesCount   int    `json:"files_count"`
    OriginalSize int64  `json:"original_size_bytes"`
    ZipSize      int64  `json:"zip_size_bytes"`
}
```

### 12.3 SQLite Schema

```sql
CREATE TABLE audit_logs (
    id          TEXT PRIMARY KEY,
    job_id      TEXT,
    folder_id   TEXT,
    folder_path TEXT NOT NULL,
    action      TEXT NOT NULL,
    level       TEXT NOT NULL DEFAULT 'info',
    detail      TEXT,          -- JSON
    result      TEXT NOT NULL, -- success|failure
    error       TEXT,
    duration_ms INTEGER,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_audit_logs_folder_id ON audit_logs(folder_id);
CREATE INDEX idx_audit_logs_action    ON audit_logs(action);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at);
```

### 12.4 日志写入时机

| 操作 | 写入时机 | 记录内容 |
|------|----------|----------|
| 扫描 | 完成后 | 扫描目录、发现文件夹数量 |
| 分类 | 每个文件夹分类后 | 分类结果、依据（auto/manual） |
| 重命名 | 每个文件操作后 | from/to 路径、使用的规则 |
| 压缩 | 完成后 | 输出文件、原始大小、压缩大小 |
| 缩略图 | 每个视频完成后 | 输出路径、耗时 |
| 移动 | 每个文件夹完成后 | from/to 路径 |
| 回退 | 完成后 | 关联的原始操作 snapshot_id |

### 12.5 API 设计

```
GET  /api/logs?action=&result=&folder_id=&from=&to=&page=&limit=  # 查询日志
GET  /api/logs/:id                                                 # 单条详情
GET  /api/logs/export?format=json|csv&from=&to=                   # 导出日志
```

### 12.6 前端设计

- **专属日志页面**（路由 `/logs`）：
  - 表格展示：时间、操作类型、文件夹路径、结果、耗时
  - 筛选：操作类型、结果（成功/失败）、时间范围
  - 点击展开 Detail 面板，展示完整操作信息
  - 导出按钮：下载 JSON 或 CSV 文件
- **错误日志高亮**：level=error 行红色标注
- **关联跳转**：点击文件夹路径跳转到 FolderListPage 对应文件夹

---

## 13. 可视化重命名规则编辑器

### 13.1 设计原则

**目标用户**：不懂正则表达式的普通用户也能快速上手。

- **无正则**：所有规则通过变量插槽（Token）组合，无需手写正则
- **所见即所得**：实时预览重命名结果
- **预设模板**：内置常用命名脚本，一键套用
- **安全预览**：执行前必须预览，避免误操作

### 13.2 Token（变量）体系

```go
// 可插入的变量 Token
type Token struct {
    Key         string `json:"key"`          // 变量标识，如 {index}
    Label       string `json:"label"`         // 中文显示名，如 "序号"
    Description string `json:"description"`  // 说明，如 "从1开始的递增序号"
    Example     string `json:"example"`       // 示例输出，如 "001"
    Config      map[string]any `json:"config"` // 可选配置项
}

// 内置 Token 列表
var BuiltinTokens = []Token{
    {Key: "{index}",      Label: "序号",     Description: "从起始值递增的序号",    Example: "001", Config: {"start": 1, "padding": 3}},
    {Key: "{filename}",   Label: "原文件名",  Description: "文件的原始名称（不含扩展名）", Example: "IMG_1234"},
    {Key: "{ext}",        Label: "扩展名",   Description: "文件扩展名（含点）",     Example: ".jpg"},
    {Key: "{foldername}", Label: "文件夹名",  Description: "所在文件夹的名称",      Example: "MyAlbum"},
    {Key: "{date}",       Label: "日期",     Description: "文件修改日期",          Example: "20240315", Config: {"format": "YYYYMMDD"}},
    {Key: "{year}",       Label: "年份",     Description: "文件修改年份",          Example: "2024"},
    {Key: "{month}",      Label: "月份",     Description: "文件修改月份",          Example: "03"},
    {Key: "{day}",        Label: "日",       Description: "文件修改日",            Example: "15"},
    {Key: "{random}",     Label: "随机码",   Description: "随机字母数字串",        Example: "a3f9", Config: {"length": 4}},
}
```

### 13.3 规则数据模型

```go
type RenameRule struct {
    ID          string          `json:"id" db:"id"`
    Name        string          `json:"name" db:"name"`
    Description string          `json:"description" db:"description"`
    IsPreset    bool            `json:"is_preset" db:"is_preset"`     // 内置预设不可删除
    Template    string          `json:"template" db:"template"`       // 如 "{foldername}_{index}{ext}"
    Config      json.RawMessage `json:"config" db:"config"`           // Token 配置参数
    ScopeType   string          `json:"scope_type" db:"scope_type"`   // files|folder
    CreatedAt   time.Time       `json:"created_at" db:"created_at"`
}
```

### 13.4 内置预设脚本

| 预设名称 | 模板 | 适用场景 |
|----------|------|----------|
| 序号重命名 | `{index}{ext}` | 批量序号化 |
| 文件夹名+序号 | `{foldername}_{index}{ext}` | 写真集 |
| 日期+序号 | `{date}_{index}{ext}` | 按日期归档 |
| 保留原名+序号 | `{filename}_{index}{ext}` | 防重名 |
| 文件夹名（整个文件夹重命名）| `{foldername}` | 文件夹本身重命名 |

### 13.5 前端编辑器 UI 设计

```
┌─────────────────────────────────────────────────────┐
│ 规则名称: [写真集默认命名            ]               │
│                                                     │
│ 命名模板:                                           │
│ ┌─────────────────────────────────────────────┐    │
│ │ [文件夹名] _ [序号] [扩展名]  + 添加变量 ▼  │    │
│ └─────────────────────────────────────────────┘    │
│   (Token 以 Badge 形式展示，可拖拽排序，可删除)      │
│                                                     │
│ 变量配置:                                           │
│   序号起始值: [1    ]  补零位数: [3  ]              │
│                                                     │
│ 实时预览:                                           │
│ ┌─────────────────────────────────────────────┐    │
│ │ IMG_001.jpg  →  MyAlbum_001.jpg             │    │
│ │ IMG_002.jpg  →  MyAlbum_002.jpg             │    │
│ │ IMG_003.jpg  →  MyAlbum_003.jpg             │    │
│ └─────────────────────────────────────────────┘    │
│                                              保存   │
└─────────────────────────────────────────────────────┘
```

### 13.6 后端渲染逻辑

```go
// 渲染模板，将 Token 替换为实际值
func RenderTemplate(template string, ctx RenameContext) string {
    result := template
    result = strings.ReplaceAll(result, "{filename}",   ctx.OriginalName)
    result = strings.ReplaceAll(result, "{ext}",        ctx.Ext)
    result = strings.ReplaceAll(result, "{foldername}", ctx.FolderName)
    result = strings.ReplaceAll(result, "{date}",       ctx.ModTime.Format("20060102"))
    result = strings.ReplaceAll(result, "{year}",       ctx.ModTime.Format("2006"))
    result = strings.ReplaceAll(result, "{month}",      ctx.ModTime.Format("01"))
    result = strings.ReplaceAll(result, "{day}",        ctx.ModTime.Format("02"))
    index := fmt.Sprintf("%0*d", ctx.Config.Padding, ctx.Index+ctx.Config.Start)
    result = strings.ReplaceAll(result, "{index}", index)
    return result
}

type RenameContext struct {
    OriginalName string
    Ext          string
    FolderName   string
    ModTime      time.Time
    Index        int
    Config       RenameConfig
}

type RenameConfig struct {
    Start   int `json:"start"`    // 序号起始值，默认 1
    Padding int `json:"padding"`  // 序号补零位数，默认 3
}
```

### 13.7 预览 API

```
POST /api/rules/preview
{
    "rule_id": "xxx",
    "folder_id": "yyy"   // 用真实文件列表生成预览
}

响应:
{
    "previews": [
        {"from": "IMG_001.jpg", "to": "MyAlbum_001.jpg"},
        {"from": "IMG_002.jpg", "to": "MyAlbum_002.jpg"}
    ],
    "conflicts": []  // 检测命名冲突
}
```

---

## 附录（补充）：新增技术决策

| 决策 | 选择 | 放弃 | 理由 |
|------|------|------|------|
| 工作流引擎 | 自研简单步骤链 | Temporal, Airflow | 轻量场景，无需分布式调度 |
| 回退机制 | Snapshot 表记录原始路径/内容 | 文件副本备份 | 省磁盘空间，rename/move 只需记录路径 |
| 审计日志 | SQLite append-only 表 | 文件日志 | 可查询、可导出、随容器持久化 |
| 重命名 UX | Token 变量组合（无正则）| 正则表达式 | 面向普通用户，降低学习门槛 |
| 工作流存储 | SQLite JSON 字段 | 独立 workflow 服务 | 单机场景够用，无需额外服务 |
