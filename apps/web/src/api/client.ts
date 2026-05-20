export interface User {
  id: number
  email: string
  first_name: string
  last_name: string
}

export interface App {
  id: number
  blueprint_id: string
  name: string
  icon: string
  subdomain: string
  url: string
  status: 'running' | 'stopped' | 'starting' | 'missing' | 'error'
  internal_port: number
  container_name: string
}

export interface Blueprint {
  id: string
  name: string
  description: string
  icon: string
  category: string
}

export interface ServerStatus {
  uptime: string
  version: string
}

class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      Accept: 'application/json',
      ...options?.headers,
    },
    ...options,
  })

  if (!res.ok) {
    const text = await res.text().catch(() => res.statusText)
    throw new ApiError(res.status, text)
  }

  const text = await res.text()
  return text ? (JSON.parse(text) as T) : ({} as T)
}

export { ApiError }

export const api = {
  auth: {
    me: (): Promise<User> => request('/api/auth/me'),
    needsSetup: (): Promise<{ needs_setup: boolean }> => request('/api/auth/setup'),
    setup: (email: string, password: string, firstName: string, lastName: string) =>
      request('/api/auth/setup', {
        method: 'POST',
        body: JSON.stringify({ email, password, first_name: firstName, last_name: lastName }),
      }),
    login: (email: string, password: string): Promise<{ status: string; needs_totp?: boolean; totp_token?: string }> =>
      request('/api/auth/login', {
        method: 'POST',
        body: JSON.stringify({ email, password }),
      }),
    logout: (): Promise<{ status: string }> =>
      request('/api/auth/logout', { method: 'POST' }),
    totp: {
      status: (): Promise<{ enabled: boolean }> => request('/api/auth/totp/status'),
      setup: (): Promise<{ secret: string; uri: string }> =>
        request('/api/auth/totp/setup', { method: 'POST' }),
      confirm: (secret: string, code: string): Promise<{ status: string }> =>
        request('/api/auth/totp/confirm', { method: 'POST', body: JSON.stringify({ secret, code }) }),
      disable: (code: string): Promise<{ status: string }> =>
        request('/api/auth/totp/disable', { method: 'POST', body: JSON.stringify({ code }) }),
      verify: (token: string, code: string): Promise<{ status: string }> =>
        request('/api/auth/totp/verify', { method: 'POST', body: JSON.stringify({ token, code }) }),
    },
  },

  status: (): Promise<ServerStatus> => request('/api/status'),

  apps: (): Promise<App[]> => request('/api/apps'),

  blueprints: (): Promise<Blueprint[]> => request('/api/blueprints'),

  installApp: (blueprintId: string): Promise<{ id: number; status: string }> =>
    request('/api/apps/install', {
      method: 'POST',
      body: JSON.stringify({ blueprint_id: blueprintId }),
    }),

  startApp: (id: number): Promise<{ status: string }> =>
    request(`/api/apps/${id}/start`, { method: 'POST' }),

  stopApp: (id: number): Promise<{ status: string }> =>
    request(`/api/apps/${id}/stop`, { method: 'POST' }),

  restartApp: (id: number): Promise<{ status: string }> =>
    request(`/api/apps/${id}/restart`, { method: 'POST' }),

  uninstallApp: (id: number): Promise<void> =>
    request(`/api/apps/${id}`, { method: 'DELETE' }),

  appLogs: (id: number, tail = 150): Promise<{ lines: string }> =>
    request(`/api/apps/${id}/logs?tail=${tail}`),

  updateApp: (id: number): Promise<{ status: string }> =>
    request(`/api/apps/${id}/update`, { method: 'POST' }),

  settings: {
    list: (): Promise<Array<{ key: string; value: string }>> => request('/api/settings'),
    set: (key: string, value: string) =>
      request(`/api/settings/${key}`, { method: 'PUT', body: JSON.stringify({ value }) }),
  },

  audit: (limit = 50) =>
    request<Array<{ id: number; action: string; actor: string; detail: string; created_at: string }>>(
      `/api/audit?limit=${limit}`
    ),

  monitors: {
    list: () => request<Array<{ id: number; name: string; url: string; status: string; latency_ms: number | null; last_checked: string | null }>>('/api/monitors'),
    create: (name: string, url: string) => request<{ id: number }>('/api/monitors', { method: 'POST', body: JSON.stringify({ name, url }) }),
    remove: (id: number): Promise<void> => request(`/api/monitors/${id}`, { method: 'DELETE' }),
  },

  backup: {
    list: (): Promise<BackupFile[]> => request('/api/backup/list'),
    create: (): Promise<{ name: string; size: number; volumes: number }> =>
      request('/api/backup/create', { method: 'POST' }),
    safeEscapeUrl: '/api/backup/safe-escape',
    restore: (file: File, passphrase?: string): Promise<{ status: string; message: string }> => {
      const form = new FormData()
      form.append('file', file)
      if (passphrase) form.append('passphrase', passphrase)
      return fetch('/api/backup/restore', {
        method: 'POST',
        credentials: 'include',
        body: form,
      }).then(async r => {
        if (!r.ok) throw new ApiError(r.status, await r.text().catch(() => r.statusText))
        return r.json() as Promise<{ status: string; message: string }>
      })
    },
  },
}

export interface BackupFile {
  name: string
  size: number
  created_at: string
}
