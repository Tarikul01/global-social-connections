import { Link, useParams } from 'react-router-dom'
import { useEffect, useState } from 'react'
import { api } from '../lib/api'
import { useAuth } from '../auth/AuthContext'

type Profile = {
  username: string
  full_name: string
  profession?: string
  bio?: string
  photo_url?: string
  company?: string
  job_title?: string
}

export function ProfilePage() {
  const { username } = useParams()
  const { user } = useAuth()
  const [profile, setProfile] = useState<Profile | null>(null)
  const [error, setError] = useState('')
  const isOwn = !username || username === user?.username

  useEffect(() => {
    const path = isOwn
      ? '/api/v1/me/profile'
      : `/api/v1/profiles/${encodeURIComponent(username!)}`
    api
      .get<{ success: boolean; data: Profile }>(path)
      .then((r) => setProfile(r.data))
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load'))
  }, [username, isOwn])

  if (error) {
    return (
      <div className="flex min-h-full flex-col items-center justify-center gap-3 p-6">
        <p className="text-red-300">{error}</p>
        <Link to="/login" className="text-blue-300 underline">
          Login required for some profiles
        </Link>
        <Link to="/" className="text-slate-400 underline">
          Back to map
        </Link>
      </div>
    )
  }

  if (!profile) {
    return <div className="flex min-h-full items-center justify-center text-slate-400">Loading…</div>
  }

  return (
    <div className="min-h-full bg-[radial-gradient(900px_500px_at_90%_0%,rgba(250,204,21,0.08),transparent_50%),#050816] px-4 py-8">
      <div className="mx-auto max-w-lg rounded-2xl border border-white/10 bg-[#0b1224] p-6">
        <Link to="/" className="text-sm text-blue-300 underline">
          ← Map
        </Link>
        <div className="mt-4 flex items-center gap-4">
          <div className="disc-marker online" style={{ width: 72, height: 72, fontSize: 20 }}>
            {profile.photo_url ? <img src={profile.photo_url} alt="" /> : profile.full_name?.[0] || '?'}
          </div>
          <div>
            <h1 className="text-xl font-bold">{profile.full_name || profile.username}</h1>
            <p className="text-slate-400">@{profile.username}</p>
            <p className="text-sm text-yellow-200">{profile.profession || 'Professional'}</p>
          </div>
        </div>
        {(profile.job_title || profile.company) && (
          <p className="mt-4 text-sm text-slate-300">
            {[profile.job_title, profile.company].filter(Boolean).join(' · ')}
          </p>
        )}
        {profile.bio && <p className="mt-3 text-sm leading-relaxed text-slate-300">{profile.bio}</p>}
      </div>
    </div>
  )
}
