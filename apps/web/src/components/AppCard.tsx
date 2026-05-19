import { useState } from 'react'
import { ExternalLink, Play, Square, RotateCcw, Trash2, Loader2, Circle } from 'lucide-react'
import { App, api } from '../api/client'

const statusConfig = {
  running: { label: 'Running', color: 'text-emerald-400', dot: 'bg-emerald-400' },
  stopped: { label: 'Stopped', color: 'text-slate-500', dot: 'bg-slate-600' },
  starting: { label: 'Starting', color: 'text-amber-400', dot: 'bg-amber-400 animate-pulse' },
  missing: { label: 'Missing', color: 'text-red-400', dot: 'bg-red-400' },
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
    try { await fn(); onStatusChange() }
    catch { /* parent will refresh */ }
    finally { setBusy(false) }
  }

  return (
    <div className="card p-5 flex flex-col gap-3 hover:border-slate-600 transition-colors duration-150">
      <div className="flex items-start justify-between">
        <div className="w-10 h-10 bg-accent/10 border border-accent/20 rounded-lg flex items-center justify-center">
          <Circle className="w-5 h-5 text-accent/60" strokeWidth={1.5} />
        </div>
        <span className={`flex items-center gap-1.5 text-xs font-medium ${status.color}`}>
          <span className={`w-1.5 h-1.5 rounded-full ${status.dot}`} />
          {status.label}
        </span>
      </div>

      <div>
        <h3 className="font-medium text-slate-100">{app.name}</h3>
        <a
          href={app.url} target="_blank" rel="noopener noreferrer"
          className="text-xs text-slate-500 hover:text-accent transition-colors flex items-center gap-1 mt-0.5 truncate"
        >
          {app.url}
          <ExternalLink className="w-3 h-3 shrink-0" />
        </a>
      </div>

      <div className="flex items-center gap-1 pt-1 border-t border-border">
        {busy
          ? <Loader2 className="w-3.5 h-3.5 animate-spin text-slate-500 mx-1" />
          : app.status === 'running'
            ? <>
                <CtrlBtn icon={<Square className="w-3 h-3" />} label="Stop" onClick={() => action(() => api.stopApp(app.id))} className="text-slate-400 hover:text-slate-200 hover:bg-white/5" />
                <CtrlBtn icon={<RotateCcw className="w-3 h-3" />} label="Restart" onClick={() => action(() => api.restartApp(app.id))} className="text-slate-400 hover:text-slate-200 hover:bg-white/5" />
              </>
            : <CtrlBtn icon={<Play className="w-3 h-3" />} label="Start" onClick={() => action(() => api.startApp(app.id))} className="text-emerald-400 hover:text-emerald-300 hover:bg-emerald-500/10" />
        }
        <CtrlBtn
          icon={<Trash2 className="w-3 h-3" />} label="Remove"
          onClick={() => action(() => api.uninstallApp(app.id))}
          className="ml-auto text-red-400 hover:text-red-300 hover:bg-red-500/10"
          disabled={busy}
        />
      </div>
    </div>
  )
}

function CtrlBtn({ icon, label, onClick, className, disabled = false }: {
  icon: React.ReactNode
  label: string
  onClick: () => void
  className: string
  disabled?: boolean
}) {
  return (
    <button
      onClick={onClick} disabled={disabled} title={label}
      className={`flex items-center gap-1.5 text-xs px-2 py-1 rounded-md transition-colors disabled:opacity-40 ${className}`}
    >
      {icon}{label}
    </button>
  )
}
