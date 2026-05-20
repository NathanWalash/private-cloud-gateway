import { useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Globe, Plus, LogOut, Package, Settings, Sun, Sunset, Moon } from 'lucide-react'
import { useAuth } from '../hooks/useAuth'
import { api, App, Blueprint } from '../api/client'
import AppCard from '../components/AppCard'
import StatusWidget from '../components/StatusWidget'
import BackupWidget from '../components/BackupWidget'
import MonitorWidget from '../components/MonitorWidget'
import InstallDialog from '../components/InstallDialog'

export default function DashboardPage() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const [apps, setApps] = useState<App[]>([])
  const [appsLoading, setAppsLoading] = useState(true)
  const [showInstall, setShowInstall] = useState(false)
  const [blueprints, setBlueprints] = useState<Blueprint[]>([])

  const loadApps = useCallback(async () => {
    try { setApps(await api.apps()) }
    catch { /* ignore */ }
    finally { setAppsLoading(false) }
  }, [])

  useEffect(() => {
    void loadApps()
    // Poll every 30s to stay in sync with server-side health polling
    const id = setInterval(() => { void loadApps() }, 30_000)
    return () => clearInterval(id)
  }, [loadApps])

  async function openInstall() {
    const bps = await api.blueprints().catch(() => [])
    setBlueprints(bps)
    setShowInstall(true)
  }

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
              <Globe className="w-4 h-4 text-accent" strokeWidth={1.5} />
            </div>
            <span className="font-semibold text-sm text-slate-100">Private Cloud Gateway</span>
          </div>
          <div className="flex items-center gap-2">
            <span className="text-xs text-slate-500 hidden sm:block">{user?.email}</span>
            <button
              type="button"
              onClick={() => navigate('/settings')}
              className="text-slate-400 hover:text-slate-200 transition-colors p-1.5 rounded-lg hover:bg-white/5"
              title="Settings"
            >
              <Settings className="w-3.5 h-3.5" />
            </button>
            <button
              type="button"
              onClick={handleLogout}
              className="flex items-center gap-1.5 text-xs text-slate-400 hover:text-slate-200 transition-colors px-2.5 py-1.5 rounded-lg hover:bg-white/5"
            >
              <LogOut className="w-3.5 h-3.5" />
              Sign out
            </button>
          </div>
        </div>
      </header>

      <main className="max-w-6xl mx-auto px-4 sm:px-6 py-8">
        {/* Greeting */}
        <div className="mb-8 flex items-center gap-3">
          <GreetingIcon />
          <div>
            <h1 className="text-2xl font-semibold text-slate-100">
              Good {getGreeting()},{' '}
              <span className="text-slate-400">{user?.first_name || user?.email.split('@')[0]}</span>
            </h1>
            <p className="text-sm text-slate-500 mt-0.5">Your private cloud is running.</p>
          </div>
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-4 gap-6">
          {/* Apps — 3/4 */}
          <div className="lg:col-span-3 space-y-4">
            <div className="flex items-center justify-between">
              <h2 className="text-sm font-medium text-slate-400 uppercase tracking-wider">Apps</h2>
              <div className="flex items-center gap-3">
                {apps.length > 0 && <span className="text-xs text-slate-600">{apps.length} installed</span>}
                <button
                  onClick={openInstall}
                  className="flex items-center gap-1.5 text-xs font-medium text-accent hover:text-accent-hover
                             bg-accent/10 hover:bg-accent/20 border border-accent/20 px-3 py-1.5 rounded-lg transition-colors"
                >
                  <Plus className="w-3.5 h-3.5" />
                  Install app
                </button>
              </div>
            </div>

            {appsLoading && (
              <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-3">
                {[1, 2, 3].map(i => (
                  <div key={i} className="card p-5 h-36 animate-pulse">
                    <div className="w-10 h-10 bg-border rounded-lg mb-3" />
                    <div className="h-3 bg-border rounded w-24 mb-2" />
                    <div className="h-2 bg-border rounded w-32" />
                  </div>
                ))}
              </div>
            )}

            {!appsLoading && apps.length === 0 && (
              <div className="card p-10 text-center">
                <Package className="w-10 h-10 text-slate-600 mx-auto mb-4" strokeWidth={1.5} />
                <h3 className="text-sm font-medium text-slate-400 mb-1">No apps installed</h3>
                <p className="text-xs text-slate-600 max-w-xs mx-auto mb-4">
                  Install apps from YAML blueprints. Each app gets its own protected subdomain.
                </p>
                <button
                  onClick={openInstall}
                  className="flex items-center gap-1.5 text-xs font-medium text-accent hover:text-accent-hover
                             bg-accent/10 hover:bg-accent/20 border border-accent/20 px-4 py-2 rounded-lg
                             transition-colors mx-auto"
                >
                  <Plus className="w-3.5 h-3.5" />
                  Install your first app
                </button>
              </div>
            )}

            {!appsLoading && apps.length > 0 && (
              <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-3">
                {apps.map(app => <AppCard key={app.id} app={app} onStatusChange={loadApps} />)}
              </div>
            )}
          </div>

          {/* Sidebar — 1/4 */}
          <div className="space-y-4">
            <h2 className="text-sm font-medium text-slate-400 uppercase tracking-wider">Status</h2>
            <StatusWidget />
            <BackupWidget />
            <MonitorWidget />
          </div>
        </div>
      </main>

      {showInstall && (
        <InstallDialog
          blueprints={blueprints}
          onClose={() => setShowInstall(false)}
          onInstalled={() => { setShowInstall(false); void loadApps() }}
        />
      )}
    </div>
  )
}

function getGreeting() {
  const h = new Date().getHours()
  if (h < 12) return 'morning'
  if (h < 17) return 'afternoon'
  return 'evening'
}

function GreetingIcon() {
  const h = new Date().getHours()
  const cls = "w-7 h-7"
  if (h < 12) return <Sun className={`${cls} text-amber-400`} strokeWidth={1.5} />
  if (h < 17) return <Sunset className={`${cls} text-orange-400`} strokeWidth={1.5} />
  return <Moon className={`${cls} text-blue-400`} strokeWidth={1.5} />
}
