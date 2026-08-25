const API_BASE = import.meta.env.VITE_API_URL ?? ''

let csrfToken = ''

export function setCsrfToken(token: string) {
  csrfToken = token || ''
}

export function getCsrfToken() {
  return csrfToken
}

export type ApiError = { success: false; error: string }

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers || {})
  if (init.body && !(init.body instanceof FormData) && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }
  if (csrfToken && init.method && init.method !== 'GET' && init.method !== 'HEAD') {
    headers.set('X-CSRF-Token', csrfToken)
  }

  const res = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers,
    credentials: 'include',
  })

  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    throw new Error((data as ApiError).error || res.statusText || 'Request failed')
  }
  return data as T
}

export const api = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, body?: unknown) =>
    request<T>(path, {
      method: 'POST',
      body: body instanceof FormData ? body : body != null ? JSON.stringify(body) : undefined,
    }),
  delete: <T>(path: string) => request<T>(path, { method: 'DELETE' }),
}

export type AuthUser = {
  id: string
  email: string
  username: string
  onboarding_completed: boolean
  csrf_token?: string
}

export type NearbyUser = {
  id?: string
  username: string
  full_name?: string
  profession?: string
  lat: number
  lng: number
  distance_km?: number
  online?: boolean
  initials?: string
  photo_url?: string
  is_me?: boolean
  live?: boolean
}

export type Profession = { name: string; slug: string; count: number }
