import { useEffect, useState } from 'react'
import { Activity, Clock, Tag, AlertTriangle } from 'lucide-react'
import { api, ServerStatus } from '../api/client'

export default function StatusWidget() {
  const [status, setStatus] = useState<ServerStatus | null>(null)
  const [error, setError] = useState(false)

  useEffect(() => {
    api.status().then(setStatus).catch(() => setError(true))
    const id = setInterval(() => {
      api.status().then(setStatus).catch(() => {})
    }, 30_000)
    return () => clearInterval(id)
  }, [])

  return (
    <div className="card p-5">
      <div className="flex items-center gap-2 mb-4">
        <Activity className="w-3.5 h-3.5 text-emerald-400" />
        <h3 className="text-sm font-medium text-slate-300">System</h3>
      </div>

      {error && (
        <div className="flex items-center gap-2 text-xs text-red-400">
          <AlertTriangle className="w-3.5 h-3.5" />
          Could not load status
        </div>
      )}

      {status && (
        <div className="space-y-2.5">
          <Stat icon={<Activity className="w-3 h-3 text-emerald-400" />} label="Status" value="Healthy" valueClass="text-emerald-400" />
          <Stat icon={<Clock className="w-3 h-3 text-slate-500" />} label="Uptime" value={status.uptime} />
          <Stat icon={<Tag className="w-3 h-3 text-slate-500" />} label="Version" value={`v${status.version}`} />
        </div>
      )}

      {!status && !error && (
        <div className="space-y-2.5">
          {[1, 2, 3].map(i => (
            <div key={i} className="flex justify-between">
              <div className="h-3 bg-border rounded w-16 animate-pulse" />
              <div className="h-3 bg-border rounded w-20 animate-pulse" />
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

function Stat({ icon, label, value, valueClass = 'text-slate-200' }: {
  icon: React.ReactNode
  label: string
  value: string
  valueClass?: string
}) {
  return (
    <div className="flex items-center justify-between">
      <span className="flex items-center gap-1.5 text-xs text-slate-500">{icon}{label}</span>
      <span className={`text-xs font-medium ${valueClass}`}>{value}</span>
    </div>
  )
}
