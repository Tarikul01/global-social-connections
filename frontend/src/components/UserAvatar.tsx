import { useState } from 'react'
import { nameShortform } from '../lib/avatar'

type Props = {
  name?: string | null
  initials?: string | null
  photoUrl?: string | null
  online?: boolean
  size?: number
  className?: string
  fontSize?: number
  showDot?: boolean
}

/** Circular profile avatar: shortform initials, or uploaded photo when it loads. */
export function UserAvatar({
  name,
  initials,
  photoUrl,
  online,
  size = 36,
  className = '',
  fontSize,
  showDot = true,
}: Props) {
  const [imgFailed, setImgFailed] = useState(false)
  const short = (initials && initials.trim()) || nameShortform(name)
  const showPhoto = Boolean(photoUrl) && !imgFailed
  const status = online ? 'online' : 'offline'

  return (
    <div
      className={`disc-marker ${status} ${className}`.trim()}
      style={{
        width: size,
        height: size,
        fontSize: fontSize ?? Math.max(10, Math.round(size * 0.28)),
        flexShrink: 0,
      }}
      aria-hidden
    >
      <span className="disc-initials">{short.slice(0, 2).toUpperCase()}</span>
      {showPhoto && (
        <img src={photoUrl!} alt="" onError={() => setImgFailed(true)} />
      )}
      {showDot && <span className="dot" />}
    </div>
  )
}
