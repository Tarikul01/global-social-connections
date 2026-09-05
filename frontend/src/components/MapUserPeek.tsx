import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Briefcase, Users, X } from 'lucide-react'
import { api, type NearbyUser } from '../lib/api'
import { UserAvatar } from './UserAvatar'

type Preview = {
  username: string
  full_name?: string
  profession?: string
  photo_url?: string
  bio?: string
  followers?: number
  following?: number
}

type Props = {
  user: NearbyUser
  loggedIn: boolean
  onClose: () => void
}

export function MapUserPeek({ user, loggedIn, onClose }: Props) {
  const navigate = useNavigate()
  const [preview, setPreview] = useState<Preview | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setPreview(null)
    api
      .get<{ success: boolean; data: Preview }>(
        `/api/v1/profiles/${encodeURIComponent(user.username)}`,
      )
      .then((r) => {
        if (!cancelled) setPreview(r.data)
      })
      .catch(() => {
        if (!cancelled) {
          setPreview({
            username: user.username,
            full_name: user.full_name,
            profession: user.profession,
            photo_url: user.photo_url,
          })
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [user.username, user.full_name, user.profession, user.photo_url])

  const name =
    (loggedIn && (preview?.full_name || user.full_name)) ||
    `@${preview?.username || user.username}`
  const profession = preview?.profession || user.profession || 'Professional'
  const photo = preview?.photo_url || user.photo_url
  const followers = preview?.followers
  const following = preview?.following

  const openFull = () => {
    if (user.is_me) {
      navigate('/profile')
      return
    }
    if (loggedIn) {
      navigate(`/u/${user.username}`)
      return
    }
    navigate(`/login?next=${encodeURIComponent(`/u/${user.username}`)}`)
  }

  return (
    <div
      className="pointer-events-auto absolute bottom-4 left-1/2 z-[1050] w-[min(92vw,22rem)] -translate-x-1/2 rounded-2xl border border-white/15 bg-[#0b1224]/96 p-4 shadow-2xl backdrop-blur"
      role="dialog"
      aria-label={`${name} preview`}
    >
      <button
        type="button"
        onClick={onClose}
        className="absolute right-3 top-3 rounded-full bg-white/10 p-1.5 text-slate-300 hover:bg-white/20"
        aria-label="Close"
      >
        <X size={14} />
      </button>

      <div className="flex items-start gap-3 pr-8">
        <UserAvatar
          name={name}
          initials={user.initials}
          photoUrl={photo}
          online={user.online}
          size={56}
          fontSize={16}
        />
        <div className="min-w-0 flex-1">
          <div className="truncate text-base font-bold text-white">
            {name}
            {user.is_me ? (
              <span className="ml-1 text-xs font-medium text-yellow-300">(You)</span>
            ) : null}
          </div>
          <div className="mt-0.5 truncate text-sm text-slate-400">@{user.username}</div>
          <div className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-slate-300">
            <span className="inline-flex items-center gap-1">
              <Briefcase size={12} className="text-slate-500" />
              {profession}
            </span>
            {typeof followers === 'number' && (
              <span className="inline-flex items-center gap-1">
                <Users size={12} className="text-slate-500" />
                {followers} followers
                {typeof following === 'number' ? ` · ${following} following` : ''}
              </span>
            )}
            {loading && <span className="text-slate-500">Loading…</span>}
          </div>
          <div className="mt-1 text-xs text-slate-500">
            {user.online ? (
              <span className="text-emerald-400">Online</span>
            ) : (
              <span>Offline</span>
            )}
            {user.live ? <span className="text-red-300"> · Live</span> : null}
            {loggedIn && typeof user.distance_km === 'number' ? (
              <span> · {user.distance_km.toFixed(1)} km</span>
            ) : null}
          </div>
        </div>
      </div>

      <button
        type="button"
        onClick={openFull}
        className="mt-4 w-full rounded-xl bg-yellow-400 py-2.5 text-sm font-bold text-slate-900 hover:bg-yellow-300"
      >
        {user.is_me ? 'View my profile' : 'View full profile'}
      </button>
    </div>
  )
}
