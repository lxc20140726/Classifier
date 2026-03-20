# Classifier — 系统架构

> 更新说明：若查看最新的 Job-Workflow-Node 三层设计、节点级快照、SSE+轮询混合方案，请优先阅读 [ARCHITECTURE_V3.md](ARCHITECTURE_V3.md)。

> 版本：v2.0 | 日期：2026-03-19

## 概述

Classifier 是一个部署在 NAS 上的媒体文件整理 Web 应用。单 Docker 容器内包含 Go 后端、React 前端静态文件和 FFmpeg，通过 bind mount 访问宿主机文件系统。

## 系统架构图

```
┌─────────────────────────────────────────────────────────┐
│                    Docker Container                      │
│                                                          │
│  ┌──────────────┐    HTTP/SSE    ┌──────────────────┐   │
│  │  React SPA   │ ◄────────────► │  Gin HTTP Server  │   │
│  │  (静态文件)   │                │  :8080            │   │
│  └──────────────┘                └────────┬─────────┘   │
│                                           │              │
│         ┌─────────────────────────────────┤              │
│         │                                 │              │
│  ┌──────▼──────┐   ┌──────────┐  ┌───────▼──────────┐  │
│  │ Job         │   │ SSE      │  │ Scanner Service   │  │
│  │ Scheduler   │   │ Broker   │  │ (分类算法)        │  │
│  │ (有界并发)  │   │          │  └───────────────────┘  │
│  └──────┬──────┘   └──────────┘                         │
│         │                                               │
│  ┌──────▼────────────────────────────────────────────┐  │
│  │                  Service Layer                     │  │
│  │  Rename │ Compress │ Thumbnail │ Move │ Workflow   │  │
│  │  Snapshot │ AuditLog │ Classifier                  │  │
│  └──────────────────────┬─────────────────────────────┘  │
│                         │                               │
│  ┌──────────────────────▼──────────────────────────┐   │
│  │           Repository Layer (SQLite)              │   │
│  │  folders │ jobs │ workflows │ snapshots │ logs   │   │
│  └──────────────────────┬──────────────────────────┘   │
│                         │                               │
│  ┌──────────────────────▼──────────────────────────┐   │
│  │              FS Adapter Layer                    │   │
│  │  /data/source (ro) │ /data/target (rw)          │   │
│  │  /tmp/work (tmpfs) │ /config (named vol)        │   │
│  └─────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────┘
```

## 分层说明

| 层次 | 职责 |
|------|------|
| Transport | Gin router、SSE hub、静态文件服务 |
| Handler | 请求解析、参数校验、响应序列化 |
| Service | 业务逻辑（分类、重命名、压缩、缩略图、移动、工作流、快照、日志） |
| Scheduler | Job 队列、并发控制（semaphore 有界 goroutine） |
| Repository | SQLite CRUD，屏蔽 SQL 细节 |
| FS Adapter | 所有文件系统操作集中此层，便于 mock 测试 |

## 模块依赖关系

```
Handler
  └─► Service
        ├─► Repository (SQLite)
        ├─► FS Adapter
        ├─► SSE Broker (push 进度)
        └─► Scheduler (异步任务)

Scheduler
  └─► Worker Pool
        └─► Service (执行具体操作)
              └─► AuditLog (写入每次操作结果)
              └─► Snapshot (操作前保存快照)
```

## 数据流

```
用户操作
  ├─► REST API (同步)
  │     └─► Handler → Service → Repository → 响应
  │
  └─► 批量任务 (异步)
        └─► Handler → Scheduler → Job Queue
              └─► Worker → Service → FS Adapter
                    ├─► Snapshot (操作前)
                    ├─► 实际文件操作
                    ├─► Snapshot.After (操作后)
                    ├─► AuditLog (记录结果)
                    └─► SSE Broker (推送进度)
```

## 文档索引

| 文档 | 内容 |
|------|------|
| [TECH_STACK.md](TECH_STACK.md) | 技术栈选型与决策记录 |
| [API.md](API.md) | 完整 REST API + SSE 事件设计 |
| [DATA_MODELS.md](DATA_MODELS.md) | 数据模型与 SQLite Schema |
| [FRONTEND.md](FRONTEND.md) | 前端页面、组件树、状态管理 |
| [CLASSIFICATION.md](CLASSIFICATION.md) | 文件分类算法 |
| [WORKFLOW.md](WORKFLOW.md) | 节点式工作流系统设计 |
| [SNAPSHOT.md](SNAPSHOT.md) | 可回退操作设计 |
| [AUDIT_LOG.md](AUDIT_LOG.md) | 审计日志系统设计 |
| [RENAME_EDITOR.md](RENAME_EDITOR.md) | 可视化重命名编辑器 |
| [DEPLOYMENT.md](DEPLOYMENT.md) | Docker 部署配置 |
| [ROADMAP.md](ROADMAP.md) | 开发路线图 |
