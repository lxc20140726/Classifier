# 开发路线图

> 版本：v3.5 | 日期：2026-03-23
> 当前实现以仓库实际代码为准，v3 目标设计见 [ROADMAP_V3.md](ROADMAP_V3.md)。本文件只跟踪当前仓库里的「已完成 / 进行中 / 下一步」。

---

## 已完成

### Phase 1 — MVP 基础能力

- [x] Go + Gin + SQLite 基础工程（`CGO_ENABLED=0`，`modernc.org/sqlite`）
- [x] Folder 扫描、分类、列表、详情、分类/状态修改
- [x] Move 基础链路（创建任务、异步执行、落盘进度、SSE 通知）
- [x] Snapshot 创建、查询、回滚
- [x] AuditLog 后端持久化
- [x] Config 读取与保存接口
- [x] 前端 FolderList / Settings / SnapshotDrawer
- [x] Docker / docker-compose / NAS（极空间）部署资产

### Phase 1.5 — 扫描 / 历史 / 主界面增强

后端：

- [x] jobs 表与 JobRepository
- [x] scan 任务类型：扫描动作持久化为 Job，逐目录触发分类，不等全部完成
- [x] POST /api/jobs/move
- [x] GET /api/jobs / GET /api/jobs/:id / GET /api/jobs/:id/progress
- [x] MoveService 持久化 Job 状态与进度
- [x] ScannerService：逐目录扫描、逐目录落库、逐目录立即分类
- [x] scan.started / scan.progress / scan.error / scan.done SSE 事件
- [x] Folder 软删除 + POST /api/folders/:id/restore
- [x] folders 查询默认过滤已删除记录
- [x] GET /api/audit-logs（支持 job_id / folder_id / action / result 过滤）
- [x] Snapshot detail 元数据（分类结果 / 源目录 / 相对路径）
- [x] folders.source_dir / folders.relative_path 持久化（迁移 004）
- [x] scan_input_dirs 多输入扫描目录配置读取
- [x] GET /api/fs/dirs：图形目录浏览 API（路径校验 + 过滤隐藏目录）
- [x] SnapshotService.Revert preflight 安全检查：移动前验证路径状态，失败返回 RevertResult
- [x] POST /api/snapshots/:id/revert 返回结构化 revert_result（含 preflight 错误和当前状态）

前端：

- [x] src/api/jobs.ts：listJobs / getJob / getJobProgress
- [x] src/store/jobStore.ts：Zustand Job 状态管理
- [x] src/pages/JobsPage.tsx：Job 列表、状态 badge、进度条、可展开详情
- [x] FolderListPage：多选 + Move 弹窗 + 软删除 + Restore 按钮
- [x] useSSE.ts：处理 scan.* / job.progress / job.done / job.error 事件
- [x] 主界面升级为中文仪表盘：目录网格/列表切换 / 最近任务 / 最近日志 / 扫描进度条
- [x] 通知系统（ToastList）：任务完成 / 失败 toast，6 秒自动消失
- [x] SnapshotDrawer 升级为时间线展示，显示 detail 元数据
- [x] SnapshotDrawer 回退失败详情面板（preflight 原因 + 当前文件位置）
- [x] SettingsPage 改为多扫描目录配置（图形 DirPicker 选择器）
- [x] DirPicker Modal：目录树实时浏览、可滚动列表、手动路径输入
- [x] 前端用户可见文案统一为中文
- [x] dev.sh 一键本地启动脚本（自动初始化目录、等待健康检查、Ctrl+C 同时停止）

---

## 下一步（Phase 2）— 工作流引擎核心

> 目标：让「扫描 -> 分类 -> 处理」链路由可配置的工作流节点驱动，而不是硬编码在 service 里。

### 2A — 后端执行模型（优先）

- [ ] 新增迁移：workflow_definitions / workflow_runs / node_runs / node_snapshots 四张表
- [ ] 实现 WorkflowDefinitionRepository 和对应 handler（CRUD）
- [ ] 实现 WorkflowRunRepository / NodeRunRepository
- [ ] 实现 WorkflowRunner 接口与调度循环（串/并行节点执行）
- [ ] 实现 NodeExecutor 注册机制（按 type 字符串分发）
- [ ] 内置分类节点：ext-ratio-classifier（现有算法迁移为节点）
- [ ] 内置动作节点：move（现有 MoveService 封装为节点）
- [ ] 节点级 pre/post snapshot（node_snapshots 表）
- [ ] WorkflowRun 失败时记录 resume_node_id，支持断点续传

### 2B — 后端 API 扩展

- [ ] POST /api/jobs：通用 Job 创建（指定 workflow_def_id + folder_ids）
- [ ] GET /api/jobs/:id/workflow-runs：Job 下的 WorkflowRun 列表
- [ ] GET /api/workflow-runs/:id：WorkflowRun 详情（含 node_runs）
- [ ] POST /api/workflow-runs/:id/resume：从断点继续
- [ ] POST /api/workflow-runs/:id/rollback：回滚整个 WorkflowRun
- [ ] GET /api/workflow-defs / POST / PUT / DELETE：工作流定义 CRUD
- [ ] SSE 新增 workflow_run.node_started / workflow_run.node_done / workflow_run.node_failed 事件

### 2C — 前端适配

- [ ] JobsPage 三层展开：Job -> WorkflowRun -> NodeRun 时间线
- [ ] WorkflowRun 详情面板：节点状态 / 耗时 / snapshot diff
- [ ] 局部回退按钮：对 WorkflowRun 提供「回退到初始状态」
- [ ] 断点续传按钮：failed/partial WorkflowRun 显示「从断点继续」
- [ ] SSE 接入 workflow_run.* 事件，页面刷新后用 HTTP 拉取补齐

---

## Phase 3 — 分类器节点化

- [ ] name-keyword-classifier 节点（基于文件夹名关键词规则）
- [ ] file-tree-classifier 节点（基于文件树结构判断）
- [ ] manual-classifier 节点（暂停流程，等待用户确认）
- [ ] 分类结果输出 category + confidence + reason，写入 audit_logs.detail
- [ ] 默认工作流：name-keyword-classifier -> file-tree-classifier -> ext-ratio-classifier -> manual-classifier

---

## Phase 4 — 工作流编辑器 & 配置系统

- [ ] 工作流节点编辑器（可视化图形编辑，左侧节点面板 + 右侧属性面板）
- [ ] 工作流节点输出目录配置（move 节点内配置，支持同一 Job 内不同目录走不同输出路径）
- [ ] app_config 结构化配置迁移（替换现有 KV config 表）
- [ ] SettingsPage 补全工作流节点输出目录配置入口
- [ ] 完整审计日志前端页（高级检索、时间范围、分页）

---

## Phase 5 — 扩展能力（规划）

- [ ] rename 节点（模板化重命名，支持 Emby 命名规则）
- [ ] compress 节点（zip/cbz 打包）
- [ ] thumbnail 节点（FFmpeg 截帧，Emby 封面生成）
- [ ] 调度器：定时触发扫描任务
- [ ] 认证/授权基础（NAS 多用户场景）

---

## 风险与约束

- WorkflowRunner / NodeExecutor 尚未实现；现有 scan / move Job 仍通过 handler 内直接起 goroutine 执行
- 现有 Snapshot 是 folder/job 粒度，Phase 2A 完成后需迁移到 node_snapshot 粒度
- Config 系统仍为简单 KV；app_config 迁移放在 Phase 4
- 运行时迁移以 backend/internal/db/migrations/ 为准；backend/migrations/ 是落后拷贝，下次清理
- AuditLog 前端只有最近 20 条预览，完整检索页放在 Phase 4
