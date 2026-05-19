import { useState, useEffect, useCallback } from 'react'
import { api, User, ApiError } from '../api/client'

interface AuthState {
  user: User | null
  loading: boolean
  error: string | null
}

export function useAuth() {
  const [state, setState] = useState<AuthState>({ user: null, loading: true, error: null })

  const refresh = useCallback(async () => {
    setState(s => ({ ...s, loading: true, error: null }))
    try {
      const user = await api.auth.me()
      setState({ user, loading: false, error: null })
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        setState({ user: null, loading: false, error: null })
      } else {
        setState({ user: null, loading: false, error: 'Failed to verify session' })
      }
    }
  }, [])

  useEffect(() => { void refresh() }, [refresh])

  const logout = useCallback(async () => {
    await api.auth.logout()
    setState({ user: null, loading: false, error: null })
  }, [])

  return { ...state, refresh, logout }
}
