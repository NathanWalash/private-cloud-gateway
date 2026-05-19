import { AppStatus } from '../api/client'

const statusConfig = {
  online: { label: 'Online', dot: 'bg-emerald-400', text: 'text-emerald-400' },
  offline: { label: 'Offline', dot: 'bg-slate-500', text: 'text-slate-500' },
  starting: { label: 'Starting', dot: 'bg-amber-400 animate-pulse', text: 'text-amber-400' },
  sleeping: { label: 'Sleeping', dot: 'bg-blue-400', text: 'text-blue-400' },
}

interface AppCardProps {
  app: AppStatus
}

export default function AppCard({ app }: AppCardProps) {
  const status = statusConfig[app.status]

  return (
    <a
      href={app.url}
      target="_blank"
      rel="noopener noreferrer"
      className="card p-5 flex flex-col gap-3 hover:border-slate-600 hover:bg-card/80 transition-all duration-150 group"
    >
      <div className="flex items-start justify-between">
        <div className="w-10 h-10 bg-accent/10 border border-accent/20 rounded-lg flex items-center justify-center text-lg">
          {app.icon}
        </div>
        <span className={`flex items-center gap-1.5 text-xs font-medium ${status.text}`}>
          <span className={`w-1.5 h-1.5 rounded-full ${status.dot}`} />
          {status.label}
        </span>
      </div>
      <div>
        <h3 className="font-medium text-slate-100 group-hover:text-white transition-colors">{app.name}</h3>
        <p className="text-xs text-slate-500 mt-0.5 truncate">{app.url}</p>
      </div>
    </a>
  )
}
