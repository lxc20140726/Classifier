import { useEffect } from 'react'
import { AlertCircle, CheckCircle, Info, X } from 'lucide-react'

import { cn } from '@/lib/utils'
import { useNotificationStore, type AppNotification } from '@/store/notificationStore'

const LEVEL_CONFIG = {
  success: {
    icon: CheckCircle,
    bgClass: 'bg-green-50 border-green-200',
    iconClass: 'text-green-600',
    textClass: 'text-green-900',
  },
  error: {
    icon: AlertCircle,
    bgClass: 'bg-red-50 border-red-200',
    iconClass: 'text-red-600',
    textClass: 'text-red-900',
  },
  info: {
    icon: Info,
    bgClass: 'bg-blue-50 border-blue-200',
    iconClass: 'text-blue-600',
    textClass: 'text-blue-900',
  },
}

function Toast({ notification }: { notification: AppNotification }) {
  const dismissNotification = useNotificationStore((store) => store.dismissNotification)
  const config = LEVEL_CONFIG[notification.level]
  const Icon = config.icon

  useEffect(() => {
    const timer = window.setTimeout(() => {
      dismissNotification(notification.id)
    }, 6000)

    return () => window.clearTimeout(timer)
  }, [dismissNotification, notification.id])

  return (
    <div
    className={cn(
        'pointer-events-auto flex w-full max-w-sm items-start gap-3 rounded-lg border p-4 shadow-lg',
        config.bgClass,
      )}
    >
      <Icon className={cn('mt-0.5 h-5 w-5 shrink-0', config.iconClass)} />
      <div className="flex-1 space-y-1">
        <p className={cn('text-sm font-semibold', config.textClass)}>{notification.title}</p>
      <p className={cn('text-sm', config.textClass)}>{notification.message}</p>
      </div>
      <button
        type="button"
        onClick={() => dismissNotification(notification.id)}
        className={cn('shrink-0 rounded p-1 hover:bg-black/5', config.textClass)}
        aria-label="关闭通知"
      >
        <X className="h-4 w-4" />
      </button>
    </div>
  )
}

export function ToastList() {
  const notifications = useNotificationStore((store) => store.notifications)

  if (notifications.length === 0) {
    return null
  }

  return (
    <div className="pointer-events-none fixed right-4 top-4 z-50 flex flex-col gap-3">
      {notifications.map((notification) => (
        <Toast key={notification.id} notification={notification} />
      ))}
    </div>
  )
}
