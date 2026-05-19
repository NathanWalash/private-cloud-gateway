import { useState } from 'react'
import { Blueprint, api, ApiError } from '../api/client'

interface InstallDialogProps {
  blueprints: Blueprint[]
  onClose: () => void
  onInstalled: () => void
}

export default function InstallDialog({ blueprints, onClose, onInstalled }: InstallDialogProps) {
  const [selected, setSelected] = useState<string | null>(null)
  const [installing, setInstalling] = useState(false)
  const [error, setError] = useState('')

  async function handleInstall() {
    if (!selected) return
    setInstalling(true)
    setError('')
    try {
      await api.installApp(selected)
      onInstalled()
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message)
      } else {
        setError('Installation failed')
      }
    } finally {
      setInstalling(false)
    }
  }

  return (
    <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50 p-4">
      <div className="card w-full max-w-lg">
        {/* Header */}
        <div className="flex items-center justify-between p-5 border-b border-border">
          <h2 className="font-semibold text-slate-100">Install App</h2>
          <button
            onClick={onClose}
            className="text-slate-500 hover:text-slate-300 transition-colors p-1"
          >
            ✕
          </button>
        </div>

        {/* Body */}
        <div className="p-5">
          {blueprints.length === 0 ? (
            <div className="text-center py-6">
              <p className="text-sm text-slate-400 mb-1">No blueprints found</p>
              <p className="text-xs text-slate-600">
                Add YAML files to the <code className="bg-surface px-1 rounded">blueprints/</code> directory.
              </p>
            </div>
          ) : (
            <div className="space-y-2 max-h-72 overflow-y-auto">
              {blueprints.map(bp => (
                <button
                  key={bp.id}
                  onClick={() => setSelected(bp.id)}
                  className={`w-full text-left p-3.5 rounded-lg border transition-all
                    ${selected === bp.id
                      ? 'border-accent bg-accent/10'
                      : 'border-border hover:border-slate-600 hover:bg-white/3'}`}
                >
                  <div className="flex items-center gap-3">
                    <span className="text-xl">{bp.icon || '📦'}</span>
                    <div>
                      <p className="text-sm font-medium text-slate-200">{bp.name}</p>
                      {bp.description && (
                        <p className="text-xs text-slate-500 mt-0.5">{bp.description}</p>
                      )}
                    </div>
                    {selected === bp.id && (
                      <span className="ml-auto text-accent text-sm">✓</span>
                    )}
                  </div>
                </button>
              ))}
            </div>
          )}

          {error && (
            <p className="text-xs text-red-400 mt-3 bg-red-500/10 border border-red-500/20 px-3 py-2 rounded-lg">
              {error}
            </p>
          )}
        </div>

        {/* Footer */}
        <div className="flex items-center justify-end gap-3 p-5 border-t border-border">
          <button
            onClick={onClose}
            className="text-sm text-slate-400 hover:text-slate-200 px-4 py-2 transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={handleInstall}
            disabled={!selected || installing || blueprints.length === 0}
            className="btn-primary !w-auto px-6 text-sm"
          >
            {installing ? (
              <span className="flex items-center gap-2">
                <span className="w-3.5 h-3.5 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                Installing…
              </span>
            ) : (
              'Install'
            )}
          </button>
        </div>
      </div>
    </div>
  )
}
