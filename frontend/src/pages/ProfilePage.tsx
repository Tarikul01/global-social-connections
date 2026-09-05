import { Link, useParams } from 'react-router-dom'
import { useEffect, useRef, useState, type FormEvent } from 'react'
import { Pencil, X } from 'lucide-react'
import { api } from '../lib/api'
import { useAuth } from '../auth/AuthContext'
import { UserAvatar } from '../components/UserAvatar'
import { EventLocationPicker } from '../components/EventLocationPicker'

type ProfessionOpt = { id: string; name: string; slug: string }

type Profile = {
  username: string
  full_name: string
  email?: string
  profession?: string
  profession_id?: string
  bio?: string
  about?: string
  photo_url?: string
  company?: string
  job_title?: string
  experience_years?: number
  availability?: string
  gender?: string
  birthday?: string
  birthday_display?: string
  tags?: string[]
  skills?: string[]
  social_links?: Record<string, string>
  lat?: number
  lng?: number
  followers?: number
  following?: number
  views?: number
  verified?: boolean
  is_own?: boolean
  professions?: ProfessionOpt[]
  suggested_tags?: string[]
  suggested_skills?: string[]
  social_platforms?: string[]
}

const DEFAULT_LAT = 23.8103
const DEFAULT_LNG = 90.4125

const DEFAULT_SOCIAL_PLATFORMS = [
  'linkedin',
  'github',
  'twitter',
  'youtube',
  'instagram',
  'facebook',
  'website',
]

const GENDER_OPTIONS = [
  { value: '', label: 'Prefer not to say' },
  { value: 'male', label: 'Male' },
  { value: 'female', label: 'Female' },
  { value: 'other', label: 'Other' },
]

