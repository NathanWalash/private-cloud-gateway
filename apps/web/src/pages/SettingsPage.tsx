import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  Settings, Globe, Shield, Archive, ChevronLeft,
  Save, Loader2, CheckCircle, AlertCircle,
  ShieldCheck, ShieldOff, Key,
} from 'lucide-react'
import { api, ApiError } from '../api/client'

interface SettingField {
  key: string; label: string; description: string
  type: 'text' | 'password' | 'select'; options?: string[]; placeholder?: string
}

const FIELDS: SettingField[] = [
  { key: 'CLOUD_CORE_COOKIE_DOMAIN', label: 'Cookie domain', description: 'Root domain shared across all subdomains. Requires restart.', type: 'text', placeholder: 'localtest.me' },
  { key: 'CLOUD_CORE_LOGIN_URL', label: 'Login URL', description: 'Absolute URL of the login page. Requires restart.', type: 'text', placeholder: 'http://home.localtest.me/login' },
  { key: 'CLOUD_CORE_BACKUP_SCHEDULE', label: 'Backup schedule', description: 'Automatic backup interval. Empty to disable. Requires restart.', type: 'select', options: ['', '6h', '12h', '24h', '48h', '168h'] },
  { key: 'CLOUD_CORE_BACKUP_PASSPHRASE', label: 'Backup passphrase', description: 'AES-256 encryption passphrase for backups. Requires restart.', type: 'password', placeholder: '(leave blank for unencrypted)' },
]

