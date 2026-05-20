import { useState } from 'react'
import { Globe, AlertCircle, Loader2, ShieldCheck } from 'lucide-react'
import { api, ApiError } from '../api/client'

interface SetupPageProps {
  onComplete: () => void
}

export default function SetupPage({ onComplete }: SetupPageProps) {
  const [firstName, setFirstName] = useState('')
  const [lastName, setLastName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (password !== confirm) {
      setError('Passwords do not match.')
      return
    }
    if (password.length < 8) {
      setError('Password must be at least 8 characters.')
      return
    }
    setError('')
    setLoading(true)
    try {
      await api.auth.setup(email, password, firstName, lastName)
      onComplete()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Setup failed.')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen bg-surface flex items-center justify-center p-4">
      <div className="w-full max-w-md">
        {/* Brand */}
        <div className="text-center mb-8">
          <div className="inline-flex items-center justify-center w-14 h-14 bg-accent/10 border border-accent/20 rounded-2xl mb-4">
            <Globe className="w-7 h-7 text-accent" strokeWidth={1.5} />
          </div>
          <h1 className="text-2xl font-semibold text-slate-100">Welcome to Private Cloud Gateway</h1>
          <p className="text-sm text-slate-500 mt-1.5">Create your admin account to get started.</p>
        </div>

        <div className="card p-8">
          {/* Security note */}
          <div className="flex items-start gap-2.5 bg-accent/5 border border-accent/15 px-3.5 py-3 rounded-lg mb-6">
            <ShieldCheck className="w-4 h-4 text-accent shrink-0 mt-0.5" />
            <p className="text-xs text-slate-400">
              This is your single private account. Your password is bcrypt-hashed and never stored in plain text.
            </p>
          </div>

          {error && (
            <div className="flex items-center gap-2 bg-red-500/10 border border-red-500/20 text-red-400 text-sm px-3 py-2.5 rounded-lg mb-5">
              <AlertCircle className="w-4 h-4 shrink-0" />
              {error}
            </div>
          )}

          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="block text-xs font-medium text-slate-400 mb-1.5">First name</label>
                <input
                  type="text" required autoComplete="given-name"
                  className="input-field" placeholder="John"
                  value={firstName} onChange={e => setFirstName(e.target.value)}
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-slate-400 mb-1.5">Last name</label>
                <input
                  type="text" autoComplete="family-name"
                  className="input-field" placeholder="Smith"
                  value={lastName} onChange={e => setLastName(e.target.value)}
                />
              </div>
            </div>

            <div>
              <label className="block text-xs font-medium text-slate-400 mb-1.5">Email</label>
              <input
                type="email" required autoComplete="email"
                className="input-field" placeholder="you@example.com"
                value={email} onChange={e => setEmail(e.target.value)}
              />
            </div>

            <div>
              <label className="block text-xs font-medium text-slate-400 mb-1.5">Password</label>
              <input
                type="password" required autoComplete="new-password"
                className="input-field" placeholder="At least 8 characters"
                value={password} onChange={e => setPassword(e.target.value)}
              />
            </div>

            <div>
              <label className="block text-xs font-medium text-slate-400 mb-1.5">Confirm password</label>
              <input
                type="password" required autoComplete="new-password"
                className="input-field" placeholder="••••••••"
                value={confirm} onChange={e => setConfirm(e.target.value)}
              />
            </div>

            <button type="submit" disabled={loading} className="btn-primary mt-2">
              {loading
                ? <span className="flex items-center justify-center gap-2"><Loader2 className="w-4 h-4 animate-spin" />Creating account…</span>
                : 'Create account & get started'}
            </button>
          </form>
        </div>
      </div>
    </div>
  )
}
