import { Outlet } from 'react-router-dom'

import { Sidebar } from '@/components/Sidebar'

export function Layout() {
  return (
    <div className="flex min-h-screen bg-background text-foreground">
      <Sidebar />
      <main className="flex-1 overflow-auto p-6 md:p-10">
        <Outlet />
      </main>
    </div>
  )
}
