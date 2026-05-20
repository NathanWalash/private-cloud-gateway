import { useEffect, useState, useCallback } from 'react'
import { Activity, Plus, Trash2, CheckCircle, AlertTriangle, Loader2, X } from 'lucide-react'
import { api } from '../api/client'

type MonitorStatus = { id: number; name: string; url: string; status: string; latency_ms: number | null; last_checked: string | null }

export default function MonitorWidget() {
  const [monitors, setMonitors] = useState<MonitorStatus[]>([])
  const [showAdd, setShowAdd] = useState(false)
  const [name, setName] = useState('')
  const [url, setUrl] = useState('')
  const [adding, setAdding] = useState(false)

  const load = useCallback(() => {
    api.monitors.list().then(setMonitors).catch(() => {})
  }, [])

  useEffect(() => {
    load()
    const id = setInterval(load, 60_000)
    return () => clearInterval(id)
  }, [load])

  async function handleAdd(e: React.FormEvent) {
    e.preventDefault()
    setAdding(true)
    try {
      await api.monitors.create(name, url)
      setName(''); setUrl(''); setShowAdd(false)
      load()
    } catch { /* ignore */ }
    finally { setAdding(false) }
  }

  async function handleDelete(id: number) {
    await api.monitors.remove(id)
    load()
  }

  return (
    <div className="card p-5">
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <Activity className="w-3.5 h-3.5 text-slate-400" />
          <h3 className="text-sm font-medium text-slate-300">API Monitors</h3>
        </div>
        <button
          onClick={() => setShowAdd(v => !v)}
          className="text-accent hover:text-accent-hover transition-colors"
        >
          {showAdd ? <X className="w-3.5 h-3.5" /> : <Plus className="w-3.5 h-3.5" />}
        </button>
      </div>

      {showAdd && (
        <form onSubmit={handleAdd} className="mb-4 space-y-2">
          <input
            placeholder="Name" required
            className="input-field text-xs py-1.5"
            value={name} onChange={e => setName(e.target.value)}
          />
          <input
            placeholder="https://api.example.com/health" type="url" required
            className="input-field text-xs py-1.5"
            value={url} onChange={e => setUrl(e.target.value)}
          />
          <button
            type="submit" disabled={adding}
            className="w-full text-xs py-1.5 bg-accent/10 hover:bg-accent/20 border border-accent/20 text-accent rounded-lg transition-colors"
          >
            {adding ? <Loader2 className="w-3 h-3 animate-spin mx-auto" /> : 'Add monitor'}
          </button>
        </form>
      )}

      {monitors.length === 0 && !showAdd && (
        <p className="text-xs text-slate-600">No monitors. Add a URL to track.</p>
      )}

      <div className="space-y-2">
        {monitors.map(m => (
          <div key={m.id} className="flex items-center gap-2 group">
            {m.status === 'up'
              ? <CheckCircle className="w-3.5 h-3.5 text-emerald-400 shrink-0" />
              : m.status === 'down'
                ? <AlertTriangle className="w-3.5 h-3.5 text-red-400 shrink-0" />
                : <div className="w-3.5 h-3.5 rounded-full bg-slate-600 shrink-0" />
            }
            <div className="flex-1 min-w-0">
              <p className="text-xs font-medium text-slate-300 truncate">{m.name}</p>
              {m.latency_ms && <p className="text-xs text-slate-600">{m.latency_ms}ms</p>}
            </div>
            <button
              onClick={() => handleDelete(m.id)}
              className="opacity-0 group-hover:opacity-100 text-slate-600 hover:text-red-400 transition-all p-0.5"
            >
              <Trash2 className="w-3 h-3" />
            </button>
          </div>
        ))}
      </div>
    </div>
  )
}
