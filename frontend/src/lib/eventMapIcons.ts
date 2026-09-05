import L from 'leaflet'
import {
  EVENT_TYPES,
  getEventType,
  resolveEventCategory,
  type EventTypeId,
} from './eventTypes'

/** Lucide-style multi-path SVGs (viewBox 0 0 24 24) — distinct per event type. */
const ICON_SVG: Record<EventTypeId, string> = {
  // Coffee cup
  coffee: `
    <path d="M10 2v2"/><path d="M14 2v2"/><path d="M6 2v2"/>
    <path d="M16 8a1 1 0 0 1 1 1v8a4 4 0 0 1-4 4H7a4 4 0 0 1-4-4V9a1 1 0 0 1 1-1h14a4 4 0 1 1 0 8h-1"/>
  `,
  // People meetup
  meetup: `
    <path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/>
    <circle cx="9" cy="7" r="4"/>
    <path d="M22 21v-2a4 4 0 0 0-3-3.87"/>
    <path d="M16 3.13a4 4 0 0 1 0 7.75"/>
  `,
  // Bike
  ride_share: `
    <circle cx="18.5" cy="17.5" r="3.5"/>
    <circle cx="5.5" cy="17.5" r="3.5"/>
    <circle cx="15" cy="5" r="1"/>
    <path d="m12 17.5 3-9.5 2 3h4"/>
    <path d="M6 10.5 9 17.5"/>
    <path d="M9 8.5 12 5l3 2.5"/>
  `,
  // Briefcase
  business: `
    <path d="M16 20V4a2 2 0 0 0-2-2h-4a2 2 0 0 0-2 2v16"/>
    <rect width="20" height="14" x="2" y="6" rx="2"/>
  `,
}

function svgGlyph(typeId: EventTypeId | null, size: number, stroke: string) {
  const key: EventTypeId = typeId && ICON_SVG[typeId] ? typeId : 'meetup'
  return `<svg xmlns="http://www.w3.org/2000/svg" width="${size}" height="${size}" viewBox="0 0 24 24" fill="none" stroke="${stroke}" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${ICON_SVG[key]}</svg>`
}

export type EventMapIconOpts = {
  focused?: boolean
  label?: string
  /** Used with category to resolve type for older events missing category */
  title?: string
}

/**
 * Leaflet divIcon: teardrop pin + type glyph (coffee / meetup / bike / briefcase).
 */
export function eventMapDivIcon(category?: string | null, opts: EventMapIconOpts = {}) {
  const typeId = resolveEventCategory(category, opts.title || opts.label)
  const t = typeId ? getEventType(typeId) : null
  const color = t?.color ?? '#94a3b8'
  const typeLabel = t?.label ?? 'Event'
  const focused = !!opts.focused
  const w = focused ? 48 : 40
  const h = focused ? 62 : 52
  const glyph = svgGlyph(typeId, focused ? 18 : 15, '#0b1224')
  const labelHtml = opts.label
    ? `<span class="fn-event-pin__label">${escapeHtml(shortLabel(opts.label))}</span>`
    : ''

  const html = `
    <div class="fn-event-pin${focused ? ' fn-event-pin--focus' : ''}" title="${escapeAttr(typeLabel)}" style="--pin:${color}">
      <div class="fn-event-pin__bubble">${glyph}</div>
      <div class="fn-event-pin__stem"></div>
      ${labelHtml}
    </div>
  `

  return L.divIcon({
    className: 'fn-event-marker',
    html,
    iconSize: [w, h],
    iconAnchor: [w / 2, focused ? 56 : 48],
    popupAnchor: [0, -44],
  })
}

export function eventTypeLegendItems() {
  return EVENT_TYPES.map((t) => ({
    id: t.id,
    label: t.label.replace(/ event$/i, ''),
    color: t.color,
    Icon: t.Icon,
  }))
}

function shortLabel(s: string) {
  const t = s.trim()
  return t.length <= 22 ? t : `${t.slice(0, 20)}…`
}

function escapeHtml(s: string) {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}

function escapeAttr(s: string) {
  return escapeHtml(s).replace(/'/g, '&#39;')
}
