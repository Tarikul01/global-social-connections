import { Link, useParams } from 'react-router-dom'
import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { MapPin, Users } from 'lucide-react'
import { api } from '../lib/api'
import type { EventDetail } from '../lib/events'

export function EventDetailPage() {
  const { id } = useParams()
  const [detail, setDetail] = useState<EventDetail | null>(null)
  const [error, setError] = useState('')
  const [message, setMessage] = useState('')
  const [busy, setBusy] = useState(false)
  const [inviteUser, setInviteUser] = useState('')
  const [joinMsg, setJoinMsg] = useState('')

  const load = useCallback(async () => {
    if (!id) return
    setError('')
    try {
      const res = await api.get<{ success: boolean; data: EventDetail }>(`/api/v1/events/${id}`)
      setDetail(res.data)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load event')
      setDetail(null)
    }
  }, [id])

  useEffect(() => {
    void load()
  }, [load])

  const act = async (fn: () => Promise<unknown>, okMsg: string) => {
    setBusy(true)
    setError('')
    setMessage('')
    try {
      await fn()
      setMessage(okMsg)
      await load()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Action failed')
    } finally {
      setBusy(false)
    }
  }

  if (error && !detail) {
    return (
      <div className="flex min-h-full flex-col items-center justify-center gap-3 p-6">
        <p className="text-red-300">{error}</p>
        <Link to="/events" className="text-blue-300 underline">
          Back to events
        </Link>
      </div>
    )
  }

  if (!detail) {
    return <div className="flex min-h-full items-center justify-center text-slate-400">Loading…</div>
  }

  const e = detail.event
  const when = e.event_date_label || e.event_date
  const timeRange = [e.start_time, e.end_time].filter(Boolean).join(' – ')

  return (
    <div className="min-h-full bg-[radial-gradient(900px_500px_at_90%_0%,rgba(250,204,21,0.08),transparent_50%),#050816] px-4 py-8">
      <div className="mx-auto max-w-lg space-y-4 rounded-2xl border border-white/10 bg-[#0b1224] p-6">
        <Link to="/events" className="text-sm text-blue-300 underline">
          ← Events
        </Link>

        <div className="flex items-start justify-between gap-3">
          <h1 className="text-2xl font-bold">{e.title}</h1>
          <span
            className={`shrink-0 rounded-full border px-2.5 py-1 text-[10px] font-semibold uppercase tracking-wide ${
              e.visibility === 'private'
                ? 'border-amber-500/40 text-amber-300'
                : 'border-blue-500/40 text-blue-300'
            }`}
          >
            {e.visibility === 'private' ? 'Private' : 'Public'}
          </span>
        </div>

        <p className="text-sm text-slate-400">
          {when}
          {timeRange ? ` · ${timeRange}` : ''}
          {e.location_name ? ` · ${e.location_name}` : ''}
        </p>

        {e.description && <p className="text-slate-200">{e.description}</p>}

        <p className="flex flex-wrap gap-4 text-sm text-slate-400">
          <span className="inline-flex items-center gap-1">
            <Users size={14} /> {e.member_count}
            {e.max_participants ? ` / ${e.max_participants}` : ''} going
          </span>
          {e.lat != null && e.lng != null && (
            <span className="inline-flex items-center gap-1">
              <MapPin size={14} /> {e.lat.toFixed(4)}, {e.lng.toFixed(4)}
            </span>
          )}
        </p>

        {error && (
          <div className="rounded-xl border border-red-400/30 bg-red-500/10 px-3 py-2 text-sm text-red-200">
            {error}
          </div>
        )}
        {message && (
          <div className="rounded-xl border border-emerald-400/30 bg-emerald-500/10 px-3 py-2 text-sm text-emerald-200">
            {message}
          </div>
        )}

        <div className="flex flex-wrap gap-2 border-t border-white/10 pt-4">
          {detail.is_member && (
            <span className="rounded-xl border border-emerald-500/30 bg-emerald-500/10 px-3 py-2 text-sm text-emerald-200">
              You&apos;re going
            </span>
          )}
          {detail.has_pending_join && (
            <span className="rounded-xl border border-amber-500/30 bg-amber-500/10 px-3 py-2 text-sm text-amber-200">
              Join request pending
            </span>
          )}
          {detail.my_invitation_id && !detail.is_member && (
            <>
              <button
                type="button"
                disabled={busy}
                onClick={() =>
                  void act(
                    () =>
                      api.post(`/api/v1/events/${e.id}/invitations/${detail.my_invitation_id}/accept`),
                    'You joined the event',
                  )
                }
                className="rounded-xl bg-yellow-400 px-4 py-2 text-sm font-bold text-slate-900 disabled:opacity-60"
              >
                Accept invite
              </button>
              <button
                type="button"
                disabled={busy}
                onClick={() =>
                  void act(
                    () =>
                      api.post(`/api/v1/events/${e.id}/invitations/${detail.my_invitation_id}/decline`),
                    'Invitation declined',
                  )
                }
                className="rounded-xl border border-white/15 px-4 py-2 text-sm text-slate-200 disabled:opacity-60"
              >
                Decline
              </button>
            </>
          )}
          {!detail.is_member &&
            !detail.has_pending_join &&
            !detail.my_invitation_id &&
            e.visibility === 'public' &&
            !detail.is_creator && (
              <form
                className="flex w-full flex-col gap-2 sm:flex-row"
                onSubmit={(ev: FormEvent) => {
                  ev.preventDefault()
                  void act(
                    () => api.post(`/api/v1/events/${e.id}/join`, { message: joinMsg }),
                    'Join request sent',
                  )
                }}
              >
                <input
                  value={joinMsg}
                  onChange={(ev) => setJoinMsg(ev.target.value)}
                  placeholder="Optional message"
                  className="flex-1 rounded-xl border border-white/10 bg-[#111827] px-3 py-2 text-sm outline-none focus:border-yellow-400"
                />
                <button
                  type="submit"
                  disabled={busy}
                  className="rounded-xl bg-yellow-400 px-4 py-2 text-sm font-bold text-slate-900 disabled:opacity-60"
                >
                  Request to join
                </button>
              </form>
            )}
        </div>

        {detail.is_host && (
          <section className="space-y-3 border-t border-white/10 pt-4">
            <h2 className="text-sm font-semibold text-white">Host tools</h2>
            <form
              className="flex gap-2"
              onSubmit={(ev: FormEvent) => {
                ev.preventDefault()
                if (!inviteUser.trim()) return
                void act(
                  () => api.post(`/api/v1/events/${e.id}/invite`, { username: inviteUser.trim() }),
                  `Invited @${inviteUser.trim()}`,
                ).then(() => setInviteUser(''))
              }}
            >
              <input
                value={inviteUser}
                onChange={(ev) => setInviteUser(ev.target.value)}
                placeholder="Invite by username"
                className="flex-1 rounded-xl border border-white/10 bg-[#111827] px-3 py-2 text-sm outline-none focus:border-yellow-400"
              />
              <button
                type="submit"
                disabled={busy}
                className="rounded-xl border border-white/15 px-3 py-2 text-sm font-medium text-white disabled:opacity-60"
              >
                Invite
              </button>
            </form>

            {detail.pending_join_requests?.length > 0 && (
              <div className="space-y-2">
                <h3 className="text-xs font-semibold uppercase tracking-wide text-slate-500">
                  Join requests
                </h3>
                {detail.pending_join_requests.map((r) => (
                  <div
                    key={r.id}
                    className="flex flex-wrap items-center justify-between gap-2 rounded-xl border border-white/10 bg-[#111827] px-3 py-2"
                  >
                    <div>
                      <div className="text-sm font-medium">
                        {r.full_name || r.username}{' '}
                        <span className="text-slate-500">@{r.username}</span>
                      </div>
                      {r.message && <p className="text-xs text-slate-400">{r.message}</p>}
                    </div>
                    <div className="flex gap-2">
                      <button
                        type="button"
                        disabled={busy}
                        onClick={() =>
                          void act(
                            () => api.post(`/api/v1/events/${e.id}/requests/${r.id}/accept`),
                            'Request accepted',
                          )
                        }
                        className="rounded-lg bg-yellow-400 px-2.5 py-1 text-xs font-bold text-slate-900"
                      >
                        Accept
                      </button>
                      <button
                        type="button"
                        disabled={busy}
                        onClick={() =>
                          void act(
                            () => api.post(`/api/v1/events/${e.id}/requests/${r.id}/reject`),
                            'Request declined',
                          )
                        }
                        className="rounded-lg border border-white/15 px-2.5 py-1 text-xs text-slate-200"
                      >
                        Decline
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </section>
        )}

        {detail.hosts?.length > 0 && (
          <section className="border-t border-white/10 pt-4">
            <h2 className="mb-2 text-xs font-semibold uppercase tracking-wide text-slate-500">Hosts</h2>
            <ul className="space-y-1 text-sm text-slate-300">
              {detail.hosts.map((h) => (
                <li key={h.user_id}>
                  {h.full_name || h.username}{' '}
                  <span className="text-slate-500">@{h.username}</span>
                  {h.is_creator && <span className="ml-1 text-yellow-400/80">· creator</span>}
                </li>
              ))}
            </ul>
          </section>
        )}
      </div>
    </div>
  )
}
