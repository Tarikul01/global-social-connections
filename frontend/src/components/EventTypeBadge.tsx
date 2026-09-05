import { getEventType } from '../lib/eventTypes'

/** Small type badge with icon — for detail / list cards. */
export function EventTypeBadge({
  category,
  className = '',
}: {
  category?: string | null
  className?: string
}) {
  const t = getEventType(category)
  if (!t) return null
  const Icon = t.Icon
  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-[11px] font-semibold ${className}`}
      style={{
        borderColor: `${t.color}66`,
        color: t.color,
        backgroundColor: `${t.color}14`,
      }}
    >
      <Icon size={12} />
      {t.label}
    </span>
  )
}
