import { Link, useNavigate, useParams } from 'react-router-dom'
import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { MapPin, Pencil, Share2, Users, X } from 'lucide-react'
import { api } from '../lib/api'
import { useAuth } from '../auth/AuthContext'
import type { EventDetail } from '../lib/events'
import { EventShareBar, eventShareUrl } from '../components/EventShareBar'
import { UserInviteSuggest, type SuggestUser } from '../components/UserInviteSuggest'
import { EventMapPreview } from '../components/EventMapPreview'
import { EventTypeBadge } from '../components/EventTypeBadge'
import { resolveEventCategory } from '../lib/eventTypes'

type EventMember = {
  user_id: string
  username: string
  full_name?: string
  role: string
  is_creator: boolean
}

type EventInvite = {
  id: string
  invitee_id: string
  username: string
  full_name?: string
  email?: string
  status: string
}

type PeopleRoster = {
  going: EventMember[]
  invited: EventInvite[]
  is_host?: boolean
}

export function EventDetailPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const { user } = useAuth()
  const [detail, setDetail] = useState<EventDetail | null>(null)
  const [error, setError] = useState('')
  const [message, setMessage] = useState('')
  const [busy, setBusy] = useState(false)
  const [joinMsg, setJoinMsg] = useState('')
  const [showPeople, setShowPeople] = useState(false)
  const [people, setPeople] = useState<PeopleRoster | null>(null)
  const [peopleError, setPeopleError] = useState('')
  const [invitePick, setInvitePick] = useState<SuggestUser[]>([])
  const [confirmDelete, setConfirmDelete] = useState(false)
  const [showShare, setShowShare] = useState(false)

  const load = useCallback(async () => {
    if (!id) return
    setError('')
    try {
      const res = await api.get<{ success: boolean; data: EventDetail }>(`/api/v1/events/${id}`)
      setDetail(res.data)
      const title = res.data?.event?.title
      if (title) document.title = `${title} · FindNeighbour`
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load event')
      setDetail(null)
    }
  }, [id])

  useEffect(() => {
    void load()
    return () => {
      document.title = 'FindNeighbour · Global Social Connections'
    }
  }, [load])

  const loadPeople = useCallback(async () => {
    if (!id) return
    setPeopleError('')
    try {
      const res = await api.get<{ success: boolean; data: PeopleRoster }>(
        `/api/v1/events/${id}/members`,
      )
      setPeople({
        going: res.data?.going || [],
        invited: res.data?.invited || [],
        is_host: res.data?.is_host,
      })
    } catch (e) {
      setPeopleError(e instanceof Error ? e.message : 'Could not load people')
      setPeople({ going: [], invited: [] })
    }
  }, [id])

  useEffect(() => {
    if (showPeople) void loadPeople()
  }, [showPeople, loadPeople])

  const act = async (fn: () => Promise<unknown>, okMsg: string) => {
    setBusy(true)
    setError('')
    setMessage('')
    try {
      await fn()
      setMessage(okMsg)
      await load()
      if (showPeople) await loadPeople()
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
  const shareUrl = detail.share_url || e.share_url || eventShareUrl(e.id)
  const canShare = detail.is_host || detail.is_creator || e.visibility === 'public'
  const hasCoords = e.lat != null && e.lng != null && !(e.lat === 0 && e.lng === 0)
  const mapHref = hasCoords
    ? `/?lat=${e.lat}&lng=${e.lng}&zoom=15&event=${encodeURIComponent(e.id)}&label=${encodeURIComponent(e.title)}${
        (() => {
          const t = resolveEventCategory(e.category, e.title)
          return t ? `&type=${encodeURIComponent(t)}` : ''
        })()
      }`
    : null
  const host =
    detail.hosts?.find((h) => h.is_creator) || detail.hosts?.[0] || null
  const hostLabel = host
    ? host.full_name || host.username
    : null

  if (detail.login_required || detail.private) {
    return (
      <div className="min-h-full bg-[radial-gradient(900px_500px_at_90%_0%,rgba(250,204,21,0.08),transparent_50%),#050816] px-4 py-8">
        <div className="mx-auto max-w-lg space-y-4 rounded-2xl border border-white/10 bg-[#0b1224] p-6">
          <Link to="/events" className="text-sm text-blue-300 underline">
            ← Events
          </Link>
          <h1 className="text-2xl font-bold">{e.title}</h1>
          <p className="text-sm text-amber-200">This is a private invite-only event.</p>
          <p className="text-sm text-slate-400">
            {when}
            {e.location_name ? ` · ${e.location_name}` : ''}
          </p>
          <Link
            to={`/login?next=${encodeURIComponent(`/e/${e.id}`)}`}
            className="inline-flex rounded-xl bg-yellow-400 px-4 py-2.5 text-sm font-bold text-slate-900"
          >
            Log in to view &amp; join
          </Link>
          {user && (
            <p className="text-xs text-slate-500">
              You&apos;re logged in but need an invitation to see this private event.
            </p>
          )}
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-full bg-[radial-gradient(900px_500px_at_90%_0%,rgba(250,204,21,0.08),transparent_50%),#050816] px-4 py-8">
      <div className="mx-auto max-w-lg space-y-4 rounded-2xl border border-white/10 bg-[#0b1224] p-6">
        <div className="flex items-center justify-between gap-3">
          <Link to="/events" className="text-sm text-blue-300 underline">
            ← Events
          </Link>
          <div className="flex items-center gap-2">
            {detail.is_creator && (
              <Link
                to={`/events/${e.id}/edit`}
                className="inline-flex h-9 w-9 items-center justify-center rounded-full border border-white/15 bg-white/5 text-slate-200 hover:bg-white/10"
                title="Edit event"
                aria-label="Edit event"
              >
                <Pencil size={16} />
              </Link>
            )}
            {canShare && (
              <button
                type="button"
                onClick={() => setShowShare((v) => !v)}
                className={`inline-flex h-9 w-9 items-center justify-center rounded-full border transition ${
                  showShare
                    ? 'border-yellow-400/50 bg-yellow-400/15 text-yellow-300'
                    : 'border-white/15 bg-white/5 text-slate-200 hover:bg-white/10'
                }`}
                title={showShare ? 'Hide share' : 'Share event'}
                aria-pressed={showShare}
                aria-label="Share event"
              >
                {showShare ? <X size={16} /> : <Share2 size={16} />}
              </button>
            )}
          </div>
        </div>

        {showShare && canShare && <EventShareBar shareUrl={shareUrl} title={e.title} />}

        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-2">
              <h1 className="text-2xl font-bold">{e.title}</h1>
              <EventTypeBadge category={e.category} />
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
          </div>
          {hostLabel && (
            <div className="max-w-[40%] shrink-0 text-right">
              <p className="text-[10px] font-semibold uppercase tracking-wide text-slate-500">
                Host
              </p>
              <p className="truncate text-sm font-medium text-yellow-300/90" title={hostLabel}>
                {hostLabel}
              </p>
              {host?.username && (
                <p className="truncate text-xs text-slate-500">@{host.username}</p>
              )}
            </div>
          )}
        </div>

        <p className="text-sm text-slate-400">
          {when}
          {timeRange ? ` · ${timeRange}` : ''}
          {e.location_name ? ` · ${e.location_name}` : ''}
        </p>

        {e.description && <p className="text-slate-200">{e.description}</p>}

        {hasCoords ? (
          <EventMapPreview
            lat={e.lat!}
            lng={e.lng!}
            label={e.location_name || e.title}
            category={e.category}
            discoverHref={mapHref}
          />
        ) : (
          <p className="inline-flex items-center gap-1.5 text-sm text-slate-500">
            <MapPin size={14} /> No map location set
          </p>
        )}

        <div className="flex flex-wrap items-center justify-between gap-2 text-sm text-slate-400">
          <span className="inline-flex items-center gap-1.5">
            <Users size={14} /> {e.member_count ?? 0}
            {e.max_participants ? ` / ${e.max_participants}` : ''} going
          </span>
          <button
            type="button"
            onClick={() => setShowPeople((v) => !v)}
            className={`inline-flex items-center gap-1.5 rounded-lg border px-2.5 py-1 text-xs font-medium transition ${
              showPeople
                ? 'border-yellow-400/40 bg-yellow-400/10 text-yellow-200'
                : 'border-white/15 bg-white/5 text-slate-200 hover:bg-white/10'
            }`}
          >
            <Users size={12} />
            {showPeople ? 'Hide list' : 'People'}
          </button>
        </div>

        {showPeople && (
          <section className="space-y-3 rounded-xl border border-white/10 bg-[#111827] p-3">
            {detail.is_host && (
              <div className="space-y-1.5 border-b border-white/10 pb-3">
                <p className="text-xs text-slate-500">Invite by username, email, or name</p>
                <UserInviteSuggest
                  searchPath={`/api/v1/events/${e.id}/users/search`}
                  selected={invitePick}
                  onChange={setInvitePick}
                  single
                  disabled={busy}
                  onPickSingle={(u) => {
                    void act(
                      () =>
                        api.post(`/api/v1/events/${e.id}/invite`, {
                          username: u.username,
                          query: u.email || u.username,
                        }),
                      `Invited @${u.username}`,
                    )
                  }}
                  placeholder="Type username, email, or name"
                />
              </div>
            )}

            {peopleError && <p className="text-sm text-red-300">{peopleError}</p>}
            {!people && !peopleError && <p className="text-sm text-slate-500">Loading…</p>}

            {people && (
              <>
                <div>
                  <h2 className="mb-1.5 text-xs font-semibold uppercase tracking-wide text-slate-500">
                    Going ({people.going.length})
                  </h2>
                  {people.going.length === 0 ? (
                    <p className="text-sm text-slate-500">No one has joined yet.</p>
                  ) : (
                    <ul className="space-y-1.5">
                      {people.going.map((m) => (
                        <li key={m.user_id} className="text-sm text-slate-300">
                          {m.full_name || m.username}{' '}
                          <span className="text-slate-500">@{m.username}</span>
                          <span className="ml-1 text-xs text-emerald-400/80">· going</span>
                          {m.is_creator && (
                            <span className="ml-1 text-xs text-yellow-400/80">· creator</span>
                          )}
                        </li>
                      ))}
                    </ul>
                  )}
                </div>

                <div className="border-t border-white/10 pt-3">
                  <h2 className="mb-1.5 text-xs font-semibold uppercase tracking-wide text-slate-500">
                    Invited ({people.invited.length})
                  </h2>
                  {people.invited.length === 0 ? (
                    <p className="text-sm text-slate-500">No pending invitations.</p>
                  ) : (
                    <ul className="space-y-1.5">
                      {people.invited.map((inv) => (
                        <li key={inv.id} className="text-sm text-slate-300">
                          {inv.full_name || inv.username}{' '}
                          <span className="text-slate-500">@{inv.username}</span>
                          <span className="ml-1 text-xs text-amber-300/90">· invited</span>
                        </li>
                      ))}
                    </ul>
                  )}
                </div>
              </>
            )}
          </section>
        )}

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
          {!user && e.visibility === 'public' && (
            <Link
              to={`/login?next=${encodeURIComponent(`/e/${e.id}`)}`}
              className="rounded-xl bg-yellow-400 px-4 py-2 text-sm font-bold text-slate-900"
            >
              Log in to join
            </Link>
          )}
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
          {user &&
            !detail.is_member &&
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

        {detail.is_host && detail.pending_join_requests?.length > 0 && (
          <section className="space-y-2 border-t border-white/10 pt-4">
            <h2 className="text-sm font-semibold text-white">Join requests</h2>
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
          </section>
        )}

        {detail.is_creator && (
          <section className="space-y-3 border-t border-red-500/20 pt-4">
            <h2 className="text-sm font-semibold text-red-200">Danger zone</h2>
            <p className="text-xs text-slate-500">
              Deleting removes this event and everyone on the going list, invites, and join
              requests. This cannot be undone.
            </p>
            {!confirmDelete ? (
              <button
                type="button"
                disabled={busy}
                onClick={() => setConfirmDelete(true)}
                className="rounded-xl border border-red-500/40 bg-red-500/10 px-4 py-2 text-sm font-medium text-red-200 hover:bg-red-500/20 disabled:opacity-60"
              >
                Delete event
              </button>
            ) : (
              <div className="flex flex-wrap gap-2">
                <button
                  type="button"
                  disabled={busy}
                  onClick={() => {
                    void (async () => {
                      setBusy(true)
                      setError('')
                      try {
                        await api.delete(`/api/v1/events/${e.id}`)
                        navigate('/events', { replace: true })
                      } catch (err) {
                        setError(err instanceof Error ? err.message : 'Delete failed')
                        setConfirmDelete(false)
                      } finally {
                        setBusy(false)
                      }
                    })()
                  }}
                  className="rounded-xl bg-red-500 px-4 py-2 text-sm font-bold text-white hover:bg-red-400 disabled:opacity-60"
                >
                  {busy ? 'Deleting…' : 'Yes, delete forever'}
                </button>
                <button
                  type="button"
                  disabled={busy}
                  onClick={() => setConfirmDelete(false)}
                  className="rounded-xl border border-white/15 px-4 py-2 text-sm text-slate-200 disabled:opacity-60"
                >
                  Cancel
                </button>
              </div>
            )}
          </section>
        )}
      </div>
    </div>
  )
}
