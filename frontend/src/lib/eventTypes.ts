import { Bike, Briefcase, Coffee, Users, type LucideIcon } from 'lucide-react'

export type EventTypeId = 'coffee' | 'meetup' | 'ride_share' | 'business'

export type EventType = {
  id: EventTypeId
  label: string
  description: string
  Icon: LucideIcon
  /** Accent for badges / map pin */
  color: string
}

export const EVENT_TYPES: EventType[] = [
  {
    id: 'coffee',
    label: 'Coffee event',
    description: 'Casual coffee or cafe meetup',
    Icon: Coffee,
    color: '#c4a574',
  },
  {
    id: 'meetup',
    label: 'Meetup event',
    description: 'General hangout or group meetup',
    Icon: Users,
    color: '#60a5fa',
  },
  {
    id: 'ride_share',
    label: 'Ride share event',
    description: 'Share a ride or travel together',
    Icon: Bike,
    color: '#34d399',
  },
  {
    id: 'business',
    label: 'Business event',
    description: 'Networking or professional meet',
    Icon: Briefcase,
    color: '#fbbf24',
  },
]

const byId = Object.fromEntries(EVENT_TYPES.map((t) => [t.id, t])) as Record<
  EventTypeId,
  EventType
>

export function isEventTypeId(value: string | undefined | null): value is EventTypeId {
  return !!value && value in byId
}

export function getEventType(category?: string | null): EventType | null {
  if (!category || !isEventTypeId(category)) return null
  return byId[category]
}

/** Prefer stored category; fall back to title keywords for older events. */
export function resolveEventCategory(
  category?: string | null,
  title?: string | null,
): EventTypeId | null {
  if (isEventTypeId(category)) return category
  const t = (title || '').toLowerCase()
  if (/\b(coffee|cafe|café|espresso)\b/.test(t)) return 'coffee'
  if (/\b(ride|bike|bus|carpool|car\s*share|taxi)\b/.test(t)) return 'ride_share'
  if (/\b(business|network|professional|office)\b/.test(t)) return 'business'
  if (/\b(meetup|hangout|gathering|meet\s*up)\b/.test(t)) return 'meetup'
  return null
}

export function eventTypeLabel(category?: string | null, title?: string | null): string {
  const id = resolveEventCategory(category, title)
  return (id && byId[id]?.label) || 'Event'
}
