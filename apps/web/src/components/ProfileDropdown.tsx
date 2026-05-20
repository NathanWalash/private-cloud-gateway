import { useState, useRef, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { LogOut, Settings, User, ChevronDown } from 'lucide-react'
import { useAuth } from '../hooks/useAuth'

export default function ProfileDropdown() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  // Close on outside click
  useEffect(() => {
    function handler(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [])

  const initials = user
    ? ((user.first_name?.[0] ?? '') + (user.last_name?.[0] ?? user.email[0])).toUpperCase()
    : '?'

  const displayName = user?.first_name
    ? `${user.first_name}${user.last_name ? ' ' + user.last_name : ''}`
    : user?.email ?? ''

  async function handleLogout() {
    setOpen(false)
    await logout()
    navigate('/login', { replace: true })
  }

  return (
    <div ref={ref} className="relative">
      <button
        type="button"
        onClick={() => setOpen(v => !v)}
        className="flex items-center gap-1.5 hover:bg-white/5 px-2 py-1.5 rounded-lg transition-colors"
      >
        {/* Avatar */}
        <div className="w-7 h-7 rounded-full bg-accent/20 border border-accent/30 flex items-center justify-center text-xs font-semibold text-accent">
          {initials}
        </div>
        <ChevronDown className={`w-3 h-3 text-slate-500 transition-transform ${open ? 'rotate-180' : ''}`} />
      </button>

      {open && (
        <div className="absolute right-0 top-full mt-1.5 w-52 card shadow-xl z-50 py-1.5 border border-border">
          {/* User info */}
          <div className="px-3.5 py-2.5 border-b border-border mb-1">
            <p className="text-sm font-medium text-slate-200 truncate">{displayName}</p>
            <p className="text-xs text-slate-500 truncate">{user?.email}</p>
          </div>

          <DropItem
            icon={<User className="w-3.5 h-3.5" />}
            label="Edit profile"
            onClick={() => { setOpen(false); navigate('/settings') }}
          />
          <DropItem
            icon={<Settings className="w-3.5 h-3.5" />}
            label="Settings"
            onClick={() => { setOpen(false); navigate('/settings') }}
          />

          <div className="border-t border-border mt-1 pt-1">
            <DropItem
              icon={<LogOut className="w-3.5 h-3.5" />}
              label="Sign out"
              onClick={handleLogout}
              danger
            />
          </div>
        </div>
      )}
    </div>
  )
}

function DropItem({ icon, label, onClick, danger = false }: {
  icon: React.ReactNode
  label: string
  onClick: () => void
  danger?: boolean
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`w-full flex items-center gap-2.5 px-3.5 py-2 text-xs transition-colors hover:bg-white/5
        ${danger ? 'text-red-400 hover:text-red-300' : 'text-slate-300 hover:text-slate-100'}`}
    >
      {icon}
      {label}
    </button>
  )
}
