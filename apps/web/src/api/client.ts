export interface User {
  id: number
  email: string
}

export interface AppStatus {
  id: string
  name: string
  status: 'online' | 'offline' | 'starting' | 'sleeping'
  url: string
  icon: string
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

    login: (email: string, password: string): Promise<{ status: string }> =>
      request('/api/auth/login', {
        method: 'POST',
        body: JSON.stringify({ email, password }),
      }),

    logout: (): Promise<{ status: string }> =>
      request('/api/auth/logout', { method: 'POST' }),
  },

  status: (): Promise<ServerStatus> => request('/api/status'),

  apps: (): Promise<AppStatus[]> => request('/api/apps'),
}
