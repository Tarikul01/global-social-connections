import { Link, useNavigate } from 'react-router-dom'
import { useState, type FormEvent } from 'react'
import { api } from '../lib/api'
import type { EventSummary } from '../lib/events'

const DEFAULT_LAT = 23.8103
const DEFAULT_LNG = 90.4125

export function EventCreatePage() {
  const navigate = useNavigate()
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [eventDate, setEventDate] = useState('')
  const [startTime, setStartTime] = useState('')
  const [endTime, setEndTime] = useState('')
  const [locationName, setLocationName] = useState('')
  const [lat, setLat] = useState(DEFAULT_LAT)
  const [lng, setLng] = useState(DEFAULT_LNG)
  const [maxParticipants, setMaxParticipants] = useState(20)
  const [visibility, setVisibility] = useState<'public' | 'private'>('public')
  const [inviteUsernames, setInviteUsernames] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const useMyLocation = () => {
    if (!navigator.geolocation) {
      setError('Geolocation is not available')
      return
    }
    navigator.geolocation.getCurrentPosition(
      (pos) => {
        setLat(Number(pos.coords.latitude.toFixed(6)))
        setLng(Number(pos.coords.longitude.toFixed(6)))
      },
      () => setError('Could not read your location'),
      { enableHighAccuracy: true, timeout: 10000 },
    )
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    setBusy(true)
    try {
      const res = await api.post<{ success: boolean; data: EventSummary }>('/api/v1/events', {
        title,
        description,
        event_date: eventDate,
        start_time: startTime,
        end_time: endTime || undefined,
        location_name: locationName,
        lat,
        lng,
        max_participants: maxParticipants,
        visibility,
        invite_usernames: inviteUsernames,
      })
      navigate(`/events/${res.data.id}`, { replace: true })
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
        </div>

        {error && (
          <div className="rounded-xl border border-red-400/30 bg-red-500/10 px-3 py-2 text-sm text-red-200">
            {error}
          </div>
        )}

        <label className="block text-sm">
          <span className="mb-1 block text-slate-300">Title</span>
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
          <span className="mb-1 block text-slate-300">Date</span>
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
            <span className="mb-1 block text-slate-300">Start</span>
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
          <span className="mb-1 block text-slate-300">Location name</span>
          <input
            value={locationName}
            onChange={(e) => setLocationName(e.target.value)}
            className="w-full rounded-xl border border-white/10 bg-[#111827] px-3 py-2.5 outline-none focus:border-yellow-400"
            placeholder="Cafe / park / venue"
          />
        </label>

        <div className="grid grid-cols-2 gap-3">
          <label className="block text-sm">
            <span className="mb-1 block text-slate-300">Latitude</span>
            <input
              type="number"
              step="any"
              required
              value={lat}
              onChange={(e) => setLat(Number(e.target.value))}
              className="w-full rounded-xl border border-white/10 bg-[#111827] px-3 py-2.5 outline-none focus:border-yellow-400"
            />
          </label>
          <label className="block text-sm">
            <span className="mb-1 block text-slate-300">Longitude</span>
            <input
              type="number"
              step="any"
              required
              value={lng}
              onChange={(e) => setLng(Number(e.target.value))}
              className="w-full rounded-xl border border-white/10 bg-[#111827] px-3 py-2.5 outline-none focus:border-yellow-400"
            />
          </label>
        </div>
        <button
          type="button"
          onClick={useMyLocation}
          className="text-sm text-blue-300 underline"
        >
          Use my current location
        </button>

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

        <fieldset className="space-y-2 text-sm">
          <legend className="mb-1 text-slate-300">Who can join?</legend>
          <label className="flex cursor-pointer items-start gap-2">
            <input
              type="radio"
              className="mt-1"
              checked={visibility === 'public'}
              onChange={() => setVisibility('public')}
            />
            <span>
              <strong className="text-white">Public</strong> — anyone can request to join
            </span>
          </label>
          <label className="flex cursor-pointer items-start gap-2">
            <input
              type="radio"
              className="mt-1"
              checked={visibility === 'private'}
              onChange={() => setVisibility('private')}
            />
            <span>
              <strong className="text-white">Private</strong> — invite only
            </span>
          </label>
        </fieldset>

        <label className="block text-sm">
          <span className="mb-1 block text-slate-300">Invite people (optional)</span>
          <input
            value={inviteUsernames}
            onChange={(e) => setInviteUsernames(e.target.value)}
            className="w-full rounded-xl border border-white/10 bg-[#111827] px-3 py-2.5 outline-none focus:border-yellow-400"
            placeholder="username1, username2"
          />
        </label>

        <button
          type="submit"
          disabled={busy}
          className="w-full rounded-xl bg-yellow-400 py-2.5 text-sm font-bold text-slate-900 hover:bg-yellow-300 disabled:opacity-60"
        >
          {busy ? 'Creating…' : 'Create event'}
        </button>
      </form>
    </div>
  )
}
