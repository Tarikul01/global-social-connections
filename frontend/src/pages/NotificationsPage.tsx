import { Link } from 'react-router-dom'
import { useCallback, useEffect, useState } from 'react'
import { Bell, Calendar, Check, X } from 'lucide-react'
import { api } from '../lib/api'

export type AppNotification = {
  id: string
  type: string
  title: string
  body: string
  unread: boolean
  invite_pending?: boolean
  event_id?: string
  invitation_id?: string
  created_at: string
}

function formatWhen(iso: string) {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return d.toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

export function NotificationsPage() {
  const [items, setItems] = useState<AppNotification[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [busyId, setBusyId] = useState('')

  const load = useCallback(async () => {
    setError('')
    try {
      const res = await api.get<{ success: boolean; data: AppNotification[] }>(
        '/api/v1/notifications',
      )
      setItems(res.data || [])
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load notifications')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  const markRead = async (id: string) => {
    try {
      await api.post(`/api/v1/notifications/${id}/read`, {})
      setItems((prev) => prev.map((n) => (n.id === id ? { ...n, unread: false } : n)))
      window.dispatchEvent(new Event('fn:notifications-changed'))
    } catch {
      /* ignore */
    }
  }

  const respondInvite = async (
    n: AppNotification,
    action: 'accept' | 'decline',
  ) => {
    if (!n.event_id || !n.invitation_id) return
    setBusyId(n.id)
    setError('')
    try {
      await api.post(
        `/api/v1/events/${n.event_id}/invitations/${n.invitation_id}/${action}`,
        {},
      )
      await markRead(n.id)
      setItems((prev) =>
        prev.map((x) =>
          x.id === n.id ? { ...x, invite_pending: false, unread: false } : x,
        ),
      )
      window.dispatchEvent(new Event('fn:notifications-changed'))
    } catch (e) {
      setError(e instanceof Error ? e.message : `${action} failed`)
    } finally {
      setBusyId('')
    }
  }

  return (
    <div className="min-h-full bg-[radial-gradient(900px_500px_at_90%_0%,rgba(250,204,21,0.08),transparent_50%),#050816] px-4 py-8">
      <div className="mx-auto max-w-2xl">
        <Link to="/" className="text-sm text-blue-300 underline">
          ← Map
        </Link>
        <div className="mt-2 mb-6 flex items-center gap-2">
          <Bell size={22} className="text-amber-300" />
          <div>
            <h1 className="text-2xl font-bold">Notifications</h1>
            <p className="text-sm text-slate-400">Invites, join requests, and updates</p>
          </div>
        </div>

        {error && (
          <div className="mb-4 rounded-xl border border-red-400/30 bg-red-500/10 px-3 py-2 text-sm text-red-200">
            {error}
          </div>
        )}

        {loading ? (
          <p className="text-slate-400">Loading…</p>
        ) : items.length === 0 ? (
          <div className="rounded-2xl border border-dashed border-white/15 bg-[#0b1224]/60 p-8 text-center text-slate-300">
            No notifications yet.
          </div>
        ) : (
          <ul className="space-y-3">
            {items.map((n) => (
              <li
                key={n.id}
                className={`rounded-2xl border p-4 ${
                  n.unread
                    ? 'border-amber-400/30 bg-amber-400/5'
                    : 'border-white/10 bg-[#0b1224]'
                }`}
              >
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <div className="flex flex-wrap items-center gap-2">
                      <h2 className="font-semibold text-white">{n.title}</h2>
                      {n.unread && (
                        <span className="rounded-full bg-amber-400 px-1.5 py-0.5 text-[10px] font-bold uppercase text-slate-900">
                          New
                        </span>
                      )}
                    </div>
                    <p className="mt-1 text-sm text-slate-300">{n.body}</p>
                    <p className="mt-1 text-xs text-slate-500">{formatWhen(n.created_at)}</p>
                  </div>
                  {n.unread && (
                    <button
                      type="button"
                      onClick={() => void markRead(n.id)}
                      className="shrink-0 rounded-lg border border-white/15 px-2 py-1 text-[11px] text-slate-300 hover:bg-white/5"
                      title="Mark as read"
                    >
                      <Check size={14} />
                    </button>
                  )}
                </div>

                <div className="mt-3 flex flex-wrap gap-2">
                  {n.event_id && (
                    <Link
                      to={`/e/${n.event_id}`}
                      onClick={() => {
                        if (n.unread) void markRead(n.id)
                      }}
                      className="inline-flex items-center gap-1.5 rounded-lg border border-white/15 bg-white/5 px-3 py-1.5 text-xs font-medium text-slate-100 hover:bg-white/10"
                    >
                      <Calendar size={12} /> View event
                    </Link>
                  )}
                  {n.invite_pending && n.event_id && n.invitation_id && (
                    <>
                      <button
                        type="button"
                        disabled={busyId === n.id}
                        onClick={() => void respondInvite(n, 'accept')}
                        className="rounded-lg bg-yellow-400 px-3 py-1.5 text-xs font-bold text-slate-900 hover:bg-yellow-300 disabled:opacity-60"
                      >
                        Accept invite
                      </button>
                      <button
                        type="button"
                        disabled={busyId === n.id}
                        onClick={() => void respondInvite(n, 'decline')}
                        className="inline-flex items-center gap-1 rounded-lg border border-white/20 px-3 py-1.5 text-xs font-medium text-slate-200 hover:bg-white/5 disabled:opacity-60"
                      >
                        <X size={12} /> Decline
                      </button>
                    </>
                  )}
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}
