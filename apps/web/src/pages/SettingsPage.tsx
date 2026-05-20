import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Settings, Globe, Shield, Archive, ChevronLeft, Save, Loader2, CheckCircle } from 'lucide-react'
import { api } from '../api/client'

interface SettingField {
  key: string
  label: string
  description: string
  type: 'text' | 'password' | 'select'
  options?: string[]
  placeholder?: string
}

const FIELDS: SettingField[] = [
  { key: 'CLOUD_CORE_COOKIE_DOMAIN', label: 'Cookie domain', description: 'Root domain shared across all subdomains. Requires restart.', type: 'text', placeholder: 'localtest.me' },
  { key: 'CLOUD_CORE_LOGIN_URL', label: 'Login URL', description: 'Absolute URL of the login page. Requires restart.', type: 'text', placeholder: 'http://home.localtest.me/login' },
  { key: 'CLOUD_CORE_BACKUP_SCHEDULE', label: 'Backup schedule', description: 'Interval for automatic backups. Empty disables. Requires restart.', type: 'select', options: ['', '6h', '12h', '24h', '48h', '168h'], placeholder: 'disabled' },
  { key: 'CLOUD_CORE_BACKUP_PASSPHRASE', label: 'Backup passphrase', description: 'Encryption passphrase for backup archives. Requires restart.', type: 'password', placeholder: '(leave blank for unencrypted)' },
]

export default function SettingsPage() {
  const navigate = useNavigate()
  const [values, setValues] = useState<Record<string, string>>({})
  const [saving, setSaving] = useState<string | null>(null)
  const [saved, setSaved] = useState<string | null>(null)
  const [audit, setAudit] = useState<Array<{ id: number; action: string; actor: string; detail: string; created_at: string }>>([])

  useEffect(() => {
    api.settings.list().then(settings => {
      const map: Record<string, string> = {}
      settings.forEach(s => { map[s.key] = s.value })
      setValues(map)
    }).catch(() => {})

    api.audit(20).then(setAudit).catch(() => {})
  }, [])

  async function saveSetting(key: string, value: string) {
    setSaving(key)
    try {
      await api.settings.set(key, value)
      setSaved(key)
      setTimeout(() => setSaved(null), 2000)
    } catch { /* ignore */ }
    finally { setSaving(null) }
  }

  return (
    <div className="min-h-screen bg-surface">
      <header className="border-b border-border bg-card/50 sticky top-0 z-10">
        <div className="max-w-3xl mx-auto px-4 sm:px-6 h-14 flex items-center gap-3">
          <button
            onClick={() => navigate('/')}
            className="text-slate-400 hover:text-slate-200 p-1.5 rounded-md hover:bg-white/5 transition-colors"
          >
            <ChevronLeft className="w-4 h-4" />
          </button>
          <Settings className="w-4 h-4 text-slate-400" />
          <h1 className="font-semibold text-sm text-slate-100">Settings</h1>
        </div>
      </header>

      <main className="max-w-3xl mx-auto px-4 sm:px-6 py-8 space-y-8">

        {/* Runtime settings */}
        <section>
          <div className="flex items-center gap-2 mb-4">
            <Globe className="w-4 h-4 text-slate-400" />
            <h2 className="text-sm font-medium text-slate-300">Runtime configuration</h2>
          </div>
          <p className="text-xs text-slate-500 mb-4">
            These settings are stored in the database and take effect after the next restart.
            They override the corresponding environment variables.
          </p>
          <div className="space-y-4">
            {FIELDS.map(f => (
              <div key={f.key} className="card p-4">
                <label className="block text-xs font-medium text-slate-300 mb-1">{f.label}</label>
                <p className="text-xs text-slate-500 mb-2">{f.description}</p>
                <div className="flex gap-2">
                  {f.type === 'select' ? (
                    <select
                      value={values[f.key] ?? ''}
                      onChange={e => setValues(v => ({ ...v, [f.key]: e.target.value }))}
                      className="flex-1 input-field text-sm"
                    >
                      {f.options?.map(o => (
                        <option key={o} value={o}>{o || '(disabled)'}</option>
                      ))}
                    </select>
                  ) : (
                    <input
                      type={f.type}
                      className="flex-1 input-field text-sm"
                      placeholder={f.placeholder}
                      value={values[f.key] ?? ''}
                      onChange={e => setValues(v => ({ ...v, [f.key]: e.target.value }))}
                    />
                  )}
                  <button
                    onClick={() => saveSetting(f.key, values[f.key] ?? '')}
                    disabled={saving === f.key}
                    className="px-3 py-2 bg-accent/10 hover:bg-accent/20 border border-accent/20 text-accent rounded-lg text-xs transition-colors shrink-0"
                  >
                    {saved === f.key
                      ? <CheckCircle className="w-3.5 h-3.5" />
                      : saving === f.key
                        ? <Loader2 className="w-3.5 h-3.5 animate-spin" />
                        : <Save className="w-3.5 h-3.5" />
                    }
                  </button>
                </div>
              </div>
            ))}
          </div>
        </section>

        {/* Audit log */}
        <section>
          <div className="flex items-center gap-2 mb-4">
            <Shield className="w-4 h-4 text-slate-400" />
            <h2 className="text-sm font-medium text-slate-300">Recent activity</h2>
          </div>
          <div className="card divide-y divide-border">
            {audit.length === 0 && (
              <p className="text-xs text-slate-600 p-4">No activity recorded yet.</p>
            )}
            {audit.map(e => (
              <div key={e.id} className="px-4 py-2.5 flex items-start gap-3">
                <span className="text-xs font-medium text-slate-400 shrink-0 w-32 truncate">{e.action}</span>
                <span className="text-xs text-slate-500 flex-1 truncate">{e.actor}{e.detail ? ` — ${e.detail}` : ''}</span>
                <span className="text-xs text-slate-600 shrink-0">{new Date(e.created_at).toLocaleTimeString()}</span>
              </div>
            ))}
          </div>
        </section>

        {/* Security info */}
        <section>
          <div className="flex items-center gap-2 mb-4">
            <Archive className="w-4 h-4 text-slate-400" />
            <h2 className="text-sm font-medium text-slate-300">Data &amp; security</h2>
          </div>
          <div className="card p-4 space-y-2 text-xs text-slate-500">
            <p>Passwords are hashed with <strong className="text-slate-400">bcrypt</strong> — never stored in plain text.</p>
            <p>Session cookies are <strong className="text-slate-400">HttpOnly</strong> and <strong className="text-slate-400">SameSite=Lax</strong>.</p>
            <p>Backup archives are encrypted with <strong className="text-slate-400">AES-256-GCM</strong> when a passphrase is set.</p>
            <p>Only Caddy exposes public ports. App containers have no host port bindings.</p>
          </div>
        </section>

      </main>
    </div>
  )
}
