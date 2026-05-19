import { useEffect, useState, useCallback } from 'react'
import { Archive, Download, Plus, RefreshCw, CheckCircle, AlertTriangle, Loader2 } from 'lucide-react'
import { api, BackupFile } from '../api/client'

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleString(undefined, {
    month: 'short', day: 'numeric',
    hour: '2-digit', minute: '2-digit',
  })
}

export default function BackupWidget() {
  const [backups, setBackups] = useState<BackupFile[]>([])
  const [loading, setLoading] = useState(true)
  const [creating, setCreating] = useState(false)
  const [lastResult, setLastResult] = useState<'ok' | 'error' | null>(null)

  const load = useCallback(() => {
    api.backup.list()
      .then(setBackups)
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => { load() }, [load])

  async function handleCreate() {
    setCreating(true)
    setLastResult(null)
    try {
      await api.backup.create()
      setLastResult('ok')
      load()
    } catch {
      setLastResult('error')
    } finally {
      setCreating(false)
    }
  }

  const latest = backups[0]

  return (
    <div className="card p-5">
      {/* Header */}
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <Archive className="w-3.5 h-3.5 text-slate-400" />
          <h3 className="text-sm font-medium text-slate-300">Backups</h3>
        </div>
        {loading && <Loader2 className="w-3.5 h-3.5 animate-spin text-slate-600" />}
      </div>

      {/* Latest backup status */}
      <div className="mb-4">
        {latest ? (
          <div className="space-y-1.5">
            <div className="flex items-center gap-1.5 text-xs text-emerald-400">
              <CheckCircle className="w-3 h-3" />
              Last backup
            </div>
            <p className="text-xs text-slate-300">{formatDate(latest.created_at)}</p>
            <p className="text-xs text-slate-500">{formatSize(latest.size)}</p>
          </div>
        ) : (
          <div className="flex items-center gap-1.5 text-xs text-amber-400">
            <AlertTriangle className="w-3 h-3" />
            No backups yet
          </div>
        )}
      </div>

      {/* Feedback */}
      {lastResult === 'ok' && (
        <p className="text-xs text-emerald-400 mb-3 flex items-center gap-1.5">
          <CheckCircle className="w-3 h-3" /> Backup created
        </p>
      )}
      {lastResult === 'error' && (
        <p className="text-xs text-red-400 mb-3 flex items-center gap-1.5">
          <AlertTriangle className="w-3 h-3" /> Backup failed
        </p>
      )}

      {/* Actions */}
      <div className="space-y-2">
        <button
          onClick={handleCreate}
          disabled={creating}
          className="w-full flex items-center justify-center gap-1.5 text-xs font-medium
                     bg-white/5 hover:bg-white/10 border border-border hover:border-slate-600
                     text-slate-300 px-3 py-2 rounded-lg transition-colors disabled:opacity-50"
        >
          {creating
            ? <><Loader2 className="w-3.5 h-3.5 animate-spin" /> Creating…</>
            : <><Plus className="w-3.5 h-3.5" /> Backup now</>
          }
        </button>

        <a
          href={api.backup.safeEscapeUrl}
          download
          className="w-full flex items-center justify-center gap-1.5 text-xs font-medium
                     bg-accent/10 hover:bg-accent/20 border border-accent/20
                     text-accent px-3 py-2 rounded-lg transition-colors"
        >
          <Download className="w-3.5 h-3.5" />
          Safe Escape download
        </a>

        {backups.length > 0 && (
          <details className="group">
            <summary className="flex items-center gap-1.5 text-xs text-slate-500 cursor-pointer hover:text-slate-400 transition-colors list-none">
              <RefreshCw className="w-3 h-3" />
              {backups.length} backup{backups.length !== 1 ? 's' : ''} stored
            </summary>
            <div className="mt-2 space-y-1 max-h-32 overflow-y-auto">
              {backups.map(b => (
                <div key={b.name} className="flex items-center justify-between text-xs py-1 border-t border-border first:border-0">
                  <span className="text-slate-500 truncate mr-2">{formatDate(b.created_at)}</span>
                  <span className="text-slate-600 shrink-0">{formatSize(b.size)}</span>
                </div>
              ))}
            </div>
          </details>
        )}
      </div>
    </div>
  )
}
