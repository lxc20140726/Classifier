# 开发路线图

> 版本：v3.3 | 日期：2026-03-20
> 当前实现以仓库实际代码为准，v3 目标设计见 [ROADMAP_V3.md](ROADMAP_V3.md)。本文件只跟踪当前仓库里的"已完成 / 进行中 / 下一步"。

## 已完成

### Phase 1 - MVP 基础能力已落地

- [x] Go + Gin + SQLite 基础工程
- [x] Folder 扫描、分类、列表、详情、分类/状态修改
- [x] Move 基础链路（创建任务、异步执行、落盘进度、SSE 通知）
- [x] Snapshot 创建、查询、回滚
- [x] AuditLog 后端持久化
- [x] Config 读取与保存接口
- [x] 前端 FolderList / Settings / SnapshotDrawer
- [x] Docker / docker-compose / NAS 部署基础资产

### Phase 1.5 - v3 基础设施全部落地

后端：

- [x] `jobs` 表与 `JobRepository`
- [x] `POST /api/jobs/move`
- [x] `GET /api/jobs`
- [x] `GET /api/jobs/:id`
- [x] `GET /api/jobs/:id/progress`
- [x] MoveService 持久化 Job 状态与进度
- [x] Folder 软删除
- [x] `POST /api/folders/:id/restore`
- [x] `folders` 查询默认过滤已删除记录
- [x] v3 设计文档集补齐

前端：

- [x] `src/api/jobs.ts`：`listJobs` / `getJob` / `getJobProgress`
- [x] `src/store/jobStore.ts`：Zustand Job 状态管理 + 2s 轮询
- [x] `src/pages/JobsPage.tsx`：Job 列表、状态 badge、进度条、可展开详情
- [x] FolderListPage：多选 + Move 弹窗（调用 `POST /api/jobs/move`）
- [x] FolderListPage：软删除 Folder 显示 Restore 按钮（调用 `POST /api/folders/:id/restore`）
- [x] `src/api/folders.ts` 补齐 `restoreFolder`
- [x] `useSSE.ts`：处理 `job.progress` / `job.done` / `job.error` 事件
- [x] `/jobs` 路由上线，侧边栏补齐 Jobs 导航项
- [x] `src/types/index.ts`：补齐 `Job` / `JobStatus` / `JobProgress` 类型

## 进行中

### Phase 2 - 从单一 move job 过渡到通用工作流执行模型

- [ ] 新增 `workflow_runs`
- [ ] 新增 `node_runs`
- [ ] 新增 `node_snapshots`
- [ ] 新增 `workflow_definitions`
- [ ] 实现 WorkflowRunner
- [ ] 实现 NodeExecutor 注册与调度机制
- [ ] 将当前 `MoveService` 下沉为 `move` 节点执行器
- [ ] 实现 WorkflowRun resume / rollback

## 下一步

### Phase 2.5 - 补齐剩余产品流

- [ ] 前端补齐分页翻页体验
- [ ] 补齐 audit log HTTP API
- [ ] 补齐 AuditLog 前端视图

### Phase 3 - 节点能力补齐

- [ ] Rename 节点（文件 + 文件夹）
- [ ] Compress 节点
- [ ] Thumbnail 节点
- [ ] 节点级 snapshot 与补偿
- [ ] 条件节点与等待节点

### Phase 4 - 分类器节点化

- [ ] `name-keyword-classifier`
- [ ] `file-tree-classifier`
- [ ] `ext-ratio-classifier`
- [ ] `manual-classifier`

### Phase 5 - 前端与配置系统升级

- [ ] JobsPage 升级为三层结构（Job / WorkflowRun / NodeRun）
- [ ] WorkflowsPage 节点编辑器
- [ ] Rename/Compress/Thumbnail 等节点配置 UI
- [ ] SSE + HTTP polling fallback 完整落地
- [ ] `app_config` 结构化配置与迁移
- [ ] SettingsPage 重构为强类型配置表单

## 风险与约束

- 当前仍未实现真正的 WorkflowRunner；现有 Job 只覆盖 move 任务
- 当前执行模型仍以 handler 内直接起 goroutine 为主，尚未切换到通用 workflow engine
- 当前配置系统仍是简单 KV + env，尚未迁移到 `app_config`
- 当前 Snapshot 仍是 folder/job 粒度，尚未切换到 node 粒度
- AuditLog 目前只有后端存储能力，尚未开放独立 HTTP 查询接口
- 运行时迁移以 `backend/internal/db/migrations` 为准；`backend/migrations` 目前是落后的拷贝，后续需要同步整理
