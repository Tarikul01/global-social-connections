/** Two-letter shortform from a display name or username. */
export function nameShortform(name?: string | null, fallback = '?'): string {
  const raw = (name || '').replace(/[@._-]+/g, ' ').trim()
  if (!raw) return fallback
  const parts = raw.split(/\s+/).filter(Boolean)
  if (parts.length === 1) {
    return parts[0].slice(0, 2).toUpperCase()
  }
  return (parts[0][0] + parts[1][0]).toUpperCase()
}

export function userShortform(u: {
  initials?: string
  full_name?: string
  username?: string
}): string {
  const fromApi = (u.initials || '').trim()
  if (fromApi.length >= 2) return fromApi.slice(0, 2).toUpperCase()
  if (fromApi.length === 1 && !u.full_name && !u.username) {
    return fromApi.toUpperCase()
  }
  return nameShortform(u.full_name || u.username || fromApi)
}
