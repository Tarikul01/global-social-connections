import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, type NearbyUser } from '../lib/api'
import { UserAvatar } from './UserAvatar'

type Preview = {
  username: string
  full_name?: string
  profession?: string
  photo_url?: string
  followers?: number
  following?: number
}

type Props = {
  user: NearbyUser
  loggedIn: boolean
}

/** Compact tooltip card content — rendered inside a Leaflet Popup pointed at the marker. */
export function MapUserPeek({ user, loggedIn }: Props) {
  const navigate = useNavigate()
  const [preview, setPreview] = useState<Preview | null>(null)

  useEffect(() => {
    let cancelled = false
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
    <div className="fn-peek-card" role="dialog" aria-label={`${name} preview`}>
      <div className="fn-peek-card__avatar">
        <UserAvatar
          name={name}
          initials={user.initials}
          photoUrl={photo}
          online={user.online}
          size={52}
          fontSize={15}
        />
      </div>
      <div className="fn-peek-card__name">
        {name}
        {user.is_me ? <span className="fn-peek-card__you"> · you</span> : null}
      </div>
      <div className="fn-peek-card__meta">@{user.username}</div>
      <div className="fn-peek-card__meta">{profession}</div>
      {typeof followers === 'number' && (
        <div className="fn-peek-card__meta">{followers} followers</div>
      )}
      <div className="fn-peek-card__status">
        <span className={user.online ? 'is-online' : ''}>{user.online ? 'Online' : 'Offline'}</span>
        {user.live ? <span className="is-live"> · Live</span> : null}
      </div>
      <button type="button" className="fn-peek-card__btn" onClick={openFull}>
        {user.is_me ? 'My profile' : 'View profile'}
      </button>
    </div>
  )
}
