import { useEffect, useMemo, useState, type FormEvent } from 'react'
import { MapContainer, Marker, TileLayer, useMapEvents } from 'react-leaflet'
import L from 'leaflet'
import { Navigate, useNavigate } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'
import { api } from '../lib/api'

type ProfessionOpt = { id: string; name: string; slug: string }

type OnboardingData = {
  completed?: boolean
  profile?: { full_name?: string; bio?: string; about?: string; profession?: string }
  professions?: ProfessionOpt[]
  suggested_tags?: string[]
  lat?: number
  lng?: number
}

const pinIcon = L.divIcon({
  className: 'leaflet-div-icon',
  html: `<div style="width:18px;height:18px;border-radius:50%;background:#facc15;border:3px solid #0b1224;box-shadow:0 2px 8px rgba(0,0,0,.45)"></div>`,
  iconSize: [18, 18],
  iconAnchor: [9, 9],
})

function LocationPicker({
  lat,
  lng,
  onPick,
}: {
  lat: number
  lng: number
  onPick: (lat: number, lng: number) => void
}) {
  useMapEvents({
    click(e) {
      onPick(e.latlng.lat, e.latlng.lng)
    },
  })
  return <Marker position={[lat, lng]} icon={pinIcon} />
}

export function OnboardingPage() {
  const { user, loading, refresh } = useAuth()
  const navigate = useNavigate()
  const [boot, setBoot] = useState(true)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [professions, setProfessions] = useState<ProfessionOpt[]>([])
  const [tags, setTags] = useState<string[]>([])
  const [selectedTags, setSelectedTags] = useState<string[]>([])
  const [fullName, setFullName] = useState('')
  const [professionId, setProfessionId] = useState('')
  const [about, setAbout] = useState('')
  const [bio, setBio] = useState('')
  const [lat, setLat] = useState(23.8103)
  const [lng, setLng] = useState(90.4125)
  const [mapKey, setMapKey] = useState(0)

  useEffect(() => {
    if (!user) return
    let cancelled = false
    ;(async () => {
      try {
        const res = await api.get<{ success: boolean; data: OnboardingData }>('/api/v1/me/onboarding')
        if (cancelled) return
        if (res.data?.completed) {
          navigate('/', { replace: true })
          return
        }
        setProfessions(res.data?.professions || [])
        setTags(res.data?.suggested_tags || [])
        setFullName(res.data?.profile?.full_name || user.username || '')
        setAbout(res.data?.profile?.about || '')
        setBio(res.data?.profile?.bio || '')
        if (typeof res.data?.lat === 'number') setLat(res.data.lat)
        if (typeof res.data?.lng === 'number') setLng(res.data.lng)
        setMapKey((k) => k + 1)
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : 'Failed to load onboarding')
      } finally {
        if (!cancelled) setBoot(false)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [user, navigate])

  const center = useMemo(() => [lat, lng] as [number, number], [lat, lng])

  if (loading || boot) {
    return <div className="flex min-h-full items-center justify-center text-slate-400">Loading…</div>
  }
  if (!user) return <Navigate to="/login?next=/onboarding" replace />
  if (user.onboarding_completed) return <Navigate to="/" replace />

  const useMyLocation = () => {
    if (!navigator.geolocation) {
      setError('Geolocation is not available in this browser')
      return
    }
    navigator.geolocation.getCurrentPosition(
      (pos) => {
        setLat(Number(pos.coords.latitude.toFixed(6)))
        setLng(Number(pos.coords.longitude.toFixed(6)))
        setMapKey((k) => k + 1)
        setError('')
      },
      () => setError('Could not read your location — click the map to drop a pin'),
      { enableHighAccuracy: true, timeout: 12000 },
    )
  }

  const toggleTag = (tag: string) => {
    setSelectedTags((prev) => (prev.includes(tag) ? prev.filter((t) => t !== tag) : [...prev, tag]))
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    setBusy(true)
    try {
      await api.post('/api/v1/me/onboarding', {
        full_name: fullName,
        about,
        bio,
        profession_id: professionId || undefined,
        tags: selectedTags,
        lat,
        lng,
      })
      await refresh()
      navigate('/', { replace: true })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not save profile')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="min-h-full bg-[radial-gradient(1000px_500px_at_80%_0%,rgba(250,204,21,0.1),transparent_55%),#050816] px-4 py-8">
      <form
        onSubmit={onSubmit}
        className="mx-auto max-w-xl space-y-5 rounded-2xl border border-white/10 bg-[#0b1224] p-6 shadow-xl"
      >
        <div>
          <h1 className="text-2xl font-bold">Complete your profile</h1>
          <p className="mt-1 text-sm text-slate-400">
            Set your permanent address on the map, tell people about you, then join the discover map.
          </p>
        </div>

        {error && (
          <div className="rounded-xl border border-red-400/30 bg-red-500/10 px-3 py-2 text-sm text-red-200">
            {error}
          </div>
        )}

        <label className="block text-sm">
          <span className="mb-1 block text-slate-300">Full name</span>
          <input
            required
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
            placeholder="What you do, what you’re looking for…"
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

        {tags.length > 0 && (
          <div>
            <div className="mb-2 text-sm text-slate-300">Tags</div>
            <div className="flex flex-wrap gap-2">
              {tags.map((tag) => {
                const on = selectedTags.includes(tag)
                return (
                  <button
                    key={tag}
                    type="button"
                    onClick={() => toggleTag(tag)}
                    className={`rounded-full border px-3 py-1.5 text-xs font-medium transition ${
                      on
                        ? 'border-yellow-400/60 bg-yellow-400/15 text-yellow-200'
                        : 'border-white/15 text-slate-300 hover:border-white/30'
                    }`}
                  >
                    {tag}
                  </button>
                )
              })}
            </div>
          </div>
        )}

        <div>
          <div className="mb-2 flex items-center justify-between gap-3">
            <div>
              <div className="text-sm text-slate-300">Your location on the map</div>
              <p className="text-xs text-slate-500">Click the map to drop a pin, or use GPS.</p>
            </div>
            <button
              type="button"
              onClick={useMyLocation}
              className="shrink-0 text-sm text-blue-300 underline"
            >
              Use my location
            </button>
          </div>
          <div className="h-56 overflow-hidden rounded-xl border border-white/10">
            <MapContainer
              key={mapKey}
              center={center}
              zoom={13}
              className="h-full w-full"
              scrollWheelZoom
            >
              <TileLayer
                attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OSM</a>'
                url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
              />
              <LocationPicker lat={lat} lng={lng} onPick={(a, b) => {
                setLat(Number(a.toFixed(6)))
                setLng(Number(b.toFixed(6)))
              }} />
            </MapContainer>
          </div>
          <p className="mt-2 text-xs text-slate-500">
            Pin: {lat.toFixed(5)}, {lng.toFixed(5)}
          </p>
        </div>

        <button
          type="submit"
          disabled={busy}
          className="w-full rounded-xl bg-yellow-400 py-2.5 text-sm font-bold text-slate-900 hover:bg-yellow-300 disabled:opacity-60"
        >
          {busy ? 'Saving…' : 'Save & show me on map'}
        </button>
      </form>
    </div>
  )
}