export default function SettingsPage() {
  const navigate = useNavigate()
  const [values, setValues] = useState<Record<string, string>>({})
  const [saving, setSaving] = useState<string | null>(null)
  const [saved, setSaved] = useState<string | null>(null)
  const [audit, setAudit] = useState<Array<{ id: number; action: string; actor: string; detail: string; created_at: string }>>([])
  // TOTP state
  const [totpEnabled, setTotpEnabled] = useState<boolean | null>(null)
  const [totpSetup, setTotpSetup] = useState<{ secret: string; uri: string } | null>(null)
  const [totpCode, setTotpCode] = useState('')
  const [totpDisableCode, setTotpDisableCode] = useState('')
  const [totpBusy, setTotpBusy] = useState(false)
  const [totpMsg, setTotpMsg] = useState<{ ok: boolean; text: string } | null>(null)

  useEffect(() => {
    api.settings.list().then(settings => {
      const map: Record<string, string> = {}
      settings.forEach(s => { map[s.key] = s.value })
      setValues(map)
    }).catch(() => {})
    api.audit(20).then(setAudit).catch(() => {})
    api.auth.totp.status().then(r => setTotpEnabled(r.enabled)).catch(() => {})
  }, [])

  async function saveSetting(key: string, value: string) {
    setSaving(key); setSaved(null)
    try {
      await api.settings.set(key, value)
      setSaved(key)
      setTimeout(() => setSaved(null), 2000)
    } catch { /* ignore */ }
    finally { setSaving(null) }
  }

  async function startTOTPSetup() {
    setTotpBusy(true); setTotpMsg(null)
    try {
      const r = await api.auth.totp.setup()
      setTotpSetup(r); setTotpCode('')
    } catch { setTotpMsg({ ok: false, text: 'Failed to generate setup code.' }) }
    finally { setTotpBusy(false) }
  }

  async function confirmTOTP() {
    if (!totpSetup) return
    setTotpBusy(true); setTotpMsg(null)
    try {
      await api.auth.totp.confirm(totpSetup.secret, totpCode)
      setTotpEnabled(true); setTotpSetup(null); setTotpCode('')
      setTotpMsg({ ok: true, text: 'Two-factor authentication enabled.' })
    } catch (err) {
      setTotpMsg({ ok: false, text: err instanceof ApiError ? err.message : 'Invalid code.' })
    } finally { setTotpBusy(false) }
  }

  async function disableTOTP() {
    setTotpBusy(true); setTotpMsg(null)
    try {
      await api.auth.totp.disable(totpDisableCode)
      setTotpEnabled(false); setTotpDisableCode('')
      setTotpMsg({ ok: true, text: 'Two-factor authentication disabled.' })
    } catch (err) {
      setTotpMsg({ ok: false, text: err instanceof ApiError ? err.message : 'Invalid code.' })
    } finally { setTotpBusy(false) }
  }

  return (
    <div className="min-h-screen bg-surface">
      <header className="border-b border-border bg-card/50 sticky top-0 z-10">
        <div className="max-w-3xl mx-auto px-4 sm:px-6 h-14 flex items-center gap-3">
          <button type="button" onClick={() => navigate('/')} className="text-slate-400 hover:text-slate-200 p-1.5 rounded-md hover:bg-white/5 transition-colors">
            <ChevronLeft className="w-4 h-4" />
          </button>
          <Settings className="w-4 h-4 text-slate-400" />
          <h1 className="font-semibold text-sm text-slate-100">Settings</h1>
        </div>
      </header>

      <main className="max-w-3xl mx-auto px-4 sm:px-6 py-8 space-y-8">

        {/* Runtime config */}
        <section>
          <div className="flex items-center gap-2 mb-4">
            <Globe className="w-4 h-4 text-slate-400" />
            <h2 className="text-sm font-medium text-slate-300">Runtime configuration</h2>
          </div>
          <p className="text-xs text-slate-500 mb-4">Stored in the database. Take effect after restart.</p>
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
                      {f.options?.map(o => <option key={o} value={o}>{o || '(disabled)'}</option>)}
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
                    type="button"
                    onClick={() => saveSetting(f.key, values[f.key] ?? '')}
                    disabled={saving === f.key}
                    className="px-3 py-2 bg-accent/10 hover:bg-accent/20 border border-accent/20 text-accent rounded-lg text-xs transition-colors shrink-0"
                  >
                    {saved === f.key ? <CheckCircle className="w-3.5 h-3.5" />
                      : saving === f.key ? <Loader2 className="w-3.5 h-3.5 animate-spin" />
                      : <Save className="w-3.5 h-3.5" />}
                  </button>
                </div>
              </div>
            ))}
          </div>
        </section>

        {/* Two-factor authentication */}
        <section>
          <div className="flex items-center gap-2 mb-4">
            <Key className="w-4 h-4 text-slate-400" />
            <h2 className="text-sm font-medium text-slate-300">Two-factor authentication</h2>
          </div>

          {totpMsg && (
            <div className={`flex items-center gap-2 text-xs px-3 py-2.5 rounded-lg mb-4 border ${
              totpMsg.ok ? 'text-emerald-400 bg-emerald-500/10 border-emerald-500/20' : 'text-red-400 bg-red-500/10 border-red-500/20'
            }`}>
              {totpMsg.ok ? <CheckCircle className="w-4 h-4 shrink-0" /> : <AlertCircle className="w-4 h-4 shrink-0" />}
              {totpMsg.text}
            </div>
          )}

          <div className="card p-5">
            {totpEnabled === null ? (
              <p className="text-xs text-slate-500">Loading…</p>
            ) : totpEnabled ? (
              <div>
                <div className="flex items-center gap-2.5 mb-4">
                  <ShieldCheck className="w-5 h-5 text-emerald-400" />
                  <div>
                    <p className="text-sm font-medium text-slate-200">TOTP is enabled</p>
                    <p className="text-xs text-slate-500">Your account requires an authenticator code on login.</p>
                  </div>
                </div>
                <div className="flex gap-2">
                  <input
                    type="text" inputMode="numeric" maxLength={6}
                    placeholder="Enter current code to disable"
                    className="input-field flex-1 text-sm"
                    value={totpDisableCode}
                    onChange={e => setTotpDisableCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                  />
                  <button type="button" onClick={disableTOTP} disabled={totpBusy || totpDisableCode.length < 6}
                    className="px-3 py-2 bg-red-500/10 hover:bg-red-500/20 border border-red-500/20 text-red-400 rounded-lg text-xs transition-colors shrink-0">
                    {totpBusy ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <ShieldOff className="w-3.5 h-3.5" />}
                  </button>
                </div>
              </div>
            ) : !totpSetup ? (
              <div>
                <div className="flex items-center gap-2.5 mb-4">
                  <Shield className="w-5 h-5 text-slate-500" />
                  <div>
                    <p className="text-sm font-medium text-slate-300">TOTP not enabled</p>
                    <p className="text-xs text-slate-500">Add a second factor using Google Authenticator, Authy, or 1Password.</p>
                  </div>
                </div>
                <button type="button" onClick={startTOTPSetup} disabled={totpBusy}
                  className="flex items-center gap-2 text-xs font-medium px-4 py-2 bg-accent/10 hover:bg-accent/20 border border-accent/20 text-accent rounded-lg transition-colors">
                  {totpBusy ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Key className="w-3.5 h-3.5" />}
                  Set up authenticator app
                </button>
              </div>
            ) : (
              <div className="space-y-4">
                <p className="text-sm font-medium text-slate-200">Scan with your authenticator app</p>

                {/* Manual entry */}
                <div className="bg-surface border border-border rounded-lg p-3">
                  <p className="text-xs text-slate-500 mb-1.5">Manual entry key</p>
                  <p className="font-mono text-sm text-slate-300 select-all break-all">{totpSetup.secret}</p>
                </div>

                {/* URI for deep link */}
                <div className="bg-surface border border-border rounded-lg p-3">
                  <p className="text-xs text-slate-500 mb-1.5">Or copy this link and open in your authenticator:</p>
                  <a href={totpSetup.uri} className="text-xs text-accent hover:underline break-all">{totpSetup.uri.slice(0, 80)}…</a>
                </div>

                <div>
                  <label className="block text-xs font-medium text-slate-400 mb-1.5">Verify — enter the 6-digit code</label>
                  <div className="flex gap-2">
                    <input
                      type="text" inputMode="numeric" maxLength={6} autoFocus
                      className="input-field flex-1 text-center text-xl tracking-widest font-mono"
                      placeholder="000000"
                      value={totpCode}
                      onChange={e => setTotpCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                    />
                    <button type="button" onClick={confirmTOTP} disabled={totpBusy || totpCode.length < 6}
                      className="px-4 py-2 bg-accent/10 hover:bg-accent/20 border border-accent/20 text-accent rounded-lg text-sm transition-colors shrink-0 disabled:opacity-50">
                      {totpBusy ? <Loader2 className="w-4 h-4 animate-spin" /> : 'Enable'}
                    </button>
                  </div>
                </div>
                <button type="button" onClick={() => setTotpSetup(null)} className="text-xs text-slate-500 hover:text-slate-300 transition-colors">
                  Cancel
                </button>
              </div>
            )}
          </div>
        </section>

        {/* Audit log */}
        <section>
          <div className="flex items-center gap-2 mb-4">
            <Shield className="w-4 h-4 text-slate-400" />
            <h2 className="text-sm font-medium text-slate-300">Recent activity</h2>
          </div>
          <div className="card divide-y divide-border">
            {audit.length === 0 && <p className="text-xs text-slate-600 p-4">No activity recorded.</p>}
            {audit.map(e => (
              <div key={e.id} className="px-4 py-2.5 flex items-center gap-3">
                <span className="text-xs font-medium text-slate-400 w-36 shrink-0 truncate">{e.action}</span>
                <span className="text-xs text-slate-500 flex-1 truncate">{e.actor}{e.detail ? ` — ${e.detail}` : ''}</span>
                <span className="text-xs text-slate-600 shrink-0">{new Date(e.created_at).toLocaleString()}</span>
              </div>
            ))}
          </div>
        </section>

        {/* Security info */}
        <section>
          <div className="flex items-center gap-2 mb-4">
            <Archive className="w-4 h-4 text-slate-400" />
            <h2 className="text-sm font-medium text-slate-300">Security overview</h2>
          </div>
          <div className="card p-4 space-y-2 text-xs text-slate-500">
            <p>Passwords hashed with <strong className="text-slate-400">bcrypt</strong>.</p>
            <p>Session cookies are <strong className="text-slate-400">HttpOnly</strong>, <strong className="text-slate-400">SameSite=Lax</strong>.</p>
            <p>Backups encrypted with <strong className="text-slate-400">AES-256-GCM</strong> + PBKDF2 key derivation when a passphrase is set.</p>
            <p>Only Caddy exposes public ports. All app containers use <strong className="text-slate-400">expose:</strong> only.</p>
            <p>Login rate-limited to <strong className="text-slate-400">10 attempts / IP / minute</strong>.</p>
          </div>
        </section>

      </main>
    </div>
  )
}
