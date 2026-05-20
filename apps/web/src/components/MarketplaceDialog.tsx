import { useState } from 'react'
import { X, Search, Package, Check, Loader2, AlertCircle } from 'lucide-react'
import { Blueprint, api, ApiError } from '../api/client'

interface MarketplaceDialogProps {
  blueprints: Blueprint[]
  onClose: () => void
  onInstalled: () => void
}

const CATEGORY_ICONS: Record<string, string> = {
  storage: '💾',
  utilities: '🛠',
  productivity: '✍️',
  monitoring: '📡',
  security: '🔐',
  finance: '💰',
  automation: '⚡',
}

const CATEGORY_ORDER = ['storage', 'productivity', 'utilities', 'monitoring', 'security', 'finance', 'automation']

export default function MarketplaceDialog({ blueprints, onClose, onInstalled }: MarketplaceDialogProps) {
  const [selected, setSelected] = useState<string | null>(null)
  const [search, setSearch] = useState('')
  const [activeCategory, setActiveCategory] = useState<string | null>(null)
  const [installing, setInstalling] = useState(false)
  const [error, setError] = useState('')

  const categories = [...new Set(blueprints.map(b => b.category).filter(Boolean))]
    .sort((a, b) => CATEGORY_ORDER.indexOf(a) - CATEGORY_ORDER.indexOf(b))

  const filtered = blueprints.filter(b => {
    const matchSearch = !search || b.name.toLowerCase().includes(search.toLowerCase()) || b.description.toLowerCase().includes(search.toLowerCase())
    const matchCat = !activeCategory || b.category === activeCategory
    return matchSearch && matchCat
  })

  async function handleInstall() {
    if (!selected) return
    setInstalling(true)
    setError('')
    try {
      await api.installApp(selected)
      onInstalled()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Installation failed')
    } finally {
      setInstalling(false)
    }
  }

  return (
    <div className="fixed inset-0 bg-black/70 backdrop-blur-sm flex items-center justify-center z-50 p-4">
      <div className="card w-full max-w-3xl max-h-[85vh] flex flex-col">

        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-border shrink-0">
          <div>
            <h2 className="font-semibold text-slate-100">App Marketplace</h2>
            <p className="text-xs text-slate-500 mt-0.5">{blueprints.length} apps available</p>
          </div>
          <button type="button" onClick={onClose} className="text-slate-500 hover:text-slate-300 p-1.5 rounded-lg hover:bg-white/5">
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Search */}
        <div className="px-6 py-3 border-b border-border shrink-0">
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-slate-500" />
            <input
              autoFocus
              type="text"
              placeholder="Search apps…"
              value={search}
              onChange={e => setSearch(e.target.value)}
              className="input-field pl-8 py-2 text-sm"
            />
          </div>
        </div>

        <div className="flex flex-1 min-h-0">
          {/* Category sidebar */}
          <div className="w-36 border-r border-border py-3 shrink-0 overflow-y-auto">
            <button
              type="button"
              onClick={() => setActiveCategory(null)}
              className={`w-full text-left px-4 py-2 text-xs transition-colors ${
                !activeCategory ? 'text-accent font-medium bg-accent/10' : 'text-slate-400 hover:text-slate-200 hover:bg-white/5'
              }`}
            >
              All apps
            </button>
            {categories.map(cat => (
              <button
                key={cat}
                type="button"
                onClick={() => setActiveCategory(activeCategory === cat ? null : cat)}
                className={`w-full text-left px-4 py-2 text-xs transition-colors flex items-center gap-2 capitalize ${
                  activeCategory === cat ? 'text-accent font-medium bg-accent/10' : 'text-slate-400 hover:text-slate-200 hover:bg-white/5'
                }`}
              >
                <span>{CATEGORY_ICONS[cat] ?? '📦'}</span>
                {cat}
              </button>
            ))}
          </div>

          {/* App grid */}
          <div className="flex-1 overflow-y-auto p-4">
            {filtered.length === 0 && (
              <div className="flex flex-col items-center justify-center h-32 text-slate-500">
                <Package className="w-8 h-8 mb-2 text-slate-700" />
                <p className="text-sm">No apps match your search</p>
              </div>
            )}
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              {filtered.map(bp => (
                <button
                  key={bp.id}
                  type="button"
                  onClick={() => setSelected(selected === bp.id ? null : bp.id)}
                  className={`text-left p-4 rounded-xl border transition-all ${
                    selected === bp.id
                      ? 'border-accent bg-accent/10 ring-1 ring-accent/30'
                      : 'border-border bg-card/50 hover:border-slate-600 hover:bg-card'
                  }`}
                >
                  <div className="flex items-start justify-between mb-2">
                    <div className="flex items-center gap-2.5">
                      <span className="text-2xl leading-none">{bp.icon || '📦'}</span>
                      <div>
                        <p className="text-sm font-medium text-slate-200">{bp.name}</p>
                        <p className="text-xs text-slate-500 capitalize">{bp.category}</p>
                      </div>
                    </div>
                    {selected === bp.id && (
                      <div className="w-5 h-5 rounded-full bg-accent flex items-center justify-center shrink-0">
                        <Check className="w-3 h-3 text-white" />
                      </div>
                    )}
                  </div>
                  {bp.description && (
                    <p className="text-xs text-slate-500 line-clamp-2 mt-1">{bp.description}</p>
                  )}
                </button>
              ))}
            </div>
          </div>
        </div>

        {/* Footer */}
        <div className="px-6 py-4 border-t border-border shrink-0">
          {error && (
            <div className="flex items-center gap-2 text-xs text-red-400 bg-red-500/10 border border-red-500/20 px-3 py-2 rounded-lg mb-3">
              <AlertCircle className="w-3.5 h-3.5 shrink-0" />{error}
            </div>
          )}
          <div className="flex items-center justify-between">
            <p className="text-xs text-slate-500">
              {selected
                ? `Installing: ${blueprints.find(b => b.id === selected)?.name}`
                : 'Select an app to install'}
            </p>
            <div className="flex gap-3">
              <button type="button" onClick={onClose} className="text-sm text-slate-400 hover:text-slate-200 px-4 py-2 transition-colors">
                Cancel
              </button>
              <button
                type="button"
                onClick={handleInstall}
                disabled={!selected || installing}
                className="btn-primary !w-auto px-6 text-sm disabled:opacity-50"
              >
                {installing
                  ? <span className="flex items-center gap-2"><Loader2 className="w-3.5 h-3.5 animate-spin" />Installing…</span>
                  : 'Install'}
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
