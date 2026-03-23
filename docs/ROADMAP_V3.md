# 开发路线图 v3.0（规划视角）

> 版本：v3.1 | 日期：2026-03-23
> 本文件是 v3 架构的规划视图，追踪每个规划条目的落地状态。
> 当前实现进度见 [ROADMAP.md](ROADMAP.md)。

## Phase 1 — 执行模型落地

- [~] 新增持久化表
  - [x] jobs
  - [ ] workflow_runs
  - [ ] node_runs
  - [ ] node_snapshots
- [~] 新增定义表
  - [ ] workflow_definitions
  - [ ] app_config
- [x] Job 查询与进度轮询接口
- [x] Folder 软删除与恢复
- [x] 扫描动作持久化为 scan Job
- [x] 扫描链路 SSE 反馈（started / progress / error / done）
- [x] GET /api/audit-logs
- [x] snapshots.detail 元数据
- [x] GET /api/fs/dirs 目录浏览接口
- [x] Snapshot Revert preflight 安全检查 + RevertResult
- [x] 多扫描目录配置（scan_input_dirs）
- [x] DirPicker 图形目录选择器
- [x] 主界面中文仪表盘
- [x] 通知系统 ToastList

## Phase 2 — 工作流引擎

- [ ] WorkflowRunner 接口与调度循环
- [ ] NodeExecutor 注册机制（按 type 字符串分发）
- [ ] 内置节点：ext-ratio-classifier / move / name-keyword-classifier
- [ ] 节点级 pre/post snapshot（node_snapshots 表）
- [ ] WorkflowRun resume（断点续传）
- [ ] workflow_runs / node_runs 持久化
- [ ] WorkflowRun rollback
- [ ] SSE workflow_run.* 事件

## Phase 3 — 分类器节点化

- [ ] name-keyword-classifier 节点
- [ ] file-tree-classifier 节点
- [ ] ext-ratio-classifier 节点（现有算法节点化）
- [ ] manual-classifier 节点（暂停等待用户确认）
- [ ] 分类输出写入 audit_logs.detail
- [ ] 默认工作流定义

## Phase 4 — 前端工作流编辑器 & 配置系统

- [ ] JobsPage 三层展开：Job -> WorkflowRun -> NodeRun
- [ ] WorkflowDefsPage 节点图形编辑器
- [ ] move 节点输出目录配置（每目录可独立配置）
- [ ] app_config 结构化配置迁移
- [ ] 完整审计日志前端页
- [ ] WorkflowRun 局部回退与断点续传按钮

## Phase 5 — 扩展节点（规划）

- [ ] rename 节点（模板化重命名，支持 Emby 命名规则）
- [ ] compress 节点（zip/cbz）
- [ ] thumbnail 节点（FFmpeg 截帧）
- [ ] 调度器（定时触发）
- [ ] 认证/授权
