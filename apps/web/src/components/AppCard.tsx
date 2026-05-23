import { useState } from 'react'
import {
  ExternalLink, Play, Square, RotateCcw, Trash2,
  Loader2, Terminal, RefreshCw, AlertTriangle, HeartPulse, ArrowUpCircle,
} from 'lucide-react'
import { App, api } from '../api/client'
import AppIcon from './AppIcon'
import LogsModal from './LogsModal'

const statusConfig = {
  running: { label: 'Running',  color: 'text-emerald-400', dot: 'bg-emerald-400' },
  stopped: { label: 'Stopped',  color: 'text-slate-500',   dot: 'bg-slate-600' },
  starting:{ label: 'Starting', color: 'text-amber-400',   dot: 'bg-amber-400 animate-pulse' },
  missing: { label: 'Missing',  color: 'text-slate-500',   dot: 'bg-slate-700' },
  error:   { label: 'Error',    color: 'text-red-400',     dot: 'bg-red-500' },
}

interface AppCardProps {
  app: App
  onStatusChange: () => void
  updateAvailable?: boolean
}

export default function AppCard({ app, onStatusChange, updateAvailable = false }: AppCardProps) {
  const [busy, setBusy] = useState(false)
  const [showLogs, setShowLogs] = useState(false)
  const status = statusConfig[app.status as keyof typeof statusConfig] ?? statusConfig.missing

  async function action(fn: () => Promise<unknown>) {
    setBusy(true)
    try { await fn(); onStatusChange() }
    catch { /* parent refreshes */ }
    finally { setBusy(false) }
  }

  return (
    <>
      <div className={`card p-5 flex flex-col gap-3 hover:border-slate-600 transition-colors duration-150 ${
        app.status === 'error' ? 'border-red-900/50 bg-red-950/10' : ''
      }`}>
        {/* Header row */}
        <div className="flex items-start justify-between">
          <div className="w-10 h-10 bg-accent/10 border border-accent/20 rounded-lg flex items-center justify-center">
            <AppIcon appId={app.blueprint_id} className="w-5 h-5 text-accent/70" />
          </div>
          <div className="flex items-center gap-2">
            {/* Update available badge */}
            {updateAvailable && (
              <span title="Update available" className="text-blue-400">
                <ArrowUpCircle className="w-3.5 h-3.5" />
              </span>
            )}
            {/* Health check indicator — only shown when running */}
            {app.status === 'running' && app.health_status !== 'unknown' && (
              <span
                title={`Health: ${app.health_status}`}
                className={`text-xs ${
                  app.health_status === 'healthy' ? 'text-emerald-400' :
                  app.health_status === 'unreachable' ? 'text-slate-500' :
                  'text-amber-400'
                }`}
              >
                <HeartPulse className="w-3 h-3" />
              </span>
            )}
            <span className={`flex items-center gap-1.5 text-xs font-medium ${status.color}`}>
              {app.status === 'error'
                ? <AlertTriangle className="w-3 h-3" />
                : <span className={`w-1.5 h-1.5 rounded-full ${status.dot}`} />
              }
              {status.label}
            </span>
          </div>
        </div>

        {/* Name + URL */}
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

        {/* Error message */}
        {app.status === 'error' && (
          <p className="text-xs text-red-400/80 bg-red-950/20 border border-red-900/30 rounded px-2 py-1.5">
            Container exited on start. Check logs for details.
          </p>
        )}

        {/* Controls */}
        <div className="flex items-center gap-1 pt-1 border-t border-border flex-wrap">
          {busy
            ? <Loader2 className="w-3.5 h-3.5 animate-spin text-slate-500 ml-1" />
            : app.status === 'running'
              ? <>
                  <CtrlBtn icon={<Square className="w-3 h-3" />} label="Stop" onClick={() => action(() => api.stopApp(app.id))} className="text-slate-400 hover:text-slate-200 hover:bg-white/5" />
                  <CtrlBtn icon={<RotateCcw className="w-3 h-3" />} label="Restart" onClick={() => action(() => api.restartApp(app.id))} className="text-slate-400 hover:text-slate-200 hover:bg-white/5" />
                </>
              : <CtrlBtn icon={<Play className="w-3 h-3" />} label="Start" onClick={() => action(() => api.startApp(app.id))} className="text-emerald-400 hover:text-emerald-300 hover:bg-emerald-500/10" />
          }
          <CtrlBtn icon={<Terminal className="w-3 h-3" />} label="Logs" onClick={() => setShowLogs(true)} className="text-slate-500 hover:text-slate-300 hover:bg-white/5" disabled={busy} />
          <CtrlBtn icon={<RefreshCw className="w-3 h-3" />} label="Update" onClick={() => action(() => api.updateApp(app.id))} className="text-slate-500 hover:text-blue-300 hover:bg-blue-500/10" disabled={busy} />
          <CtrlBtn icon={<Trash2 className="w-3 h-3" />} label="Remove" onClick={() => action(() => api.uninstallApp(app.id))} className="ml-auto text-red-400 hover:text-red-300 hover:bg-red-500/10" disabled={busy} />
        </div>
      </div>

      {showLogs && (
        <LogsModal appId={app.id} appName={app.name} onClose={() => setShowLogs(false)} />
      )}
    </>
  )
}

function CtrlBtn({ icon, label, onClick, className, disabled = false }: {
  icon: React.ReactNode; label: string; onClick: () => void; className: string; disabled?: boolean
}) {
  return (
    <button
      type="button" onClick={onClick} disabled={disabled} title={label}
      className={`flex items-center gap-1.5 text-xs px-2 py-1 rounded-md transition-colors disabled:opacity-40 ${className}`}
    >
      {icon}{label}
    </button>
  )
}
