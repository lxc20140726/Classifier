# 开发路线图

> 版本：v3.1 | 日期：2026-03-20
> 当前以 v3 架构为准，详细设计见 [ROADMAP_V3.md](ROADMAP_V3.md)。本文件用于跟踪“已完成 / 进行中 / 下一步”的实际状态。

## 已完成

### Phase 1 — MVP 基础能力

- [x] Go + Gin + SQLite 基础工程
- [x] Folder 扫描、分类、列表、状态修改
- [x] Move + Snapshot + Audit 基础链路
- [x] SSE 基础推送
- [x] 前端 FolderList / Settings / SnapshotDrawer

### Phase 1.5 — v3 基础设施已落地

- [x] 新增 `jobs` 表与 JobRepository
- [x] `POST /api/jobs/move` 返回 `job_id`，不再只是临时 `operation_id`
- [x] 新增 `GET /api/jobs`
- [x] 新增 `GET /api/jobs/:id`
- [x] 新增 `GET /api/jobs/:id/progress`
- [x] MoveService 会持久化 Job 状态与进度
- [x] Folder 删除改为软删除
- [x] 新增 `POST /api/folders/:id/restore`
- [x] `folders` 查询默认过滤已删除记录
- [x] v3 设计文档集补齐

## 进行中

### Phase 2 — 工作流执行模型

- [ ] 新增 `workflow_runs`
- [ ] 新增 `node_runs`
- [ ] 新增 `node_snapshots`
- [ ] 新增 `workflow_definitions`
- [ ] 实现 WorkflowRunner
- [ ] 将 `MoveService` 下沉为 `move` 节点执行器
- [ ] 实现 WorkflowRun resume / rollback

## 下一步

### Phase 3 — 节点能力补齐

- [ ] Rename 节点（文件 + 文件夹）
- [ ] Compress 节点
- [ ] Thumbnail 节点
- [ ] 节点级 snapshot 与补偿
- [ ] 条件节点与等待节点

### Phase 4 — 分类器节点化

- [ ] `name-keyword-classifier`
- [ ] `file-tree-classifier`
- [ ] `ext-ratio-classifier`
- [ ] `manual-classifier`

### Phase 5 — 前端与配置系统

- [ ] JobsPage 三层结构（Job / WorkflowRun / NodeRun）
- [ ] WorkflowsPage 节点编辑器
- [ ] SSE + HTTP polling fallback
- [ ] `app_config` 结构化配置与迁移
- [ ] SettingsPage 重构为强类型配置表单

## 风险与约束

- 当前仍未实现真正的 WorkflowRunner，现有 Job 仅覆盖 move 任务
- 当前配置系统仍是简单 KV + env，尚未迁移到 `app_config`
- 当前 Snapshot 仍是 operation/folder 粒度，尚未切换到 node 粒度
- 前端尚未接入新的 Job 查询接口
