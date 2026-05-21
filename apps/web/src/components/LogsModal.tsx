import { useEffect, useState, useRef, useCallback } from 'react'
import { X, RefreshCw, Loader2, Terminal, Wifi, WifiOff } from 'lucide-react'
import { api } from '../api/client'

interface LogsModalProps {
  appId: number
  appName: string
  onClose: () => void
}

export default function LogsModal({ appId, appName, onClose }: LogsModalProps) {
  const [lines, setLines] = useState<string[]>([])
  const [loading, setLoading] = useState(true)
  const [streaming, setStreaming] = useState(false)
  const [connected, setConnected] = useState(false)
  const [tail, setTail] = useState(100)
  const bottomRef = useRef<HTMLDivElement>(null)
  const esRef = useRef<EventSource | null>(null)

  const scrollToBottom = () => setTimeout(() => bottomRef.current?.scrollIntoView({ behavior: 'smooth' }), 50)

  const loadStatic = useCallback(async (t = tail) => {
    setLoading(true)
    try {
      const r = await api.appLogs(appId, t)
      setLines(r.lines ? r.lines.split('\n').filter(Boolean) : [])
      scrollToBottom()
    } catch { setLines(['Could not load logs.']) }
    finally { setLoading(false) }
  }, [appId, tail])

  const startStream = useCallback(() => {
    esRef.current?.close()
    setLines([]); setStreaming(true); setConnected(false)
    const es = new EventSource(api.appLogsStreamUrl(appId, 50))
    esRef.current = es
    es.onopen = () => setConnected(true)
    es.onmessage = (e) => {
      const line = e.data.replace(/\\n/g, '\n')
      setLines(prev => {
        const updated = [...prev, ...line.split('\n').filter(Boolean)]
        return updated.length > 500 ? updated.slice(-500) : updated
      })
      scrollToBottom()
    }
    es.onerror = () => {
      setConnected(false)
      setTimeout(() => { if (esRef.current === es) startStream() }, 3000)
    }
  }, [appId])

  const stopStream = useCallback(() => {
    esRef.current?.close(); esRef.current = null
    setStreaming(false); setConnected(false)
  }, [])

  useEffect(() => { loadStatic(); return () => esRef.current?.close() }, [appId])

  return (
    <div className="fixed inset-0 bg-black/70 backdrop-blur-sm flex items-center justify-center z-50 p-4">
      <div className="card w-full max-w-4xl h-[80vh] flex flex-col">
        <div className="flex items-center justify-between px-5 py-3.5 border-b border-border shrink-0">
          <div className="flex items-center gap-2.5">
            <Terminal className="w-4 h-4 text-slate-400" />
            <h2 className="font-medium text-slate-200 text-sm">{appName} — logs</h2>
            {streaming && (
              <span className={`flex items-center gap-1 text-xs ${connected ? 'text-emerald-400' : 'text-amber-400'}`}>
                {connected ? <><Wifi className="w-3 h-3" /> live</> : <><WifiOff className="w-3 h-3" /> reconnecting…</>}
              </span>
            )}
          </div>
          <div className="flex items-center gap-2">
            {!streaming ? (
              <>
                <select value={tail} onChange={e => { setTail(+e.target.value); loadStatic(+e.target.value) }}
                  aria-label="Number of log lines to show"
                  className="text-xs bg-surface border border-border text-slate-300 px-2 py-1 rounded-md">
                  {[50, 100, 300, 500].map(n => <option key={n} value={n}>Last {n} lines</option>)}
                </select>
                <button type="button" onClick={() => loadStatic()} className="text-slate-400 hover:text-slate-200 p-1.5 rounded-md hover:bg-white/5" title="Refresh">
                  <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
                </button>
                <button type="button" onClick={startStream}
                  className="text-xs px-2.5 py-1.5 bg-accent/10 hover:bg-accent/20 border border-accent/20 text-accent rounded-md transition-colors flex items-center gap-1">
                  <Wifi className="w-3 h-3" /> Live
                </button>
              </>
            ) : (
              <button type="button" onClick={stopStream}
                className="text-xs px-2.5 py-1.5 bg-red-500/10 hover:bg-red-500/20 border border-red-500/20 text-red-400 rounded-md transition-colors flex items-center gap-1">
                <WifiOff className="w-3 h-3" /> Stop
              </button>
            )}
            <button type="button" onClick={onClose} title="Close logs" className="text-slate-500 hover:text-slate-300 p-1.5 rounded-md hover:bg-white/5">
              <X className="w-4 h-4" />
            </button>
          </div>
        </div>
        <div className="flex-1 overflow-y-auto bg-surface p-4 font-mono text-xs text-slate-300 leading-relaxed">
          {loading && !lines.length && (
            <div className="flex items-center gap-2 text-slate-500"><Loader2 className="w-3.5 h-3.5 animate-spin" /> Loading…</div>
          )}
          {lines.length > 0 ? (
            <pre className="whitespace-pre-wrap break-all">{lines.join('\n')}</pre>
          ) : !loading ? <p className="text-slate-600">No log output.</p> : null}
          <div ref={bottomRef} />
        </div>
      </div>
    </div>
  )
}
