import { Link, useParams } from 'react-router-dom'
import { useEffect, useRef, useState } from 'react'
import { api } from '../lib/api'
import { useAuth } from '../auth/AuthContext'
import { UserAvatar } from '../components/UserAvatar'

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
  const [uploading, setUploading] = useState(false)
  const fileRef = useRef<HTMLInputElement>(null)
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

  const onPickPhoto = async (file: File | null) => {
    if (!file || !isOwn) return
    setUploading(true)
    setError('')
    try {
      const body = new FormData()
      body.append('photo', file)
      const res = await api.post<{ success: boolean; data: Profile }>('/api/v1/me/photo', body)
      setProfile(res.data)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Upload failed')
    } finally {
      setUploading(false)
      if (fileRef.current) fileRef.current.value = ''
    }
  }

  if (error && !profile) {
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
          <div className="relative">
            <UserAvatar
              name={profile.full_name || profile.username}
              photoUrl={profile.photo_url}
              online
              size={72}
              fontSize={22}
            />
            {isOwn && (
              <>
                <input
                  ref={fileRef}
                  type="file"
                  accept="image/jpeg,image/png,image/webp"
                  className="hidden"
                  onChange={(e) => void onPickPhoto(e.target.files?.[0] ?? null)}
                />
                <button
                  type="button"
                  disabled={uploading}
                  onClick={() => fileRef.current?.click()}
                  className="mt-2 w-full rounded-lg bg-white/10 px-2 py-1 text-[11px] font-semibold text-slate-200 hover:bg-white/15 disabled:opacity-50"
                >
                  {uploading ? 'Uploading…' : profile.photo_url ? 'Change photo' : 'Upload photo'}
                </button>
              </>
            )}
          </div>
          <div>
            <h1 className="text-xl font-bold">{profile.full_name || profile.username}</h1>
            <p className="text-slate-400">@{profile.username}</p>
            <p className="text-sm text-yellow-200">{profile.profession || 'Professional'}</p>
          </div>
        </div>
        {error && <p className="mt-3 text-sm text-red-300">{error}</p>}
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
