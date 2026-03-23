# 开发路线图

> 版本：v3.4 | 日期：2026-03-23
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

### Phase 1.5 - 扫描 / 历史 / 主界面增强已落地

后端：

- [x] `jobs` 表与 `JobRepository`
- [x] `scan` 任务类型：扫描动作持久化为 Job
- [x] `POST /api/jobs/move`
- [x] `GET /api/jobs`
- [x] `GET /api/jobs/:id`
- [x] `GET /api/jobs/:id/progress`
- [x] MoveService 持久化 Job 状态与进度
- [x] ScannerService：逐目录扫描、逐目录落库、逐目录立即分类
- [x] `scan.started` / `scan.progress` / `scan.error` / `scan.done` SSE 事件
- [x] Folder 软删除
- [x] `POST /api/folders/:id/restore`
- [x] `folders` 查询默认过滤已删除记录
- [x] `GET /api/audit-logs`
- [x] Snapshot `detail` 元数据（分类结果 / 源目录 / 输出目录等）
- [x] `folders.source_dir` / `folders.relative_path` 持久化
- [x] `scan_input_dirs` 多输入扫描目录配置读取
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
- [x] 主界面升级为中文仪表盘：目录列表 / 网格切换 / 最近任务 / 最近日志 / 扫描进度
- [x] 通知系统：任务完成 / 失败 toast
- [x] SnapshotDrawer 升级为时间线展示
- [x] SettingsPage 改为多扫描目录配置（图形 DirPicker 选择器）
- [x] 前端用户可见文案统一为中文
- [x] `GET /api/fs/dirs` 目录浏览 API
- [x] Snapshot 回退 preflight 安全检查 + 结构化 RevertResult
- [x] SnapshotDrawer 回退失败详情面板
- [x] `dev.sh` 一键本地启动脚本

## 进行中

### Phase 2 - 从 scan / move job 过渡到通用工作流执行模型

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
- [x] 补齐 audit log HTTP API
- [~] 补齐 AuditLog 前端视图
  - [x] 首页最近日志面板
  - [ ] 独立日志页 / 高级过滤
- [ ] 工作流节点级输出目录配置 UI
- [ ] Job 与日志的双向跳转

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
- [~] SSE + HTTP polling fallback 完整落地
  - [x] Scan / Move 主链路 SSE 联动
  - [x] Job HTTP 轮询 fallback
  - [ ] WorkflowRun / NodeRun 级状态恢复
- [ ] `app_config` 结构化配置与迁移
- [~] SettingsPage 重构为强类型配置表单
  - [x] 多扫描目录配置
  - [ ] 工作流节点输出目录配置

## 风险与约束

- 当前仍未实现真正的 WorkflowRunner；现有 Job 覆盖 scan / move，但还没有 workflow_run / node_run
- 当前执行模型仍以 handler 内直接起 goroutine 为主，尚未切换到通用 workflow engine
- 当前配置系统仍是简单 KV + env，虽然已支持多扫描目录，但尚未迁移到 `app_config`
- 当前 Snapshot 仍是 folder/job 粒度，已补充 detail 元数据，但尚未切换到 node 粒度
- AuditLog 已开放 HTTP 查询接口，但前端仍缺少完整日志页与高级检索
- 运行时迁移以 `backend/internal/db/migrations` 为准；`backend/migrations` 目前是落后的拷贝，后续需要同步整理
