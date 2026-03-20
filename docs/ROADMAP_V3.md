# 开发路线图 v3.0

> 版本：v3.0 | 日期：2026-03-20

## Phase 1 - 执行模型落地

- [~] 新增 `jobs` / `workflow_runs` / `node_runs` / `node_snapshots`
  - [x] `jobs`
  - [ ] `workflow_runs`
  - [ ] `node_runs`
  - [ ] `node_snapshots`
- [~] 新增 `workflow_definitions` / `app_config`
  - [ ] `workflow_definitions`
  - [ ] `app_config`
- [x] 实现 Job 查询与进度轮询接口
- [x] 实现 Folder 软删除与恢复

## Phase 2 - 工作流引擎

- [ ] 实现 WorkflowRunner
- [ ] 实现 NodeExecutor 注册机制
- [ ] 实现 `rename` / `compress` / `thumbnail` / `move` 节点
- [ ] 实现节点级 snapshot 与 rollback
- [ ] 实现 WorkflowRun resume

## Phase 3 - 分类器节点化

- [ ] 实现 `name-keyword-classifier`
- [ ] 实现 `file-tree-classifier`
- [ ] 实现 `ext-ratio-classifier`
- [ ] 实现 `manual-classifier`

## Phase 4 - 前端与配置系统

- [ ] JobsPage 三层展开视图
- [ ] WorkflowDefsPage 节点编辑器升级
- [ ] SettingsPage 重构为结构化配置编辑
- [ ] SSE + Poll fallback 上线
