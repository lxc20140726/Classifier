# Classifier — 系统架构 v3.0

> 版本：v3.0 | 日期：2026-03-20
> **重大变更**：引入 Job-Workflow-Node 三层模型，节点粒度快照，分类器节点化，SSE+轮询混合方案

## 概述

Classifier 是一个部署在 NAS 上的媒体文件整理 Web 应用。单 Docker 容器内包含 Go 后端、React 前端静态文件和 FFmpeg，通过 bind mount 访问宿主机文件系统。

## 核心架构变更

### 三层执行模型

```
Job (用户提交的顶层任务)
 └── WorkflowRun (每个文件夹对应一个工作流执行实例)
      └── NodeRun (工作流中每个节点的执行记录)
           └── NodeSnapshot (节点执行前后的文件系统快照)
```

**关键设计决策**：
- 用户提交一次操作 → 创建一个 `Job`
- 每个文件夹执行其对应分类的工作流 → 创建 `WorkflowRun`
- 工作流中每个步骤 → 创建 `NodeRun`
- 每个节点执行前后 → 创建 `NodeSnapshot`（pre/post）
- 支持节点级断点续传和部分回退

## 系统架构图

```
┌─────────────────────┐
│                    Docker Container                  │
│              │
│  ┌──────────────┐    HTTP/SSE+Poll   ┌──────────────────────┐  │
│  │  React SPA   │ ◄───────────► │  Gin HTTP Server     │  │
│  │  (静态文件)   │           │  :8080               │  │
│  └──────────────┘                     └──────┬───────────┘  │
│                   │        │
│       ┌─────────────┤              │
│      │                      │          │
│  ┌──────▼──────┐   ┌──────────┐   ┌─────────▼────────┐  │
│  │ Workflow    │   │ SSE      │   │ Config Service        │  │
│  │ Runner      │   │ Broker   │   │ (版本化+热更新)         │  │
│  │ (DAG执行)   │   │          │   └────────────┘  │
│  └──────┬──────┘   └──────────┘                  │
│         │             │
│  ┌──────▼───────────────┐ │
│  │              Service Layer                   │ │
│  │  NodeExecutor │ Classifier │ Rename │ Compress │ Move     │ │
│  │  Snapshot │ AuditLog │ SoftDelete              │ │
│  └───────────────┬────────────────────┘ │
│             │                         │
│  ┌────────────────▼────────────────────────────┐   │
│  │        Repository Layer (SQLite)             │   │
│  │  jobs │ workflow_runs │ node_runs │ node_snapshots      │   │
│  │  folders │ workflow_definitions │ audit_logs │ config   │   │
│  └──────────────────────┬──────────────────────┘   │
│              │                  │
│  ┌──────────────────────▼─────────────────┐   │
│  │              FS Adapter Layer                │   │
│  │  /data/source (ro) │ /data/target (rw)              │   │
│  │  /data/delete_staging │ /config (named vol)             │   │
│  └───────────────────────────┘   │
└────────────────────┘
```

## 分层说明

| 层次 | 职责 | 变更说明 |
|------|------|---------|
| Transport | Gin router、SSE hub、静态文件服务 | 新增 Job 状态查询 API |
| Handler | 请求解析、参数校验、响应序列化 | 新增 Job/WorkflowRun/NodeRun handlers |
| Service | 业务逻辑 | 新增 WorkflowRunner、NodeExecutor、ConfigService |
| Repository | SQLite CRUD | 新增 Job/WorkflowRun/NodeRun/NodeSnapshot repositories |
| FS Adapter | 文件系统操作 | 新增软删除目录支持 |

## 模块依赖关系

```
Handler
  └─► Service
      ├─► Repository (SQLite)
        ├─► FS Adapter
        ├─► SSE Broker (实时推送)
        └─► WorkflowRunner (DAG 执行)

WorkflowRunner
  └─► NodeExecutor (节点执行器)
        ├─► NodeSnapshot (节点快照)
        ├─► AuditLog (审计日志)
     └─► SSE Broker (进度推送)
```

## 数据流

### 用户提交工作流任务

