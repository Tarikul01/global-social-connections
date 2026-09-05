import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react'
import { api, setCsrfToken, type AuthUser } from '../lib/api'

type AuthState = {
  user: AuthUser | null
  loading: boolean
  login: (email: string, password: string) => Promise<AuthUser>
  register: (input: {
    email: string
    username: string
    password: string
    full_name: string
  }) => Promise<AuthUser>
  logout: () => Promise<void>
  refresh: () => Promise<void>
}

const AuthContext = createContext<AuthState | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null)
  const [loading, setLoading] = useState(true)

  const applyUser = useCallback((u: AuthUser | null) => {
    setUser(u)
    setCsrfToken(u?.csrf_token || '')
  }, [])

  const refresh = useCallback(async () => {
    try {
      const res = await api.get<{ success: boolean; data: AuthUser }>('/api/v1/auth/me')
      applyUser(res.data)
    } catch {
      applyUser(null)
      try {
        const csrf = await api.get<{ success: boolean; data: { csrf_token: string } }>(
          '/api/v1/auth/csrf',
        )
        setCsrfToken(csrf.data?.csrf_token || '')
      } catch {
        /* ignore */
      }
    }
  }, [applyUser])

  useEffect(() => {
    refresh().finally(() => setLoading(false))
  }, [refresh])

  const login = useCallback(
    async (email: string, password: string) => {
      const res = await api.post<{ success: boolean; data: AuthUser }>('/api/v1/auth/login', {
        email,
        password,
      })
      applyUser(res.data)
      return res.data
    },
    [applyUser],
  )

  const register = useCallback(
    async (input: {
      email: string
      username: string
      password: string
      full_name: string
    }) => {
      const res = await api.post<{ success: boolean; data: AuthUser }>('/api/v1/auth/register', input)
      applyUser(res.data)
      return res.data
    },
    [applyUser],
  )

  const logout = useCallback(async () => {
    await api.post('/api/v1/auth/logout')
    applyUser(null)
  }, [applyUser])

  const value = useMemo(
    () => ({ user, loading, login, register, logout, refresh }),
    [user, loading, login, register, logout, refresh],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
