import { BrowserRouter, Route, Routes } from 'react-router-dom'

import { Layout } from '@/components/Layout'
import { useSSE } from '@/hooks/useSSE'
import { useThemeStore } from '@/store/themeStore'
import AuditLogsPage from '@/pages/AuditLogsPage'
import FolderListPage from '@/pages/FolderListPage'
import JobsPage from '@/pages/JobsPage'
import NotFoundPage from '@/pages/NotFoundPage'
import SettingsPage from '@/pages/SettingsPage'
import WorkflowEditorPage from '@/pages/WorkflowEditorPage'
import WorkflowDefsPage from '@/pages/WorkflowDefsPage'

export default function App() {
  useSSE()
  useThemeStore()

  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Layout />}>
          <Route index element={<FolderListPage />} />
          <Route path="audit-logs" element={<AuditLogsPage />} />
          <Route path="jobs" element={<JobsPage />} />
          <Route path="settings" element={<SettingsPage />} />
          <Route path="workflow-defs" element={<WorkflowDefsPage />} />
          <Route path="*" element={<NotFoundPage />} />
        </Route>
        <Route path="/workflow-defs/:id/editor" element={<WorkflowEditorPage />} />
       </Routes>
    </BrowserRouter>
  )
}
