import { useEffect, useState, useCallback } from 'react'
import {
  Archive, Download, Plus, RefreshCw, CheckCircle,
  AlertTriangle, Loader2, Upload, X,
} from 'lucide-react'
import { api, BackupFile, ApiError } from '../api/client'

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

type Status = 'ok' | 'error' | 'restored' | null

export default function BackupWidget() {
  const [backups, setBackups] = useState<BackupFile[]>([])
  const [loading, setLoading] = useState(true)
  const [creating, setCreating] = useState(false)
  const [restoring, setRestoring] = useState(false)
  const [status, setStatus] = useState<Status>(null)
  const [statusMsg, setStatusMsg] = useState('')
  const [showRestore, setShowRestore] = useState(false)

  const load = useCallback(() => {
    api.backup.list()
      .then(setBackups)
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => { load() }, [load])

  async function handleCreate() {
    setCreating(true)
    setStatus(null)
    try {
      const r = await api.backup.create()
      setStatus('ok')
      setStatusMsg(`Backed up — ${formatSize(r.size)}${r.volumes > 0 ? ` · ${r.volumes} volumes` : ''}`)
      load()
    } catch {
      setStatus('error')
      setStatusMsg('Backup failed')
    } finally {
      setCreating(false)
    }
  }

  async function handleRestore(file: File, passphrase: string) {
    setRestoring(true)
    setStatus(null)
    try {
      const r = await api.backup.restore(file, passphrase || undefined)
      setStatus('restored')
      setStatusMsg(r.message)
      setShowRestore(false)
    } catch (err) {
      setStatus('error')
      setStatusMsg(err instanceof ApiError ? err.message : 'Restore failed')
    } finally {
      setRestoring(false)
    }
  }

  const latest = backups[0]

  return (
    <>
      <div className="card p-5">
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-2">
            <Archive className="w-3.5 h-3.5 text-slate-400" />
            <h3 className="text-sm font-medium text-slate-300">Backups</h3>
          </div>
          {loading && <Loader2 className="w-3.5 h-3.5 animate-spin text-slate-600" />}
        </div>

        {/* Latest status */}
        <div className="mb-4">
          {latest ? (
            <div className="space-y-1">
              <div className="flex items-center gap-1.5 text-xs text-emerald-400">
                <CheckCircle className="w-3 h-3" /> Last backup
              </div>
              <p className="text-xs text-slate-300">{formatDate(latest.created_at)}</p>
              <p className="text-xs text-slate-500">{formatSize(latest.size)}</p>
            </div>
          ) : !loading ? (
            <div className="flex items-center gap-1.5 text-xs text-amber-400">
              <AlertTriangle className="w-3 h-3" /> No backups yet
            </div>
          ) : null}
        </div>

        {/* Status feedback */}
        {status === 'ok' && (
          <p className="text-xs text-emerald-400 mb-3 flex items-center gap-1.5">
            <CheckCircle className="w-3 h-3" /> {statusMsg}
          </p>
        )}
        {status === 'restored' && (
          <p className="text-xs text-blue-400 mb-3 flex items-center gap-1.5">
            <CheckCircle className="w-3 h-3" /> {statusMsg}
          </p>
        )}
        {status === 'error' && (
          <p className="text-xs text-red-400 mb-3 flex items-center gap-1.5">
            <AlertTriangle className="w-3 h-3" /> {statusMsg}
          </p>
        )}

        {/* Actions */}
        <div className="space-y-2">
          <button
            onClick={handleCreate}
            disabled={creating || restoring}
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

          <button
            onClick={() => setShowRestore(true)}
            disabled={creating || restoring}
            className="w-full flex items-center justify-center gap-1.5 text-xs font-medium
                       bg-white/5 hover:bg-white/10 border border-border hover:border-slate-600
                       text-slate-400 px-3 py-2 rounded-lg transition-colors disabled:opacity-50"
          >
            <Upload className="w-3.5 h-3.5" />
            Restore from backup
          </button>

          {backups.length > 0 && (
            <details className="group">
              <summary className="flex items-center gap-1.5 text-xs text-slate-500 cursor-pointer hover:text-slate-400 transition-colors list-none">
                <RefreshCw className="w-3 h-3" />
                {backups.length} backup{backups.length !== 1 ? 's' : ''} stored
              </summary>
              <div className="mt-2 space-y-1 max-h-28 overflow-y-auto">
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

      {/* Restore dialog */}
      {showRestore && (
        <RestoreDialog
          onClose={() => setShowRestore(false)}
          onRestore={handleRestore}
          loading={restoring}
        />
      )}
    </>
  )
}

function RestoreDialog({
  onClose, onRestore, loading,
}: {
  onClose: () => void
  onRestore: (file: File, passphrase: string) => void
  loading: boolean
}) {
  const [file, setFile] = useState<File | null>(null)
  const [passphrase, setPassphrase] = useState('')
  const [confirmed, setConfirmed] = useState(false)

  return (
    <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50 p-4">
      <div className="card w-full max-w-md">
        <div className="flex items-center justify-between p-5 border-b border-border">
          <h2 className="font-semibold text-slate-100 flex items-center gap-2">
            <Upload className="w-4 h-4 text-slate-400" />
            Restore from backup
          </h2>
          <button onClick={onClose} className="text-slate-500 hover:text-slate-300 p-1 rounded hover:bg-white/5">
            <X className="w-4 h-4" />
          </button>
        </div>

        <div className="p-5 space-y-4">
          <div className="bg-amber-500/10 border border-amber-500/20 text-amber-400 text-xs px-3 py-2.5 rounded-lg flex items-start gap-2">
            <AlertTriangle className="w-4 h-4 shrink-0 mt-0.5" />
            This will overwrite your current database and blueprints. The service must be restarted after restore.
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-400 mb-1.5">Backup file (.pcg-backup)</label>
            <input
              type="file"
              accept=".pcg-backup"
              onChange={e => setFile(e.target.files?.[0] ?? null)}
              className="w-full text-xs text-slate-400 file:mr-3 file:py-1.5 file:px-3 file:rounded-lg
                         file:border file:border-border file:bg-card file:text-slate-300
                         file:text-xs file:cursor-pointer hover:file:bg-white/10 file:transition-colors"
            />
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-400 mb-1.5">
              Passphrase <span className="text-slate-600">(leave empty if unencrypted)</span>
            </label>
            <input
              type="password"
              className="input-field"
              placeholder="Backup passphrase"
              value={passphrase}
              onChange={e => setPassphrase(e.target.value)}
            />
          </div>

          <label className="flex items-start gap-2.5 cursor-pointer">
            <input
              type="checkbox"
              checked={confirmed}
              onChange={e => setConfirmed(e.target.checked)}
              className="mt-0.5"
            />
            <span className="text-xs text-slate-400">
              I understand this will overwrite my current data
            </span>
          </label>
        </div>

        <div className="flex items-center justify-end gap-3 p-5 border-t border-border">
          <button onClick={onClose} className="text-sm text-slate-400 hover:text-slate-200 px-4 py-2 transition-colors">
            Cancel
          </button>
          <button
            onClick={() => file && onRestore(file, passphrase)}
            disabled={!file || !confirmed || loading}
            className="btn-primary !w-auto px-5 text-sm bg-amber-600 hover:bg-amber-700 disabled:opacity-50"
          >
            {loading
              ? <span className="flex items-center gap-2"><Loader2 className="w-3.5 h-3.5 animate-spin" />Restoring…</span>
              : 'Restore'}
          </button>
        </div>
      </div>
    </div>
  )
}
