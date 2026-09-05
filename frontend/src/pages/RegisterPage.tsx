import { useState, type FormEvent } from 'react'
import { Link, Navigate, useNavigate } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'

export function RegisterPage() {
  const { user, loading, register } = useAuth()
  const navigate = useNavigate()
  const [form, setForm] = useState({
    email: '',
    username: '',
    full_name: '',
    password: '',
  })
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  if (!loading && user) {
    return <Navigate to={user.onboarding_completed ? '/' : '/onboarding'} replace />
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    setBusy(true)
    try {
      await register(form)
      navigate('/onboarding', { replace: true })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Registration failed')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex min-h-full items-center justify-center bg-[radial-gradient(1200px_600px_at_10%_-10%,rgba(59,130,246,0.18),transparent_55%),#050816] px-4 py-10">
      <form
        onSubmit={onSubmit}
        className="w-full max-w-md rounded-2xl border border-white/10 bg-[#0b1224] p-8 shadow-xl"
      >
        <h1 className="mb-1 text-2xl font-bold">Join the map</h1>
        <p className="mb-6 text-sm text-slate-400">Create your FindNeighbour account</p>
        {error && (
          <div className="mb-4 rounded-xl border border-red-400/30 bg-red-500/10 px-3 py-2 text-sm text-red-200">
            {error}
          </div>
        )}
        {(
          [
            ['full_name', 'Full name', 'text'],
            ['username', 'Username', 'text'],
            ['email', 'Email', 'email'],
            ['password', 'Password', 'password'],
          ] as const
        ).map(([key, label, type]) => (
          <label key={key} className="mb-3 block text-sm">
            <span className="mb-1 block text-slate-300">{label}</span>
            <input
              type={type}
              required
              value={form[key]}
              onChange={(e) => setForm((f) => ({ ...f, [key]: e.target.value }))}
              className="w-full rounded-xl border border-white/10 bg-[#111827] px-3 py-2.5 outline-none focus:border-blue-400"
            />
          </label>
        ))}
        <button
          type="submit"
          disabled={busy}
          className="mt-2 w-full rounded-xl bg-yellow-400 py-2.5 font-bold text-slate-900 hover:bg-yellow-300 disabled:opacity-60"
        >
          {busy ? 'Creating…' : 'Create account'}
        </button>
        <p className="mt-4 text-center text-sm text-slate-400">
          Already have an account?{' '}
          <Link to="/login" className="text-blue-300 underline">
            Login
          </Link>
        </p>
      </form>
    </div>
  )
}
