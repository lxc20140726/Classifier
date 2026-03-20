# API 设计 v3.0

> 版本：v3.0 | 日期：2026-03-20

## REST 端点

```
# Folder 管理
GET    /api/folders
POST   /api/folders/scan
GET    /api/folders/:id
PATCH  /api/folders/:id/category
PATCH  /api/folders/:id/status
DELETE /api/folders/:id                 # 软删除
POST   /api/folders/:id/restore
GET    /api/folders/deleted

# Job 管理
GET    /api/jobs
POST   /api/jobs
GET    /api/jobs/:id
GET    /api/jobs/:id/progress
POST   /api/jobs/:id/cancel

# WorkflowRun 管理
GET    /api/jobs/:job_id/workflow-runs
GET    /api/jobs/:job_id/workflow-runs/:run_id
POST   /api/workflow-runs/:id/resume
POST   /api/workflow-runs/:id/rollback

# Workflow 定义
GET    /api/workflow-defs
POST   /api/workflow-defs
GET    /api/workflow-defs/:id
PUT    /api/workflow-defs/:id
DELETE /api/workflow-defs/:id

# Node 快照
GET    /api/node-runs/:id/snapshots

# 配置
GET    /api/config
PUT    /api/config

# SSE
GET    /api/events
```

## JobProgress 响应

```json
{
  "job_id": "uuid",
  "status": "running",
  "done": 8,
  "total": 12,
  "failed": 1,
  "updated_at": "2026-03-20T11:20:00Z"
}
```

## SSE 事件

```text
event: job.progress
data: {"job_id":"uuid","done":8,"total":12,"failed":1}

event: workflow.run.progress
data: {"job_id":"uuid","workflow_run_id":"uuid","folder_id":"uuid","status":"running"}

event: workflow.node.start
data: {"job_id":"uuid","workflow_run_id":"uuid","node_id":"rename_01","node_type":"rename"}

event: workflow.node.done
data: {"job_id":"uuid","workflow_run_id":"uuid","node_id":"rename_01","elapsed_ms":120}

event: workflow.node.failed
data: {"job_id":"uuid","workflow_run_id":"uuid","node_id":"compress_01","error":"disk full"}
```

## 轮询策略

- 客户端默认建立 SSE
- SSE 断开后轮询 `GET /api/jobs/:id/progress`
- 页面首次打开和刷新时必须先轮询一次，以确保状态可恢复
