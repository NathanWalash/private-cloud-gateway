import { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Globe, Plus, Package, Settings, Sun, Sunset, Moon, Activity, Archive, Radio } from 'lucide-react'
import ProfileDropdown from '../components/ProfileDropdown'
import { useAuth } from '../hooks/useAuth'
import { useTheme } from '../hooks/useTheme'
import { api, App, Blueprint, UpdateInfo } from '../api/client'
import AppCard from '../components/AppCard'
import StatusWidget from '../components/StatusWidget'
import BackupWidget from '../components/BackupWidget'
import MonitorWidget from '../components/MonitorWidget'
import MarketplaceDialog from '../components/MarketplaceDialog'

export default function DashboardPage() {
  const { user } = useAuth()
  const navigate = useNavigate()
  const { theme, toggle: toggleTheme } = useTheme()
  const [apps, setApps] = useState<App[]>([])
  const [appsLoading, setAppsLoading] = useState(true)
  const [showInstall, setShowInstall] = useState(false)
  const [blueprints, setBlueprints] = useState<Blueprint[]>([])
  const [updateInfos, setUpdateInfos] = useState<UpdateInfo[]>([])
  const [mobileSidebar, setMobileSidebar] = useState<'status' | 'backup' | 'monitors'>('status')
  const sseRef = useRef<EventSource | null>(null)

  const loadApps = useCallback(async () => {
    try { setApps(await api.apps()) }
    catch { /* ignore */ }
    finally { setAppsLoading(false) }
  }, [])

  // Load update availability (non-critical, best-effort)
  useEffect(() => {
    api.appUpdates().then(setUpdateInfos).catch(() => {})
    const id = setInterval(() => {
      api.appUpdates().then(setUpdateInfos).catch(() => {})
    }, 6 * 60 * 60 * 1000) // re-check every 6h
    return () => clearInterval(id)
  }, [])

  useEffect(() => {
    void loadApps()
    // SSE for real-time status changes, fallback to 30s polling
    const sse = new EventSource(api.appEventsUrl, { withCredentials: true })
    sseRef.current = sse
    sse.onmessage = (e) => {
      try {
        const ev = JSON.parse(e.data) as { app_id: number; status: string }
        setApps(prev => prev.map(a => a.id === ev.app_id ? { ...a, status: ev.status as App['status'] } : a))
      } catch { /* ignore */ }
    }
    sse.onerror = () => {
      // SSE failed — fall back to polling
      sse.close()
    }
    // Fallback polling if SSE drops
    const id = setInterval(() => { void loadApps() }, 30_000)
    return () => {
      sse.close()
      clearInterval(id)
    }
  }, [loadApps])

  async function openInstall() {
    const bps = await api.blueprints().catch(() => [])
    setBlueprints(bps)
    setShowInstall(true)
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
            <span className="font-semibold text-sm text-slate-100 hidden sm:inline">Private Cloud Gateway</span>
            <span className="font-semibold text-sm text-slate-100 sm:hidden">PCG</span>
          </div>
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={toggleTheme}
              className="text-slate-400 hover:text-slate-200 transition-colors p-1.5 rounded-lg hover:bg-white/5"
              title={theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
            >
              {theme === 'dark'
                ? <Sun className="w-3.5 h-3.5" />
                : <Moon className="w-3.5 h-3.5" />
              }
            </button>
            <button
              type="button"
              onClick={() => navigate('/settings')}
              className="text-slate-400 hover:text-slate-200 transition-colors p-1.5 rounded-lg hover:bg-white/5"
              title="Settings"
            >
              <Settings className="w-3.5 h-3.5" />
            </button>
            <ProfileDropdown />
          </div>
        </div>
      </header>

      <main className="max-w-6xl mx-auto px-4 sm:px-6 py-8">
        {/* Greeting */}
        <div className="mb-6 sm:mb-8 flex items-center gap-3">
          <GreetingIcon />
          <div>
            <h1 className="text-xl sm:text-2xl font-semibold text-text-primary">
              Good {getGreeting()},{' '}
              <span className="text-text-muted">{user?.first_name || user?.email.split('@')[0]}</span>
            </h1>
            <p className="text-xs sm:text-sm mt-0.5 text-text-muted">Your private cloud is running.</p>
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
                  type="button"
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
                  type="button"
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
                {apps.map(app => (
                  <AppCard
                    key={app.id}
                    app={app}
                    onStatusChange={loadApps}
                    updateAvailable={updateInfos.find(u => u.app_id === app.id)?.update_available ?? false}
                  />
                ))}
              </div>
            )}
          </div>

          {/* Sidebar — 1/4, tabs on mobile */}
          <div>
            {/* Mobile tabs */}
            <div className="lg:hidden flex border border-border rounded-lg overflow-hidden mb-4">
              {([ ['status', 'Status', Activity], ['backup', 'Backup', Archive], ['monitors', 'Monitors', Radio] ] as const).map(([id, label, Icon]) => (
                <button
                  key={id}
                  type="button"
                  onClick={() => setMobileSidebar(id)}
                  className={`flex-1 flex items-center justify-center gap-1.5 py-2.5 text-xs font-medium transition-colors
                    ${mobileSidebar === id
                      ? 'bg-accent/10 text-accent border-b-2 border-accent'
                      : 'text-slate-500 hover:text-slate-300 hover:bg-white/5'
                    }`}
                >
                  <Icon className="w-3.5 h-3.5" />
                  {label}
                </button>
              ))}
            </div>
            {/* Desktop: all visible; mobile: only selected tab */}
            <div className={`space-y-4 ${mobileSidebar !== 'status' ? 'lg:block hidden' : ''}`}>
              <h2 className="hidden lg:block text-sm font-medium text-slate-400 uppercase tracking-wider">Status</h2>
              <StatusWidget />
            </div>
            <div className={`space-y-4 mt-4 ${mobileSidebar !== 'backup' ? 'lg:block hidden' : ''}`}>
              <BackupWidget />
            </div>
            <div className={`space-y-4 mt-4 ${mobileSidebar !== 'monitors' ? 'lg:block hidden' : ''}`}>
              <MonitorWidget />
            </div>
          </div>
        </div>
      </main>

      {showInstall && (
        <MarketplaceDialog
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
