import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth'
import { api, AppStatus } from '../api/client'
import AppCard from '../components/AppCard'
import StatusWidget from '../components/StatusWidget'

export default function DashboardPage() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const [apps, setApps] = useState<AppStatus[]>([])
  const [appsLoading, setAppsLoading] = useState(true)

  useEffect(() => {
    api.apps()
      .then(setApps)
      .catch(() => {})
      .finally(() => setAppsLoading(false))
  }, [])

  async function handleLogout() {
    await logout()
    navigate('/login', { replace: true })
  }

  return (
    <div className="min-h-screen bg-surface">
      {/* Header */}
      <header className="border-b border-border bg-card/50 backdrop-blur-sm sticky top-0 z-10">
        <div className="max-w-6xl mx-auto px-4 sm:px-6 h-14 flex items-center justify-between">
          <div className="flex items-center gap-2.5">
            <div className="w-7 h-7 bg-accent/10 border border-accent/20 rounded-lg flex items-center justify-center">
              <svg className="w-4 h-4 text-accent" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                <path strokeLinecap="round" strokeLinejoin="round"
                  d="M12 21a9.004 9.004 0 008.716-6.747M12 21a9.004 9.004 0 01-8.716-6.747M12 21c2.485 0 4.5-4.03 4.5-9S14.485 3 12 3m0 18c-2.485 0-4.5-4.03-4.5-9S9.515 3 12 3m0 0a8.997 8.997 0 017.843 4.582M12 3a8.997 8.997 0 00-7.843 4.582m15.686 0A11.953 11.953 0 0112 10.5c-2.998 0-5.74-1.1-7.843-2.918m15.686 0A8.959 8.959 0 0121 12c0 .778-.099 1.533-.284 2.253m0 0A17.919 17.919 0 0112 16.5c-3.162 0-6.133-.815-8.716-2.247m0 0A9.015 9.015 0 013 12c0-1.605.42-3.113 1.157-4.418" />
              </svg>
            </div>
            <span className="font-semibold text-sm text-slate-100">Private Cloud Gateway</span>
          </div>

          <div className="flex items-center gap-3">
            <span className="text-xs text-slate-500 hidden sm:block">{user?.email}</span>
            <button
              onClick={handleLogout}
              className="text-xs text-slate-400 hover:text-slate-200 transition-colors px-2.5 py-1.5 rounded-lg hover:bg-white/5"
            >
              Sign out
            </button>
          </div>
        </div>
      </header>

      {/* Main content */}
      <main className="max-w-6xl mx-auto px-4 sm:px-6 py-8">
        {/* Welcome */}
        <div className="mb-8">
          <h1 className="text-2xl font-semibold text-slate-100">
            Good {getGreeting()},{' '}
            <span className="text-slate-400">{user?.email.split('@')[0]}</span>
          </h1>
          <p className="text-sm text-slate-500 mt-1">Your private cloud is running.</p>
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-4 gap-6">
          {/* Apps section — takes up 3/4 */}
          <div className="lg:col-span-3 space-y-4">
            <div className="flex items-center justify-between">
              <h2 className="text-sm font-medium text-slate-400 uppercase tracking-wider">
                Apps
              </h2>
              {apps.length > 0 && (
                <span className="text-xs text-slate-600">{apps.length} installed</span>
              )}
            </div>

            {appsLoading && (
              <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-3">
                {[1, 2, 3].map(i => (
                  <div key={i} className="card p-5 h-28 animate-pulse">
                    <div className="w-10 h-10 bg-border rounded-lg mb-3" />
                    <div className="h-3 bg-border rounded w-24" />
                  </div>
                ))}
              </div>
            )}

            {!appsLoading && apps.length === 0 && (
              <div className="card p-10 text-center">
                <div className="w-12 h-12 bg-accent/5 border border-accent/10 rounded-xl flex items-center justify-center mx-auto mb-4">
                  <svg className="w-6 h-6 text-slate-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5}
                      d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4" />
                  </svg>
                </div>
                <h3 className="text-sm font-medium text-slate-400 mb-1">No apps installed</h3>
                <p className="text-xs text-slate-600 max-w-xs mx-auto">
                  Apps will appear here after you install them from blueprints.
                  Blueprint management is coming in Milestone 3.
                </p>
              </div>
            )}

            {!appsLoading && apps.length > 0 && (
              <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-3">
                {apps.map(app => <AppCard key={app.id} app={app} />)}
              </div>
            )}
          </div>

          {/* Sidebar — 1/4 */}
          <div className="space-y-4">
            <h2 className="text-sm font-medium text-slate-400 uppercase tracking-wider">Status</h2>
            <StatusWidget />

            {/* Backup status placeholder */}
            <div className="card p-5">
              <div className="flex items-center gap-2 mb-4">
                <div className="w-2 h-2 rounded-full bg-slate-600" />
                <h3 className="text-sm font-medium text-slate-500">Backups</h3>
              </div>
              <p className="text-xs text-slate-600">
                Backup management coming in Milestone 4.
              </p>
            </div>
          </div>
        </div>
      </main>
    </div>
  )
}

function getGreeting() {
  const h = new Date().getHours()
  if (h < 12) return 'morning'
  if (h < 17) return 'afternoon'
  return 'evening'
}