export function ProfilePage() {
  const { username } = useParams()
  const { user, refresh } = useAuth()
  const [profile, setProfile] = useState<Profile | null>(null)
  const [error, setError] = useState('')
  const [uploading, setUploading] = useState(false)
  const [editing, setEditing] = useState(false)
  const [saving, setSaving] = useState(false)
  const fileRef = useRef<HTMLInputElement>(null)

  const isOwn = !username || (!!user && username === user.username)

  const [fullName, setFullName] = useState('')
  const [bio, setBio] = useState('')
  const [about, setAbout] = useState('')
  const [company, setCompany] = useState('')
  const [jobTitle, setJobTitle] = useState('')
  const [professionId, setProfessionId] = useState('')
  const [experienceYears, setExperienceYears] = useState('')
  const [availability, setAvailability] = useState('available')
  const [gender, setGender] = useState('')
  const [birthday, setBirthday] = useState('')
  const [selectedTags, setSelectedTags] = useState<string[]>([])
  const [selectedSkills, setSelectedSkills] = useState<string[]>([])
  const [customSkill, setCustomSkill] = useState('')
  const [socialLinks, setSocialLinks] = useState<Record<string, string>>({})
  const [lat, setLat] = useState(DEFAULT_LAT)
  const [lng, setLng] = useState(DEFAULT_LNG)

  const load = async () => {
    const path = isOwn
      ? '/api/v1/me/profile'
      : `/api/v1/profiles/${encodeURIComponent(username!)}`
    const res = await api.get<{ success: boolean; data: Profile }>(path)
    setProfile(res.data)
    return res.data
  }

  useEffect(() => {
    let cancelled = false
    setError('')
    setEditing(false)
    void load()
      .then((p) => {
        if (cancelled) return
        hydrateForm(p)
      })
      .catch((e) => {
        if (!cancelled) setError(e instanceof Error ? e.message : 'Failed to load')
      })
    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [username, isOwn, user?.username])

  function hydrateForm(p: Profile) {
    setFullName(p.full_name || '')
    setBio(p.bio || '')
    setAbout(p.about || '')
    setCompany(p.company || '')
    setJobTitle(p.job_title || '')
    setProfessionId(p.profession_id || '')
    setExperienceYears(
      typeof p.experience_years === 'number' ? String(p.experience_years) : '',
    )
    setAvailability(p.availability || 'available')
    setGender(p.gender || '')
    setBirthday(p.birthday || '')
    setSelectedTags(p.tags || [])
    setSelectedSkills(p.skills || [])
    setSocialLinks(p.social_links || {})
    setLat(typeof p.lat === 'number' && p.lat !== 0 ? p.lat : DEFAULT_LAT)
    setLng(typeof p.lng === 'number' && p.lng !== 0 ? p.lng : DEFAULT_LNG)
  }

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

  const toggleTag = (name: string) => {
    setSelectedTags((prev) =>
      prev.includes(name) ? prev.filter((t) => t !== name) : [...prev, name],
    )
  }

  const toggleSkill = (name: string) => {
    setSelectedSkills((prev) =>
      prev.includes(name) ? prev.filter((t) => t !== name) : [...prev, name],
    )
  }

  const onSave = async (e: FormEvent) => {
    e.preventDefault()
    if (!isOwn) return
    setSaving(true)
    setError('')
    try {
      const exp =
        experienceYears.trim() === '' ? undefined : Number.parseInt(experienceYears, 10)
      const res = await api.put<{ success: boolean; data: Profile }>('/api/v1/me/profile', {
        full_name: fullName.trim().slice(0, 100),
        bio: bio.trim(),
        about: about.trim(),
        company: company.trim().slice(0, 100),
        job_title: jobTitle.trim().slice(0, 100),
        profession_id: professionId || undefined,
        experience_years: Number.isFinite(exp) && (exp as number) >= 0 ? exp : undefined,
        availability,
        gender: gender.trim().slice(0, 30),
        birthday: birthday || undefined,
        tags: selectedTags,
        skills: selectedSkills,
        social_links: socialLinks,
        lat,
        lng,
      })
      setProfile(res.data)
      hydrateForm(res.data)
      setEditing(false)
      void refresh()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed')
    } finally {
      setSaving(false)
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

  const professions = profile.professions || []
  const suggestedTags = profile.suggested_tags || []
  const suggestedSkills = profile.suggested_skills || []
  const platforms =
    profile.social_platforms && profile.social_platforms.length > 0
      ? profile.social_platforms
      : DEFAULT_SOCIAL_PLATFORMS

  return (
    <div className="min-h-full bg-[radial-gradient(900px_500px_at_90%_0%,rgba(250,204,21,0.08),transparent_50%),#050816] px-4 py-8">
      <div className="mx-auto max-w-2xl space-y-4 rounded-2xl border border-white/10 bg-[#0b1224] p-6">
        <div className="flex items-center justify-between gap-3">
          <Link to="/" className="text-sm text-blue-300 underline">
            ← Map
          </Link>
          {isOwn && !editing && (
            <button
              type="button"
              onClick={() => {
                hydrateForm(profile)
                setEditing(true)
              }}
              className="inline-flex items-center gap-1.5 rounded-xl bg-yellow-400 px-3 py-1.5 text-xs font-bold text-slate-900 hover:bg-yellow-300"
            >
              <Pencil size={14} /> Edit profile
            </button>
          )}
          {isOwn && editing && (
            <button
              type="button"
              onClick={() => {
                setEditing(false)
                hydrateForm(profile)
                setError('')
              }}
              className="inline-flex items-center gap-1.5 rounded-xl border border-white/15 px-3 py-1.5 text-xs font-medium text-slate-200"
            >
              <X size={14} /> Cancel
            </button>
          )}
        </div>

        {!editing && (
          <>
            <div className="flex items-start gap-4">
              <div className="relative shrink-0">
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
                      accept="image/*"
                      className="hidden"
                      onChange={(e) => void onPickPhoto(e.target.files?.[0] ?? null)}
                    />
                    <button
                      type="button"
                      disabled={uploading}
                      onClick={() => fileRef.current?.click()}
                      className="mt-2 w-full rounded-lg bg-white/10 px-2 py-1 text-[11px] font-semibold text-slate-200 hover:bg-white/15 disabled:opacity-50"
                    >
                      {uploading ? '…' : 'Photo'}
                    </button>
                  </>
                )}
              </div>
              <div className="min-w-0 flex-1">
                <h1 className="truncate text-xl font-bold text-white">
                  {profile.full_name || profile.username}
                </h1>
                <p className="text-sm text-slate-400">@{profile.username}</p>
                {profile.profession && (
                  <p className="mt-1 text-sm text-yellow-200/90">{profile.profession}</p>
                )}
                {(typeof profile.followers === 'number' || typeof profile.following === 'number') && (
                  <p className="mt-1 text-xs text-slate-400">
                    {profile.followers ?? 0} followers · {profile.following ?? 0} following
                    {isOwn && typeof profile.views === 'number' ? ` · ${profile.views} views` : ''}
                  </p>
                )}
              </div>
            </div>

            {error && <p className="text-sm text-red-300">{error}</p>}

            {(profile.job_title || profile.company) && (
              <p className="text-sm text-slate-300">
                {[profile.job_title, profile.company].filter(Boolean).join(' · ')}
              </p>
            )}
            {profile.about && <p className="text-sm leading-relaxed text-slate-300">{profile.about}</p>}
            {profile.bio && <p className="text-sm leading-relaxed text-slate-300">{profile.bio}</p>}

            {isOwn && (
              <div className="space-y-3 border-t border-white/10 pt-4 text-sm text-slate-300">
                {profile.email && (
                  <p>
                    <span className="text-slate-500">Email · </span>
                    {profile.email}
                  </p>
                )}
                {profile.gender && (
                  <p>
                    <span className="text-slate-500">Gender · </span>
                    {profile.gender}
                  </p>
                )}
                {(profile.birthday_display || profile.birthday) && (
                  <p>
                    <span className="text-slate-500">Birthday · </span>
                    {profile.birthday_display || profile.birthday}
                  </p>
                )}
                {typeof profile.experience_years === 'number' && (
                  <p>
                    <span className="text-slate-500">Experience · </span>
                    {profile.experience_years} years
                  </p>
                )}
                {profile.availability && (
                  <p>
                    <span className="text-slate-500">Availability · </span>
                    {profile.availability}
                  </p>
                )}
                {profile.tags && profile.tags.length > 0 && (
                  <div>
                    <span className="text-slate-500">Tags · </span>
                    <span className="inline-flex flex-wrap gap-1">
                      {profile.tags.map((t) => (
                        <span
                          key={t}
                          className="rounded-full border border-white/15 bg-white/5 px-2 py-0.5 text-xs"
                        >
                          {t}
                        </span>
                      ))}
                    </span>
                  </div>
                )}
                {profile.skills && profile.skills.length > 0 && (
                  <div>
                    <span className="text-slate-500">Skills · </span>
                    <span className="inline-flex flex-wrap gap-1">
                      {profile.skills.map((t) => (
                        <span
                          key={t}
                          className="rounded-full border border-yellow-400/30 bg-yellow-400/10 px-2 py-0.5 text-xs text-yellow-100"
                        >
                          {t}
                        </span>
                      ))}
                    </span>
                  </div>
                )}
                {profile.social_links &&
                  Object.entries(profile.social_links).filter(([, url]) => url).length > 0 && (
                    <div className="space-y-1">
                      <span className="text-slate-500">Social</span>
                      {Object.entries(profile.social_links)
                        .filter(([, url]) => url)
                        .map(([platform, url]) => (
                          <a
                            key={platform}
                            href={url}
                            target="_blank"
                            rel="noreferrer"
                            className="block text-blue-300 underline"
                          >
                            {platform}
                          </a>
                        ))}
                    </div>
                  )}
              </div>
            )}

            {!isOwn && profile.tags && profile.tags.length > 0 && (
              <div className="flex flex-wrap gap-1.5">
                {profile.tags.map((t) => (
                  <span
                    key={t}
                    className="rounded-full border border-white/15 bg-white/5 px-2.5 py-0.5 text-xs text-slate-300"
                  >
                    {t}
                  </span>
                ))}
              </div>
            )}
          </>
        )}

        {editing && isOwn && (
          <form onSubmit={onSave} className="space-y-4 text-white">
            {error && <p className="text-sm text-red-300">{error}</p>}

            <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">
              Profile (same as registration)
            </p>

            <label className="block text-sm">
              <span className="mb-1 block text-slate-300">Full name</span>
              <input
                required
                maxLength={100}
                value={fullName}
                onChange={(e) => setFullName(e.target.value)}
                className="w-full rounded-xl border border-white/10 bg-[#111827] px-3 py-2.5 outline-none focus:border-yellow-400"
              />
            </label>

            <label className="block text-sm">
              <span className="mb-1 block text-slate-300">Profession</span>
              <select
                value={professionId}
                onChange={(e) => setProfessionId(e.target.value)}
                className="w-full rounded-xl border border-white/10 bg-[#111827] px-3 py-2.5 outline-none focus:border-yellow-400"
                style={{ colorScheme: 'dark' }}
              >
                <option value="">Select…</option>
                {professions.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
                  </option>
                ))}
              </select>
            </label>

            <label className="block text-sm">
              <span className="mb-1 block text-slate-300">About you</span>
              <textarea
                rows={3}
                value={about}
                onChange={(e) => setAbout(e.target.value)}
                placeholder="What you do…"
                className="w-full rounded-xl border border-white/10 bg-[#111827] px-3 py-2.5 outline-none focus:border-yellow-400"
              />
            </label>

            <label className="block text-sm">
              <span className="mb-1 block text-slate-300">Short bio</span>
              <textarea
                rows={2}
                value={bio}
                onChange={(e) => setBio(e.target.value)}
                className="w-full rounded-xl border border-white/10 bg-[#111827] px-3 py-2.5 outline-none focus:border-yellow-400"
              />
            </label>

            <div className="text-sm">
              <span className="mb-2 block text-slate-300">Tags</span>
              <div className="flex flex-wrap gap-2">
                {Array.from(new Set([...suggestedTags, ...selectedTags])).map((t) => {
                  const on = selectedTags.includes(t)
                  return (
                    <button
                      key={t}
                      type="button"
                      onClick={() => toggleTag(t)}
                      className={`rounded-full border px-2.5 py-1 text-xs ${
                        on
                          ? 'border-yellow-400/50 bg-yellow-400/15 text-yellow-100'
                          : 'border-white/15 text-slate-300'
                      }`}
                    >
                      {t}
                    </button>
                  )
                })}
              </div>
            </div>

            <div className="text-sm">
              <span className="mb-2 block text-slate-300">Your location on the map</span>
              <EventLocationPicker
                lat={lat}
                lng={lng}
                onChange={(a, b) => {
                  setLat(a)
                  setLng(b)
                }}
                onUseMyLocation={() => {
                  if (!navigator.geolocation) return
                  navigator.geolocation.getCurrentPosition((pos) => {
                    setLat(Number(pos.coords.latitude.toFixed(6)))
                    setLng(Number(pos.coords.longitude.toFixed(6)))
                  })
                }}
              />
            </div>

            <p className="pt-2 text-xs font-semibold uppercase tracking-wide text-slate-500">
              Extra details
            </p>

            <div className="grid gap-3 sm:grid-cols-2">
              <label className="block text-sm">
                <span className="mb-1 block text-slate-300">Job title</span>
                <input
                  maxLength={100}
                  value={jobTitle}
                  onChange={(e) => setJobTitle(e.target.value)}
                  className="w-full rounded-xl border border-white/10 bg-[#111827] px-3 py-2.5 outline-none focus:border-yellow-400"
                />
              </label>
              <label className="block text-sm">
                <span className="mb-1 block text-slate-300">Company</span>
                <input
                  maxLength={100}
                  value={company}
                  onChange={(e) => setCompany(e.target.value)}
                  className="w-full rounded-xl border border-white/10 bg-[#111827] px-3 py-2.5 outline-none focus:border-yellow-400"
                />
              </label>
            </div>

            <div className="grid gap-3 sm:grid-cols-2">
              <label className="block text-sm">
                <span className="mb-1 block text-slate-300">Experience (years)</span>
                <input
                  type="number"
                  min={0}
                  value={experienceYears}
                  onChange={(e) => setExperienceYears(e.target.value)}
                  className="w-full rounded-xl border border-white/10 bg-[#111827] px-3 py-2.5 outline-none focus:border-yellow-400"
                />
              </label>
              <label className="block text-sm">
                <span className="mb-1 block text-slate-300">Availability</span>
                <select
                  value={availability}
                  onChange={(e) => setAvailability(e.target.value)}
                  className="w-full rounded-xl border border-white/10 bg-[#111827] px-3 py-2.5 outline-none focus:border-yellow-400"
                  style={{ colorScheme: 'dark' }}
                >
                  <option value="available">available</option>
                  <option value="busy">busy</option>
                  <option value="away">away</option>
                  <option value="offline">offline</option>
                </select>
              </label>
            </div>

            <div className="grid gap-3 sm:grid-cols-2">
              <label className="block text-sm">
                <span className="mb-1 block text-slate-300">Gender</span>
                <select
                  value={gender}
                  onChange={(e) => setGender(e.target.value)}
                  className="w-full rounded-xl border border-white/10 bg-[#111827] px-3 py-2.5 outline-none focus:border-yellow-400"
                  style={{ colorScheme: 'dark' }}
                >
                  {GENDER_OPTIONS.map((o) => (
                    <option key={o.value || 'none'} value={o.value}>
                      {o.label}
                    </option>
                  ))}
                </select>
              </label>
              <label className="block text-sm">
                <span className="mb-1 block text-slate-300">Birthday</span>
                <input
                  type="date"
                  value={birthday}
                  onChange={(e) => setBirthday(e.target.value)}
                  className="w-full rounded-xl border border-white/10 bg-[#111827] px-3 py-2.5 outline-none focus:border-yellow-400"
                />
              </label>
            </div>

            <div className="text-sm">
              <span className="mb-2 block text-slate-300">Skills</span>
              <div className="mb-2 flex flex-wrap gap-2">
                {Array.from(new Set([...suggestedSkills, ...selectedSkills])).map((t) => {
                  const on = selectedSkills.includes(t)
                  return (
                    <button
                      key={t}
                      type="button"
                      onClick={() => toggleSkill(t)}
                      className={`rounded-full border px-2.5 py-1 text-xs ${
                        on
                          ? 'border-yellow-400/50 bg-yellow-400/15 text-yellow-100'
                          : 'border-white/15 text-slate-300'
                      }`}
                    >
                      {t}
                    </button>
                  )
                })}
              </div>
              <div className="flex gap-2">
                <input
                  value={customSkill}
                  onChange={(e) => setCustomSkill(e.target.value)}
                  placeholder="Add custom skill"
                  maxLength={100}
                  className="min-w-0 flex-1 rounded-xl border border-white/10 bg-[#111827] px-3 py-2 text-sm outline-none focus:border-yellow-400"
                />
                <button
                  type="button"
                  onClick={() => {
                    const v = customSkill.trim()
                    if (!v) return
                    if (!selectedSkills.includes(v)) setSelectedSkills((s) => [...s, v])
                    setCustomSkill('')
                  }}
                  className="rounded-xl border border-white/15 px-3 text-xs font-medium"
                >
                  Add
                </button>
              </div>
            </div>

            <div className="space-y-2 text-sm">
              <span className="block text-slate-300">Social links</span>
              {platforms.map((platform) => (
                <label key={platform} className="block">
                  <span className="mb-1 block text-xs uppercase tracking-wide text-slate-500">
                    {platform}
                  </span>
                  <input
                    maxLength={500}
                    value={socialLinks[platform] || ''}
                    onChange={(e) =>
                      setSocialLinks((prev) => ({ ...prev, [platform]: e.target.value }))
                    }
                    className="w-full rounded-xl border border-white/10 bg-[#111827] px-3 py-2 outline-none focus:border-yellow-400"
                    placeholder={`https://${platform === 'website' ? 'yoursite.com' : `${platform}.com/...`}`}
                  />
                </label>
              ))}
            </div>

            <button
              type="submit"
              disabled={saving}
              className="w-full rounded-xl bg-yellow-400 py-2.5 text-sm font-bold text-slate-900 hover:bg-yellow-300 disabled:opacity-60"
            >
              {saving ? 'Saving…' : 'Save profile'}
            </button>
          </form>
        )}
      </div>
    </div>
  )
}
