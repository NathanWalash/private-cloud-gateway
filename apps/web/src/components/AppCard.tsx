import { useState } from 'react'
import { App, api } from '../api/client'

const statusConfig = {
  running: { label: 'Running', dot: 'bg-emerald-400', text: 'text-emerald-400' },
  stopped: { label: 'Stopped', dot: 'bg-slate-500', text: 'text-slate-500' },
  starting: { label: 'Starting', dot: 'bg-amber-400 animate-pulse', text: 'text-amber-400' },
  missing: { label: 'Missing', dot: 'bg-red-400', text: 'text-red-400' },
}

interface AppCardProps {
  app: App
  onStatusChange: () => void
}

export default function AppCard({ app, onStatusChange }: AppCardProps) {
  const [busy, setBusy] = useState(false)
  const status = statusConfig[app.status] ?? statusConfig.missing

  async function action(fn: () => Promise<unknown>) {
    setBusy(true)
    try {
      await fn()
      onStatusChange()
    } catch {
      // swallow — parent will refresh
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="card p-5 flex flex-col gap-3 hover:border-slate-600 transition-all duration-150">
      {/* Header */}
      <div className="flex items-start justify-between">
        <div className="w-10 h-10 bg-accent/10 border border-accent/20 rounded-lg flex items-center justify-center text-xl">
          {app.icon || '📦'}
        </div>
        <span className={`flex items-center gap-1.5 text-xs font-medium ${status.text}`}>
          <span className={`w-1.5 h-1.5 rounded-full ${status.dot}`} />
          {status.label}
        </span>
      </div>

      {/* Name + URL */}
      <div>
        <h3 className="font-medium text-slate-100">{app.name}</h3>
        <a
          href={app.url}
          target="_blank"
          rel="noopener noreferrer"
          className="text-xs text-slate-500 hover:text-accent transition-colors truncate block"
        >
          {app.url}
        </a>
      </div>

      {/* Controls */}
      <div className="flex gap-1.5 mt-auto pt-1 border-t border-border">
        {app.status === 'running' ? (
          <>
            <CtrlBtn
              label="Stop"
              busy={busy}
              onClick={() => action(() => api.stopApp(app.id))}
              className="text-slate-400 hover:text-slate-200 hover:bg-white/5"
            />
            <CtrlBtn
              label="Restart"
              busy={busy}
              onClick={() => action(() => api.restartApp(app.id))}
              className="text-slate-400 hover:text-slate-200 hover:bg-white/5"
            />
          </>
        ) : (
          <CtrlBtn
            label="Start"
            busy={busy}
            onClick={() => action(() => api.startApp(app.id))}
            className="text-emerald-400 hover:text-emerald-300 hover:bg-emerald-500/10"
          />
        )}
        <CtrlBtn
          label="Remove"
          busy={busy}
          onClick={() => action(() => api.uninstallApp(app.id))}
          className="ml-auto text-red-400 hover:text-red-300 hover:bg-red-500/10"
        />
      </div>
    </div>
  )
}

function CtrlBtn({
  label, busy, onClick, className,
}: {
  label: string
  busy: boolean
  onClick: () => void
  className: string
}) {
  return (
    <button
      onClick={onClick}
      disabled={busy}
      className={`text-xs px-2.5 py-1 rounded-md transition-colors disabled:opacity-40 ${className}`}
    >
      {label}
    </button>
  )
}