```
用户选择文件夹 + 点击"运行工作流"
  │
  ├─► POST /api/jobs/run-workflow
  │     └─► Handler 创建 Job 记录
  │        └─► 异步启动 WorkflowRunner
  │             │
  │                 ├─► 为每个文件夹创建 WorkflowRun
  │              │     └─► 按 DAG 拓扑排序执行节点
  │                 │         │
  │             │           ├─► 创建 NodeRun (pending)
  │             │    ├─► 创建 NodeSnapshot (pre)
  │             │           ├─► 执行节点逻辑
  │                 │           ├─► 创建 NodeSnapshot (post)
  │                 │           ├─► 更新 NodeRun (succeeded)
  │                 │           ├─► 写入 AuditLog
  │                 │           └─► 发送 SSE (job.progress)
  │                 │
  │         └─► 更新 Job 状态 (done/failed/partial)
  │
  └─► 返回 {"job_id": "uuid"}
```

### 客户端获取进度

```
客户端策略：
  1. 建立 SSE 连接 (/api/events)
  2. 监听 job.progress 事件
  3. SSE 断开时切换为 HTTP 轮询
  4. 轮询 GET /api/jobs/:id/progress
  5. SSE 重连后先 poll 一次同步状态
```

## 断点续传机制

```
WorkflowRun 失败后：
  1. 记录 last_node_id (最后成功节点)
  2. 记录 resume_node_id (下次从哪开始)
  3. 用户点击"重试"
  4. WorkflowRunner 从 resume_node_id 开始执行
  5. 跳过已成功节点（sequence < resume_seq）
  6. 继续执行后续节点
```

## 节点回退机制

```
NodeRun 失败后触发回退：
  1. 找到该 WorkflowRun 的所有已成功节点
  2. 按逆序执行补偿操作：
     - rename: 用 pre-snapshot 路径重命名回去
     - compress: 删除压缩包
     - thumbnail: 删除缩略图目录
     - move: 移回原路径
  3. 更新 NodeRun.rollback_status = succeeded
  4. 更新 WorkflowRun.status = partial
```

## 软删除机制

```
用户删除文件夹：
  1. 移动到 /data/delete_staging/{folder_name}
  2. 更新 folders.deleted_at = CURRENT_TIMESTAMP
  3. 更新 folders.delete_staging_path = 新路径
  4. 写入 audit_log (action=soft_delete)

用户恢复文件夹：
  1. 移回原路径
  2. 更新 folders.deleted_at = NULL
  3. 写入 audit_log (action=restore)

定时清理任务：
  1. 查询 deleted_at < now() - 30 days
  2. 物理删除文件
  3. 硬删除数据库记录
```

## 配置系统架构

```
配置加载链路：
  1. 代码默认值 (AppConfig struct)
  2. 数据库配置 (app_config 表)
  3. 环境变量覆盖
  4. 运行时动态更新

配置保存流程：
  1. 验证 (validator)
  2. 序列化 JSON
  3. 计算 checksum
  4. 保存到数据库
  5. 触发热更新钩子
  6. 相关服务重载配置
```

## 文档索引

| 文档 | 内容 |
|------|------|
| [TECH_STACK.md](TECH_STACK.md) | 技术栈选型与决策记录 |
| [API_V3.md](API_V3.md) | 完整 REST API + SSE 事件设计 (v3) |
| [DATA_MODELS_V3.md](DATA_MODELS_V3.md) | 数据模型与 SQLite Schema (v3) |
| [FRONTEND.md](FRONTEND.md) | 前端页面、组件树、状态管理 |
| [CLASSIFICATION_V3.md](CLASSIFICATION_V3.md) | 分类器节点化设计 (v3) |
| [WORKFLOW_V3.md](WORKFLOW_V3.md) | 节点式工作流系统设计 (v3) |
| [SNAPSHOT_V3.md](SNAPSHOT_V3.md) | 节点粒度快照与回退设计 (v3) |
| [AUDIT_LOG.md](AUDIT_LOG.md) | 审计日志系统设计 |
| [RENAME_EDITOR_V3.md](RENAME_EDITOR_V3.md) | 重命名编辑器（支持文件夹重命名）(v3) |
| [CONFIG_SYSTEM.md](CONFIG_SYSTEM.md) | 配置系统设计（版本化+热更新）(NEW) |
| [DEPLOYMENT.md](DEPLOYMENT.md) | Docker 部署配置 |
| [ROADMAP_V3.md](ROADMAP_V3.md) | 开发路线图 (v3) |
