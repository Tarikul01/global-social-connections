import { Link, useNavigate } from 'react-router-dom'
import { useEffect, useState, type FormEvent } from 'react'
import { api } from '../lib/api'
import type { EventSummary } from '../lib/events'
import { EventLocationPicker } from '../components/EventLocationPicker'
import { UserInviteSuggest, type SuggestUser } from '../components/UserInviteSuggest'
import { EventTypePicker } from '../components/EventTypePicker'
import type { EventTypeId } from '../lib/eventTypes'

const DEFAULT_LAT = 23.8103
const DEFAULT_LNG = 90.4125
const MAX_EVENTS = 2

export function EventCreatePage() {
  const navigate = useNavigate()
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [category, setCategory] = useState<EventTypeId | ''>('')
  const [eventDate, setEventDate] = useState('')
  const [startTime, setStartTime] = useState('')
  const [endTime, setEndTime] = useState('')
  const [locationName, setLocationName] = useState('')
  const [lat, setLat] = useState(DEFAULT_LAT)
  const [lng, setLng] = useState(DEFAULT_LNG)
  const [maxParticipants, setMaxParticipants] = useState(20)
  const [visibility, setVisibility] = useState<'public' | 'private' | ''>('')
  const [invitees, setInvitees] = useState<SuggestUser[]>([])
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [atLimit, setAtLimit] = useState(false)

  useEffect(() => {
    void api
      .get<{ success: boolean; meta?: { created_count?: number } }>('/api/v1/events/hosted')
      .then((r) => {
        if ((r.meta?.created_count ?? 0) >= MAX_EVENTS) setAtLimit(true)
      })
      .catch(() => {})
  }, [])

  const useMyLocation = () => {
    if (!navigator.geolocation) {
      setError('Geolocation is not available')
      return
    }
    navigator.geolocation.getCurrentPosition(
      (pos) => {
        setLat(Number(pos.coords.latitude.toFixed(6)))
        setLng(Number(pos.coords.longitude.toFixed(6)))
        setError('')
      },
      () => setError('Could not read your location'),
      { enableHighAccuracy: true, timeout: 10000 },
    )
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')

    if (!category) {
      setError('Event type is required')
      return
    }
    if (!title.trim()) {
      setError('Event name is required')
      return
    }
    if (!eventDate) {
      setError('Date is required')
      return
    }
    if (!locationName.trim()) {
      setError('Location is required')
      return
    }
    if (visibility !== 'public' && visibility !== 'private') {
      setError('Please choose public or private')
      return
    }
    if (!startTime) {
      setError('Start time is required')
      return
    }

    setBusy(true)
    try {
      const invite_usernames = invitees.map((u) => u.username).join(', ')
      const res = await api.post<{ success: boolean; data: EventSummary }>('/api/v1/events', {
        title: title.trim(),
        description,
        category,
        event_date: eventDate,
        start_time: startTime,
        end_time: endTime || undefined,
        location_name: locationName.trim(),
        lat,
        lng,
        max_participants: maxParticipants,
        visibility,
        invite_usernames,
      })
      navigate(`/e/${res.data.id}`, { replace: true })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Create failed')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="min-h-full bg-[radial-gradient(900px_500px_at_10%_0%,rgba(59,130,246,0.12),transparent_50%),#050816] px-4 py-8">
      <form onSubmit={onSubmit} className="mx-auto max-w-lg space-y-4 rounded-2xl border border-white/10 bg-[#0b1224] p-6">
        <Link to="/events" className="text-sm text-blue-300 underline">
          ← Events
        </Link>
        <div>
          <h1 className="text-2xl font-bold">Create event</h1>
          <p className="text-sm text-slate-400">Pick a title, time, and meetup spot.</p>
          <p className="mt-1 text-xs text-slate-500">You can host up to 2 events at a time.</p>
        </div>

        {atLimit && (
          <div className="rounded-xl border border-amber-400/30 bg-amber-500/10 px-3 py-2 text-sm text-amber-100">
            You already have 2 events. Delete one from the event page before creating another.{' '}
            <Link to="/events" className="underline">
              Back to events
            </Link>
          </div>
        )}

        {error && (
          <div className="rounded-xl border border-red-400/30 bg-red-500/10 px-3 py-2 text-sm text-red-200">
            {error}
          </div>
        )}

        <EventTypePicker value={category} onChange={setCategory} disabled={busy || atLimit} required />

        <label className="block text-sm">
          <span className="mb-1 block text-slate-300">
            Event name <span className="text-red-400">*</span>
          </span>
          <input
            required
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            className="w-full rounded-xl border border-white/10 bg-[#111827] px-3 py-2.5 outline-none focus:border-yellow-400"
            placeholder="Coffee meetup"
          />
        </label>

        <label className="block text-sm">
          <span className="mb-1 block text-slate-300">Description</span>
          <textarea
            rows={3}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            className="w-full rounded-xl border border-white/10 bg-[#111827] px-3 py-2.5 outline-none focus:border-yellow-400"
            placeholder="What is this about?"
          />
        </label>

        <label className="block text-sm">
          <span className="mb-1 block text-slate-300">
            Date <span className="text-red-400">*</span>
          </span>
          <input
            type="date"
            required
            value={eventDate}
            onChange={(e) => setEventDate(e.target.value)}
            className="w-full rounded-xl border border-white/10 bg-[#111827] px-3 py-2.5 outline-none focus:border-yellow-400"
          />
        </label>

        <div className="grid grid-cols-2 gap-3">
          <label className="block text-sm">
            <span className="mb-1 block text-slate-300">
              Start <span className="text-red-400">*</span>
            </span>
            <input
              type="time"
              required
              value={startTime}
              onChange={(e) => setStartTime(e.target.value)}
              className="w-full rounded-xl border border-white/10 bg-[#111827] px-3 py-2.5 outline-none focus:border-yellow-400"
            />
          </label>
          <label className="block text-sm">
            <span className="mb-1 block text-slate-300">End</span>
            <input
              type="time"
              value={endTime}
              onChange={(e) => setEndTime(e.target.value)}
              className="w-full rounded-xl border border-white/10 bg-[#111827] px-3 py-2.5 outline-none focus:border-yellow-400"
            />
          </label>
        </div>

        <label className="block text-sm">
          <span className="mb-1 block text-slate-300">
            Location <span className="text-red-400">*</span>
          </span>
          <input
            required
            value={locationName}
            onChange={(e) => setLocationName(e.target.value)}
            className="w-full rounded-xl border border-white/10 bg-[#111827] px-3 py-2.5 outline-none focus:border-yellow-400"
            placeholder="Cafe / park / venue"
          />
        </label>

        <EventLocationPicker
          lat={lat}
          lng={lng}
          onChange={(a, b) => {
            setLat(a)
            setLng(b)
          }}
          onUseMyLocation={useMyLocation}
        />

        <label className="block text-sm">
          <span className="mb-1 block text-slate-300">Max participants</span>
          <input
            type="number"
            min={1}
            value={maxParticipants}
            onChange={(e) => setMaxParticipants(Number(e.target.value) || 20)}
            className="w-full rounded-xl border border-white/10 bg-[#111827] px-3 py-2.5 outline-none focus:border-yellow-400"
          />
        </label>

        <label className="block text-sm">
          <span className="mb-1 block text-slate-300">
            Who can join? <span className="text-red-400">*</span>
          </span>
          <select
            required
            value={visibility}
            onChange={(e) =>
              setVisibility(e.target.value === 'private' ? 'private' : e.target.value === 'public' ? 'public' : '')
            }
            className="w-full rounded-xl border border-white/10 bg-[#111827] px-3 py-2.5 outline-none focus:border-yellow-400"
            style={{ colorScheme: 'dark' }}
          >
            <option value="" disabled>
              Select public or private…
            </option>
            <option value="public">Public — anyone can request to join</option>
            <option value="private">Private — invite only</option>
          </select>
        </label>

        <div className="text-sm">
          <span className="mb-1 block text-slate-300">Invite people (optional)</span>
          <UserInviteSuggest
            searchPath="/api/v1/users/search"
            selected={invitees}
            onChange={setInvitees}
            placeholder="Type username, email, or name"
          />
        </div>

        <button
          type="submit"
          disabled={busy || atLimit}
          className="w-full rounded-xl bg-yellow-400 py-2.5 text-sm font-bold text-slate-900 hover:bg-yellow-300 disabled:opacity-60"
        >
          {busy ? 'Creating…' : atLimit ? 'Limit reached' : 'Create event'}
        </button>
      </form>
    </div>
  )
}
