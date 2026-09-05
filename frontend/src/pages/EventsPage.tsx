import { Link } from 'react-router-dom'
import { useEffect, useState } from 'react'
import { CalendarPlus, MapPin, Users } from 'lucide-react'
import { api } from '../lib/api'
import type { EventSummary } from '../lib/events'

export function EventsPage() {
  const [events, setEvents] = useState<EventSummary[]>([])
  const [hosted, setHosted] = useState<EventSummary[]>([])
  const [createdCount, setCreatedCount] = useState(0)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)
  const atCreateLimit = createdCount >= 2

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      setLoading(true)
      setError('')
      try {
        const [all, mine] = await Promise.all([
          api.get<{ success: boolean; data: EventSummary[] }>('/api/v1/events'),
          api.get<{
            success: boolean
            data: Array<{
              id: string
              title: string
              location_name?: string
              event_date?: string
              member_count?: number
              lat?: number
              lng?: number
            }>
            meta?: { created_count?: number; max_events?: number }
          }>('/api/v1/events/hosted'),
        ])
        if (cancelled) return
        setEvents(all.data ?? [])
        setCreatedCount(mine.meta?.created_count ?? 0)
        setHosted(
          (mine.data ?? []).map((e) => ({
            id: e.id,
            creator_id: '',
            title: e.title,
            location_name: e.location_name,
            event_date: e.event_date ?? '',
            event_date_label: e.event_date,
            member_count: e.member_count ?? 0,
            visibility: 'public',
            lat: e.lat,
            lng: e.lng,
          })),
        )
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : 'Failed to load events')
      } finally {
        if (!cancelled) setLoading(false)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [])

  return (
    <div className="min-h-full bg-[radial-gradient(900px_500px_at_90%_0%,rgba(250,204,21,0.08),transparent_50%),#050816] px-4 py-8">
      <div className="mx-auto max-w-2xl">
        <div className="mb-6 flex items-center justify-between gap-3">
          <div>
            <Link to="/" className="text-sm text-blue-300 underline">
              ← Map
            </Link>
            <h1 className="mt-2 text-2xl font-bold">Events</h1>
            <p className="text-sm text-slate-400">Meetups near you and gatherings you host</p>
            {atCreateLimit && (
              <p className="mt-1 text-xs text-amber-300/90">
                You already host 2 events — delete one to create another.
              </p>
            )}
          </div>
          {atCreateLimit ? (
            <span
              className="inline-flex cursor-not-allowed items-center gap-2 rounded-xl border border-white/10 bg-white/5 px-4 py-2.5 text-sm font-bold text-slate-500"
              title="Delete an existing event to create a new one"
            >
              <CalendarPlus size={16} /> Create
            </span>
          ) : (
            <Link
              to="/events/new"
              className="inline-flex items-center gap-2 rounded-xl bg-yellow-400 px-4 py-2.5 text-sm font-bold text-slate-900 hover:bg-yellow-300"
            >
              <CalendarPlus size={16} /> Create
            </Link>
          )}
        </div>

        {error && (
          <div className="mb-4 rounded-xl border border-red-400/30 bg-red-500/10 px-3 py-2 text-sm text-red-200">
            {error}
          </div>
        )}

        {loading ? (
          <p className="text-slate-400">Loading events…</p>
        ) : (
          <div className="space-y-8">
            {hosted.length > 0 && (
              <section>
                <h2 className="mb-3 text-xs font-semibold uppercase tracking-wide text-slate-500">
                  Hosting
                </h2>
                <div className="space-y-3">
                  {hosted.map((e) => (
                    <EventCard key={`h-${e.id}`} event={e} />
                  ))}
                </div>
              </section>
            )}

            <section>
              <h2 className="mb-3 text-xs font-semibold uppercase tracking-wide text-slate-500">
                Upcoming
              </h2>
              {events.length === 0 ? (
                <div className="rounded-2xl border border-dashed border-white/15 bg-[#0b1224]/60 p-8 text-center">
                  <p className="text-slate-300">No published events yet.</p>
                  {atCreateLimit ? (
                    <p className="mt-3 text-sm text-amber-300/90">
                      You already host 2 events. Delete one to create another.
                    </p>
                  ) : (
                    <Link to="/events/new" className="mt-3 inline-block text-sm text-yellow-300 underline">
                      Create the first one
                    </Link>
                  )}
                </div>
              ) : (
                <div className="space-y-3">
                  {events.map((e) => (
                    <EventCard key={e.id} event={e} />
                  ))}
                </div>
              )}
            </section>
          </div>
        )}
      </div>
    </div>
  )
}

function EventCard({ event }: { event: EventSummary }) {
  const when = event.event_date_label || event.event_date
  const time = event.start_time ? ` · ${event.start_time}` : ''
  return (
    <Link
      to={`/e/${event.id}`}
      className="block rounded-2xl border border-white/10 bg-[#0b1224] p-4 transition hover:border-yellow-400/40"
    >
      <div className="flex items-start justify-between gap-3">
        <h3 className="font-semibold text-white">{event.title}</h3>
        <span
          className={`shrink-0 rounded-full border px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide ${
            event.visibility === 'private'
              ? 'border-amber-500/40 text-amber-300'
              : 'border-blue-500/40 text-blue-300'
          }`}
        >
          {event.visibility === 'private' ? 'Private' : 'Public'}
        </span>
      </div>
      <p className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-sm text-slate-400">
        <span>
          {when}
          {time}
        </span>
        {event.location_name && (
          <span className="inline-flex items-center gap-1">
            <MapPin size={14} /> {event.location_name}
          </span>
        )}
        <span className="inline-flex items-center gap-1">
          <Users size={14} /> {event.member_count} going
        </span>
      </p>
    </Link>
  )
}
