# 前端设计 v3.0

> 版本：v3.0 | 日期：2026-03-20

## 页面

- `/` FolderListPage
- `/jobs` JobsPage
- `/workflows` WorkflowDefsPage
- `/settings` SettingsPage
- `/deleted` DeletedFoldersPage

## JobsPage

- 顶层展示 Job 列表
- 每个 Job 可展开查看 WorkflowRun 列表
- 每个 WorkflowRun 可展开查看 NodeRun 时间线
- 状态来源：SSE 实时更新 + HTTP 轮询 fallback

## WorkflowsPage

- 左侧：Workflow 定义列表
- 中间：React Flow 画布
- 右侧：节点配置面板
- 节点面板新增分类器节点

## 状态管理

```typescript
interface JobStore {
  jobs: Job[]
  progressById: Record<string, JobProgress>
  fetchJobs(): Promise<void>
  fetchJob(jobId: string): Promise<void>
  pollJobProgress(jobId: string): Promise<void>
  updateJobProgress(jobId: string, progress: JobProgress): void
}
```

## SSE + Poll Fallback

- 应用启动连接 SSE
- 监听 `job.progress` / `workflow.run.progress` / `workflow.node.*`
- 若 SSE 出错，切换到轮询模式
- SSE 恢复后重新同步一次当前 Job 状态
