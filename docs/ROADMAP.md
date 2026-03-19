# 开发路线图

> 版本：v2.0 | 日期：2026-03-19

## Phase 1 — MVP（核心扫描 + 分类 + 移动 + 安全基础）

目标：能扫描目录、自动分类、手动修正、移动文件夹，建立安全操作基础

**后端：**
- [ ] 后端项目初始化（Go module、Gin、SQLite）
- [ ] Docker 多阶段构建配置
- [ ] 数据库 schema + migration（folders / snapshots / audit_logs）
- [ ] Scanner Service：递归扫描 /data/source
- [ ] Classifier Service：扩展名 + 比例判断分类
- [ ] Folder CRUD API（GET/PATCH/DELETE）
- [ ] SSE Broker 基础实现
- [ ] Move Service：移动文件夹到 /data/target
- [ ] Snapshot Service：移动前自动创建快照
- [ ] Snapshot API（GET + POST revert）
- [ ] AuditLog Service：写入扫描、分类、移动操作
- [ ] 健康检查端点（/health）

**前端：**
- [ ] 前端项目初始化（Vite + React + TypeScript + Tailwind + shadcn/ui）
- [ ] FolderListPage：列表、筛选、分类修正
- [ ] SettingsPage：source/target 目录配置
- [ ] SSE hook：连接 /api/events，更新 Zustand store
- [ ] SnapshotDrawer：查看操作快照、一键回退

**验收标准：** 能扫描 → 看到分类结果 → 手动修正 → 移动到目标目录 → 出错可回退

---

## Phase 2 — 批量操作 + 节点式工作流

目标：完整批量处理能力 + 可视化节点工作流配置

**后端：**
- [ ] Job Scheduler：队列 + 有界并发 goroutine pool
- [ ] Rename Service：Token 模板解析 + 批量重命名
- [ ] Token 系统：{index} {date} {filename} {foldername} {ext} 等
- [ ] 预设重命名规则（4 个内置模板）
- [ ] Compress Service：archive/zip 快速压缩图片目录
- [ ] Thumbnail Service：FFmpeg 生成 Emby 规范缩略图
- [ ] Job API（POST/GET/DELETE）
- [ ] RenameRule API（CRUD + preview）
- [ ] Workflow Service：DAG 执行引擎（拓扑排序 + 节点执行）
- [ ] Workflow API（CRUD + validate + run）
- [ ] Snapshot：重命名/移动前自动快照
- [ ] AuditLog：写入重命名、压缩、缩略图操作

**前端：**
- [ ] JobsPage：任务列表 + 实时进度条
- [ ] RulesPage：Token 重命名编辑器（拖拽 Token、实时预览）
- [ ] WorkflowsPage：React Flow 节点图编辑器
- [ ] LogsPage：审计日志列表 + 筛选 + CSV 导出
- [ ] FolderToolbar：批量操作按钮

**验收标准：** 能配置节点工作流 → 批量重命名（预览正确）→ 压缩图片目录 → 生成 Emby 缩略图 → 审计日志可查

---

## Phase 3 — 并发优化 + 体验打磨

目标：生产可用，流畅体验，支持大规模处理

**性能：**
- [ ] 并发扫描（多目录同时扫描）
- [ ] 扫描增量更新（只扫描新增/变更文件夹）
- [ ] Magic bytes 检测（处理无扩展名文件）
- [ ] 前端虚拟列表（react-window）
- [ ] 任务取消（cancel running job）
- [ ] 错误重试机制（失败 Job 自动重试 3 次）

**工作流增强：**
- [ ] 条件节点（如：仅当文件数 > N 时执行压缩）
- [ ] 工作流执行历史记录
- [ ] 自定义分类规则（用户扩展扩展名映射）

**运维：**
- [ ] 审计日志自动清理（保留最近 N 天）
- [ ] 配置导入/导出（JSON）
- [ ] 快照自动过期清理（保留最近 30 天）

**验收标准：** 处理 1000+ 文件夹流畅，任务可取消，审计日志完整
