import { BrowserRouter, Route, Routes } from 'react-router-dom'

import { Layout } from '@/components/Layout'
import { useSSE } from '@/hooks/useSSE'
import FolderListPage from '@/pages/FolderListPage'
import JobsPage from '@/pages/JobsPage'
import NotFoundPage from '@/pages/NotFoundPage'
import SettingsPage from '@/pages/SettingsPage'

export default function App() {
  useSSE()

  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Layout />}>
          <Route index element={<FolderListPage />} />
          <Route path="jobs" element={<JobsPage />} />
     <Route path="settings" element={<SettingsPage />} />
          <Route path="*" element={<NotFoundPage />} />
     </Route>
      </Routes>
    </BrowserRouter>
  )
}
