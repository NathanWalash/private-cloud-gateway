import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { Globe, AlertCircle, Loader2, ShieldCheck } from 'lucide-react'
import { api, ApiError } from '../api/client'

export default function LoginPage() {
  const navigate = useNavigate()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [totpToken, setTotpToken] = useState<string | null>(null)
  const [totpCode, setTotpCode] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    api.auth.me().then(() => navigate('/', { replace: true })).catch(() => {})
  }, [navigate])

  async function handleLogin(e: React.FormEvent) {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      const r = await api.auth.login(email, password)
      if (r.needs_totp && r.totp_token) {
        setTotpToken(r.totp_token)
      } else {
        navigate('/', { replace: true })
      }
    } catch (err) {
      if (err instanceof ApiError && err.status === 429) {
        setError('Too many attempts. Try again in a minute.')
      } else {
        setError('Invalid email or password.')
      }
    } finally {
      setLoading(false)
    }
  }

  async function handleTOTP(e: React.FormEvent) {
    e.preventDefault()
    if (!totpToken) return
    setError('')
    setLoading(true)
    try {
      await api.auth.totp.verify(totpToken, totpCode)
      navigate('/', { replace: true })
    } catch {
      setError('Invalid authenticator code.')
      setTotpCode('')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen bg-surface flex items-center justify-center p-4">
      <div className="w-full max-w-sm">
        <div className="text-center mb-8">
          <div className="inline-flex items-center justify-center w-12 h-12 bg-accent/10 border border-accent/20 rounded-xl mb-4">
            <Globe className="w-6 h-6 text-accent" strokeWidth={1.5} />
          </div>
          <h1 className="text-xl font-semibold text-slate-100">Private Cloud Gateway</h1>
          <p className="text-sm text-slate-500 mt-1">
            {totpToken ? 'Two-factor authentication' : 'Sign in to your private cloud'}
          </p>
        </div>

        <div className="card p-8">
          {error && (
            <div className="flex items-center gap-2 bg-red-500/10 border border-red-500/20 text-red-400 text-sm px-3 py-2.5 rounded-lg mb-5">
              <AlertCircle className="w-4 h-4 shrink-0" />
              {error}
            </div>
          )}

          {!totpToken ? (
            <form onSubmit={handleLogin} className="space-y-4">
              <div>
                <label htmlFor="email" className="block text-xs font-medium text-slate-400 mb-1.5">Email</label>
                <input
                  id="email" type="email" autoComplete="email" required
                  className="input-field" placeholder="admin@example.com"
                  value={email} onChange={e => setEmail(e.target.value)}
                />
              </div>
              <div>
                <label htmlFor="password" className="block text-xs font-medium text-slate-400 mb-1.5">Password</label>
                <input
                  id="password" type="password" autoComplete="current-password" required
                  className="input-field" placeholder="••••••••"
                  value={password} onChange={e => setPassword(e.target.value)}
                />
              </div>
              <button type="submit" disabled={loading} className="btn-primary mt-2">
                {loading
                  ? <><Loader2 className="w-4 h-4 animate-spin" /> Signing in…</>
                  : 'Sign in'}
              </button>
            </form>
          ) : (
            <form onSubmit={handleTOTP} className="space-y-4">
              <div className="flex items-center gap-2 text-xs text-slate-400 bg-accent/5 border border-accent/15 px-3 py-2.5 rounded-lg mb-1">
                <ShieldCheck className="w-4 h-4 text-accent shrink-0" />
                Enter the 6-digit code from your authenticator app.
              </div>
              <div>
                <label htmlFor="totp" className="block text-xs font-medium text-slate-400 mb-1.5">Authenticator code</label>
                <input
                  id="totp" type="text" autoComplete="one-time-code"
                  inputMode="numeric" pattern="[0-9]*" maxLength={6}
                  required autoFocus
                  className="input-field text-center text-xl tracking-widest font-mono"
                  placeholder="000000"
                  value={totpCode} onChange={e => setTotpCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                />
              </div>
              <button type="submit" disabled={loading || totpCode.length < 6} className="btn-primary">
                {loading
                  ? <><Loader2 className="w-4 h-4 animate-spin" /> Verifying…</>
                  : 'Verify'}
              </button>
              <button
                type="button"
                onClick={() => { setTotpToken(null); setError(''); setTotpCode('') }}
                className="w-full text-xs text-slate-500 hover:text-slate-300 transition-colors py-1"
              >
                Back to login
              </button>
            </form>
          )}
        </div>
      </div>
    </div>
  )
}
