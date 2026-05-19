import { useEffect, useState } from 'react'
import { api, ServerStatus } from '../api/client'

export default function StatusWidget() {
  const [status, setStatus] = useState<ServerStatus | null>(null)
  const [error, setError] = useState(false)

  useEffect(() => {
    api.status()
      .then(setStatus)
      .catch(() => setError(true))

    const id = setInterval(() => {
      api.status().then(setStatus).catch(() => {})
    }, 30_000)
    return () => clearInterval(id)
  }, [])

  return (
    <div className="card p-5">
      <div className="flex items-center gap-2 mb-4">
        <div className="w-2 h-2 rounded-full bg-emerald-400" />
        <h3 className="text-sm font-medium text-slate-300">System</h3>
      </div>

      {error && (
        <p className="text-xs text-red-400">Could not load status</p>
      )}

      {status && (
        <div className="space-y-3">
          <Stat label="Status" value="Healthy" valueClass="text-emerald-400" />
          <Stat label="Uptime" value={status.uptime} />
          <Stat label="Version" value={`v${status.version}`} />
        </div>
      )}

      {!status && !error && (
        <div className="space-y-3">
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

function Stat({ label, value, valueClass = 'text-slate-200' }: {
  label: string
  value: string
  valueClass?: string
}) {
  return (
    <div className="flex items-center justify-between">
      <span className="text-xs text-slate-500">{label}</span>
      <span className={`text-xs font-medium ${valueClass}`}>{value}</span>
    </div>
  )
}
