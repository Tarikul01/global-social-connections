import { Link, Navigate, useNavigate, useParams } from 'react-router-dom'
import { useEffect, useState, type FormEvent } from 'react'
import { api } from '../lib/api'
import { useAuth } from '../auth/AuthContext'
import type { EventDetail, EventSummary } from '../lib/events'
import { EventLocationPicker } from '../components/EventLocationPicker'
import { EventTypePicker } from '../components/EventTypePicker'
import { isEventTypeId, type EventTypeId } from '../lib/eventTypes'

const DEFAULT_LAT = 23.8103
const DEFAULT_LNG = 90.4125

function toTimeInput(value?: string) {
  if (!value) return ''
  // "12:49:00" or "12:49:00+00" → "12:49"
  const m = value.trim().match(/^(\d{1,2}:\d{2})/)
  return m ? m[1].padStart(5, '0') : ''
}

export function EventEditPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const { user, loading: authLoading } = useAuth()
  const [boot, setBoot] = useState(true)
  const [forbidden, setForbidden] = useState(false)
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [category, setCategory] = useState<EventTypeId | ''>('meetup')
  const [eventDate, setEventDate] = useState('')
  const [startTime, setStartTime] = useState('')
  const [endTime, setEndTime] = useState('')
  const [locationName, setLocationName] = useState('')
  const [lat, setLat] = useState(DEFAULT_LAT)
  const [lng, setLng] = useState(DEFAULT_LNG)
  const [maxParticipants, setMaxParticipants] = useState(20)
  const [visibility, setVisibility] = useState<'public' | 'private'>('public')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!id || authLoading) return
    if (!user) {
      setBoot(false)
      return
    }
    let cancelled = false
    ;(async () => {
      setBoot(true)
      setError('')
      try {
        const res = await api.get<{ success: boolean; data: EventDetail }>(`/api/v1/events/${id}`)
        if (cancelled) return
        if (!res.data?.is_creator) {
          setForbidden(true)
          setBoot(false)
          return
        }
        const e = res.data.event
        setTitle(e.title || '')
        setDescription(e.description || '')
        setCategory(isEventTypeId(e.category) ? e.category : 'meetup')
        setEventDate(e.event_date || '')
        setStartTime(toTimeInput(e.start_time))
        setEndTime(toTimeInput(e.end_time))
        setLocationName(e.location_name || '')
        setLat(typeof e.lat === 'number' ? e.lat : DEFAULT_LAT)
        setLng(typeof e.lng === 'number' ? e.lng : DEFAULT_LNG)
        setMaxParticipants(e.max_participants || 20)
        setVisibility(e.visibility === 'private' ? 'private' : 'public')
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : 'Failed to load event')
      } finally {
        if (!cancelled) setBoot(false)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [id, user, authLoading])

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
    if (!id) return
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
      await api.put<{ success: boolean; data: EventSummary }>(`/api/v1/events/${id}`, {
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
      })
      navigate(`/e/${id}`, { replace: true })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Update failed')
    } finally {
      setBusy(false)
    }
  }

  if (authLoading || boot) {
    return <div className="flex min-h-full items-center justify-center text-slate-400">Loading…</div>
  }

  if (!user) {
    return <Navigate to={`/login?next=${encodeURIComponent(`/events/${id}/edit`)}`} replace />
  }

  if (forbidden) {
    return (
      <div className="flex min-h-full flex-col items-center justify-center gap-3 p-6">
        <p className="text-red-300">Only the event creator can edit this event.</p>
        <Link to={`/e/${id}`} className="text-blue-300 underline">
          Back to event
        </Link>
      </div>
    )
  }

  return (
    <div className="min-h-full bg-[radial-gradient(900px_500px_at_10%_0%,rgba(59,130,246,0.12),transparent_50%),#050816] px-4 py-8">
      <form onSubmit={onSubmit} className="mx-auto max-w-lg space-y-4 rounded-2xl border border-white/10 bg-[#0b1224] p-6">
        <Link to={`/e/${id}`} className="text-sm text-blue-300 underline">
          ← Event
        </Link>
        <div>
          <h1 className="text-2xl font-bold">Edit event</h1>
          <p className="text-sm text-slate-400">Update title, time, location, and who can join.</p>
        </div>

        {error && (
          <div className="rounded-xl border border-red-400/30 bg-red-500/10 px-3 py-2 text-sm text-red-200">
            {error}
          </div>
        )}

        <EventTypePicker value={category} onChange={setCategory} disabled={busy} required />

        <label className="block text-sm">
          <span className="mb-1 block text-slate-300">
            Event name <span className="text-red-400">*</span>
          </span>
          <input
            required
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            className="w-full rounded-xl border border-white/10 bg-[#111827] px-3 py-2.5 outline-none focus:border-yellow-400"
          />
        </label>

        <label className="block text-sm">
          <span className="mb-1 block text-slate-300">Description</span>
          <textarea
            rows={3}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            className="w-full rounded-xl border border-white/10 bg-[#111827] px-3 py-2.5 outline-none focus:border-yellow-400"
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
              setVisibility(e.target.value === 'private' ? 'private' : 'public')
            }
            className="w-full rounded-xl border border-white/10 bg-[#111827] px-3 py-2.5 outline-none focus:border-yellow-400"
            style={{ colorScheme: 'dark' }}
          >
            <option value="public">Public — anyone can request to join</option>
            <option value="private">Private — invite only</option>
          </select>
        </label>

        <div className="flex gap-2">
          <Link
            to={`/e/${id}`}
            className="flex-1 rounded-xl border border-white/15 py-2.5 text-center text-sm font-medium text-slate-200"
          >
            Cancel
          </Link>
          <button
            type="submit"
            disabled={busy}
            className="flex-1 rounded-xl bg-yellow-400 py-2.5 text-sm font-bold text-slate-900 hover:bg-yellow-300 disabled:opacity-60"
          >
            {busy ? 'Saving…' : 'Save changes'}
          </button>
        </div>
      </form>
    </div>
  )
}
