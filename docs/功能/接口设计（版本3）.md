# API 设计 v3.1

> 版本：v3.1 | 日期：2026-03-23
> 本文描述仓库当前**已实现**的 API。带 `[规划]` 标记的端点尚未实现。

## 已实现 REST 端点

```
# Folder 管理
GET    /api/folders                     # 列表（?status=&category=&page=&limit=&deleted=）
POST   /api/folders/scan                # 触发扫描，返回 job_id
GET    /api/folders/:id                 # 单个详情
PATCH  /api/folders/:id/category        # 手动修正分类
PATCH  /api/folders/:id/status          # 标记状态（pending/done/skip）
DELETE /api/folders/:id                 # 隐藏目录记录（不改动真实文件）
POST   /api/folders/:id/restore         # 恢复扫描（取消隐藏）

# Job 管理
GET    /api/jobs                        # 任务列表（?status=&type=&page=&limit=）
GET    /api/jobs/:id                    # 任务详情
GET    /api/jobs/:id/progress           # 实时进度轮询
POST   /api/jobs/move                   # 批量移动（创建 move job）

# 快照 / 回退
GET    /api/snapshots                   # 列表（?folder_id= 或 ?job_id=）
POST   /api/snapshots/:id/revert        # 执行回退（含 preflight 检查）

# 审计日志
GET    /api/audit-logs                  # 日志列表（?job_id=&folder_id=&action=&result=&page=&limit=）

# 配置
GET    /api/config                      # 读取全部配置 KV
PUT    /api/config                      # 批量保存配置 KV

# 文件系统目录浏览
GET    /api/fs/dirs                     # 列出目录子目录（?path=/abs/path）

# SSE
GET    /api/events                      # Server-Sent Events 流

# 健康检查
GET    /health
```

## 规划中端点（未实现）

```
GET    /api/jobs/:job_id/workflow-runs       [规划]
GET    /api/workflow-runs/:id               [规划]
POST   /api/workflow-runs/:id/rollback      [规划]
POST   /api/workflow-runs/:id/resume        [规划]
GET    /api/workflow-defs                   [规划]
POST   /api/workflow-defs                   [规划]
GET    /api/node-runs/:id/snapshots         [规划]
```

## 关键请求/响应结构

### POST /api/folders/scan

响应：
```json
{
  "started": true,
  "job_id": "9d72f5f2-a49f-42eb-a4bb-40bad22eb251",
  "source_dirs": ["/data/source", "/data/source-2"]
}
```

### POST /api/jobs/move

请求：
```json
{
  "folder_ids": ["id1", "id2"],
  "target_dir": "/data/target/video"
}
```

响应：
```json
{ "job_id": "uuid" }
```

### GET /api/jobs/:id/progress

响应：
```json
{
  "job_id": "uuid",
  "status": "running",
  "done": 8,
  "total": 12,
  "failed": 1,
  "updated_at": "2026-03-23T10:00:00Z"
}
```

### POST /api/snapshots/:id/revert

成功响应 (200)：
```json
{
  "reverted": true,
  "revert_result": {
    "ok": true,
    "current_state": [
      { "original_path": "/data/source/MyFolder", "current_path": "/data/source/MyFolder" }
    ]
  }
}
```

失败响应 (422)：
```json
{
  "error": "snapshot.Revert preflight: original path already exists",
  "revert_result": {
    "ok": false,
    "preflight_error": "目标路径 /data/source/MyFolder 已存在其他内容，回退会造成冲突，操作已取消",
    "current_state": [
      { "original_path": "/data/source/MyFolder", "current_path": "/data/target/video/MyFolder" }
    ]
  }
}
```

### GET /api/fs/dirs

请求参数：`?path=/data`

响应：
```json
{
  "path": "/data",
  "parent": "/",
  "entries": [
    { "name": "source", "path": "/data/source" },
    { "name": "target", "path": "/data/target" }
  ]
}
```

错误（400）：
```json
{ "error": "path not accessible: stat /invalid: no such file or directory" }
```

### GET /api/audit-logs

响应：
```json
{
  "data": [
    {
      "ID": "audit-scan-xxx",
      "JobID": "uuid",
      "FolderID": "xxx",
      "FolderPath": "/data/source/MyFolder",
      "Action": "scan",
      "Level": "info",
      "Detail": { "category": "video", "source_dir": "/data/source", "relative_path": "MyFolder" },
      "Result": "success",
      "ErrorMsg": "",
      "CreatedAt": "2026-03-23T10:00:00Z"
    }
  ],
  "total": 1,
  "page": 1,
  "limit": 50
}
```

### PUT /api/config

请求（KV map，值均为 string）：
```json
{
  "scan_input_dirs": "[\"/data/source\",\"/data/source-2\"]",
  "source_dir": "/data/source"
}
```

## SSE 事件

```
event: scan.started
data: {"job_id":"uuid","source_dirs":["/data/source"],"total":12}

event: scan.progress
data: {"job_id":"uuid","folder_id":"xxx","folder_name":"MyFolder","folder_path":"/data/source/MyFolder","source_dir":"/data/source","relative_path":"MyFolder","category":"video","done":3,"total":12}

event: scan.error
data: {"job_id":"uuid","folder_name":"BrokenFolder","error":"stat error","done":4,"total":12}

event: scan.done
data: {"job_id":"uuid","status":"succeeded","processed":11,"failed":1,"total":12}

event: job.progress
data: {"job_id":"uuid","done":3,"total":12,"failed":0}

event: job.done
data: {"job_id":"uuid","status":"succeeded","processed":12,"failed":0,"total":12}

event: job.error
data: {"job_id":"uuid","error":"move failed: permission denied"}
```

## 轮询策略

- 客户端默认建立 SSE 连接（`GET /api/events`）
- SSE 断开后 3 秒自动重连
- 客户端对进行中的 Job 同时轮询 `GET /api/jobs/:id/progress`（2 秒间隔）
- 前端页面刷新时先轮询一次所有 in-progress Job 以恢复状态
