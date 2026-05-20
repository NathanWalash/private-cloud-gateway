import { useEffect, useState, useRef } from 'react'
import { X, RefreshCw, Loader2, Terminal } from 'lucide-react'
import { api } from '../api/client'

interface LogsModalProps {
  appId: number
  appName: string
  onClose: () => void
}

export default function LogsModal({ appId, appName, onClose }: LogsModalProps) {
  const [lines, setLines] = useState<string>('')
  const [loading, setLoading] = useState(true)
  const [tail, setTail] = useState(150)
  const bottomRef = useRef<HTMLDivElement>(null)

  async function load(t = tail) {
    setLoading(true)
    try {
      const r = await api.appLogs(appId, t)
      setLines(r.lines)
      setTimeout(() => bottomRef.current?.scrollIntoView(), 50)
    } catch {
      setLines('Could not load logs.')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [appId])

  return (
    <div className="fixed inset-0 bg-black/70 backdrop-blur-sm flex items-center justify-center z-50 p-4">
      <div className="card w-full max-w-4xl h-[75vh] flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-3.5 border-b border-border shrink-0">
          <div className="flex items-center gap-2.5">
            <Terminal className="w-4 h-4 text-slate-400" />
            <h2 className="font-medium text-slate-200 text-sm">{appName} — logs</h2>
          </div>
          <div className="flex items-center gap-2">
            <select
              value={tail}
              onChange={e => { setTail(+e.target.value); load(+e.target.value) }}
              className="text-xs bg-surface border border-border text-slate-300 px-2 py-1 rounded-md"
            >
              {[50, 150, 300, 500].map(n => (
                <option key={n} value={n}>Last {n} lines</option>
              ))}
            </select>
            <button
              onClick={() => load()}
              className="text-slate-400 hover:text-slate-200 p-1.5 rounded-md hover:bg-white/5"
            >
              <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
            </button>
            <button onClick={onClose} className="text-slate-500 hover:text-slate-300 p-1.5 rounded-md hover:bg-white/5">
              <X className="w-4 h-4" />
            </button>
          </div>
        </div>

        {/* Log output */}
        <div className="flex-1 overflow-y-auto bg-surface p-4 font-mono text-xs text-slate-300 leading-relaxed">
          {loading && !lines && (
            <div className="flex items-center gap-2 text-slate-500">
              <Loader2 className="w-3.5 h-3.5 animate-spin" /> Loading logs…
            </div>
          )}
          {lines ? (
            <pre className="whitespace-pre-wrap break-all">{lines}</pre>
          ) : !loading ? (
            <p className="text-slate-600">No log output.</p>
          ) : null}
          <div ref={bottomRef} />
        </div>
      </div>
    </div>
  )
}
