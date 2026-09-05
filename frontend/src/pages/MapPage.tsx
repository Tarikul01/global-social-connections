import { useEffect, useMemo, useRef, useState, Fragment, type ReactNode } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { MapContainer, TileLayer, Marker, Popup, Polyline, useMap, useMapEvents } from 'react-leaflet'
import L from 'leaflet'
import { CalendarDays, Filter, LocateFixed, MapPin, Minus, Plus, Radio, Users, X } from 'lucide-react'
import { useAuth } from '../auth/AuthContext'
import { api, type NearbyUser, type Profession } from '../lib/api'
import { Recenter } from '../components/Recenter'
import { useMapSocket } from '../hooks/useMapSocket'
import { UserAvatar } from '../components/UserAvatar'
import { MapUserPeek } from '../components/MapUserPeek'
import { userShortform } from '../lib/avatar'
import { EVENT_TYPES, eventTypeLabel, resolveEventCategory } from '../lib/eventTypes'
import { eventMapDivIcon } from '../lib/eventMapIcons'
import type { EventDetail, EventSummary } from '../lib/events'

type EventMapMember = {
  id: string
  username: string
  full_name: string
  role: string
  initials: string
  lat: number
  lng: number
  online?: boolean
  live: boolean
}

function eventMemberIcon(initials: string, online: boolean, live: boolean) {
  const short = escapeHtml(initials || '?')
  const classes = [
    'disc-marker',
    online ? 'online' : 'offline',
    live ? 'live' : '',
    'event-attendee',
  ]
    .filter(Boolean)
    .join(' ')
  return L.divIcon({
    className: '',
    html: `<div class="${classes}"><span class="disc-initials">${short}</span><span class="dot"></span></div>`,
    iconSize: [40, 40],
    iconAnchor: [20, 20],
  })
}

function FitBounds({
  points,
  tick,
}: {
  points: [number, number][]
  tick: number
}) {
  const map = useMap()
  const last = useRef(0)
  useEffect(() => {
    if (tick === last.current || points.length === 0) return
    last.current = tick
    if (points.length === 1) {
      map.setView(points[0], 15)
      return
    }
    map.fitBounds(L.latLngBounds(points), { padding: [56, 56], maxZoom: 15 })
  }, [map, points, tick])
  return null
}

function markerIcon(u: NearbyUser, loggedIn: boolean) {
  const status = u.online ? 'online' : 'offline'
  const me = u.is_me ? ' me' : ''
  const live = u.live ? ' live' : ''
  const short = escapeHtml(
    userShortform({
      initials: u.initials,
      full_name: loggedIn ? u.full_name : undefined,
      username: u.username,
    }),
  )
  // Always render shortform; photo sits on top and hides itself if it fails to load.
  const img = u.photo_url
    ? `<img src="${escapeAttr(u.photo_url)}" alt="" onerror="this.style.display='none'">`
    : ''
  const html = `<div class="disc-marker ${status}${me}${live}"><span class="disc-initials">${short}</span>${img}<span class="dot"></span></div>`
  return L.divIcon({
    className: '',
    html,
    iconSize: [46, 46],
    iconAnchor: [23, 23],
  })
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

function zoomToRadius(zoom: number) {
  if (zoom >= 16) return 5
  if (zoom >= 14) return 15
  if (zoom >= 12) return 40
  if (zoom >= 10) return 120
  if (zoom >= 8) return 400
  if (zoom >= 5) return 1500
  return 5000
}

function haversineM(lat1: number, lng1: number, lat2: number, lng2: number) {
  const R = 6371000
  const toRad = (d: number) => (d * Math.PI) / 180
  const dLat = toRad(lat2 - lat1)
  const dLng = toRad(lng2 - lng1)
  const a =
    Math.sin(dLat / 2) ** 2 +
    Math.cos(toRad(lat1)) * Math.cos(toRad(lat2)) * Math.sin(dLng / 2) ** 2
  return 2 * R * Math.asin(Math.sqrt(a))
}

function MapEvents({ onMove }: { onMove: (lat: number, lng: number, zoom: number) => void }) {
  const map = useMapEvents({
    moveend: () => {
      const c = map.getCenter()
      onMove(c.lat, c.lng, map.getZoom())
    },
  })
  return null
}

function MapControls({
  onLocate,
  loggedIn,
  liveOn,
  liveBusy,
  onToggleLive,
  panelOpen,
  onTogglePanel,
}: {
  onLocate: () => void
  loggedIn: boolean
  liveOn: boolean
  liveBusy: boolean
  onToggleLive: () => void
  panelOpen: boolean
  onTogglePanel: () => void
}) {
  const map = useMap()
  const right = panelOpen ? 'right-[min(100%,22rem)] mr-3' : 'right-3'
  return (
    <div className={`absolute top-3 z-[1000] flex flex-col gap-2 ${right}`}>
      <button
        type="button"
        className="flex h-10 w-10 items-center justify-center rounded-full bg-[#0b1224] text-white shadow-lg ring-1 ring-white/15"
        onClick={() => map.zoomIn()}
        title="Zoom in"
      >
        <Plus size={18} />
      </button>
      <button
        type="button"
        className="flex h-10 w-10 items-center justify-center rounded-full bg-[#0b1224] text-white shadow-lg ring-1 ring-white/15"
        onClick={() => map.zoomOut()}
        title="Zoom out"
      >
        <Minus size={18} />
      </button>
      <button
        type="button"
        className="flex h-10 w-10 items-center justify-center rounded-full bg-emerald-600 text-white shadow-lg"
        onClick={onLocate}
        title="Go to my saved location"
      >
        <LocateFixed size={18} />
      </button>
      {loggedIn && (
        <button
          type="button"
          disabled={liveBusy}
          className={`flex h-10 w-10 items-center justify-center rounded-full text-white shadow-lg ring-1 disabled:opacity-60 ${
            liveOn
              ? 'bg-red-500 ring-red-300/40 animate-pulse'
              : 'bg-yellow-500 ring-yellow-300/30 text-slate-900'
          }`}
          onClick={onToggleLive}
          title={liveOn ? 'Stop sharing live location' : 'Share my live location'}
          aria-pressed={liveOn}
        >
          {liveOn ? <Radio size={18} /> : <MapPin size={18} />}
        </button>
      )}
      <button
        type="button"
        className={`flex h-10 w-10 items-center justify-center rounded-full text-white shadow-lg ring-1 ${
          panelOpen ? 'bg-blue-600 ring-blue-400/40' : 'bg-[#0b1224] ring-white/15'
        }`}
        onClick={onTogglePanel}
        title={panelOpen ? 'Hide people' : 'Show people nearby'}
        aria-pressed={panelOpen}
      >
        <Users size={18} />
      </button>
    </div>
  )
}

function hasEventCoords(e: { lat?: number; lng?: number }) {
  return (
    typeof e.lat === 'number' &&
    typeof e.lng === 'number' &&
    Number.isFinite(e.lat) &&
    Number.isFinite(e.lng) &&
    !(e.lat === 0 && e.lng === 0)
  )
}

export function MapPage() {
  const { user } = useAuth()
  const [searchParams] = useSearchParams()
  const loggedIn = !!user
  const [panelOpen, setPanelOpen] = useState(false)
  const [selected, setSelected] = useState<NearbyUser | null>(null)
  const [users, setUsers] = useState<NearbyUser[]>([])
  const [total, setTotal] = useState(0)
  const [online, setOnline] = useState(0)
  const [search, setSearch] = useState('')
  const [status, setStatus] = useState('')
  const [professions, setProfessions] = useState<Profession[]>([])
  const [selectedProfessions, setSelectedProfessions] = useState<string[]>([])
  const [filterOpen, setFilterOpen] = useState(false)
  const [eventsOpen, setEventsOpen] = useState(false)
  /** When true, only the focused event pin is shown (hide other events). */
  const [soloEvent, setSoloEvent] = useState(false)
  const [center, setCenter] = useState<[number, number]>([23.8103, 90.4125])
  const [zoom, setZoom] = useState(12)
  const [viewTick, setViewTick] = useState(0)
  const [mapReady, setMapReady] = useState(false)
  const [eventFocus, setEventFocus] = useState<{
    lat: number
    lng: number
    id?: string
    label?: string
    type?: string
  } | null>(null)
  const [mapEvents, setMapEvents] = useState<EventSummary[]>([])
  const [eventMembers, setEventMembers] = useState<EventMapMember[]>([])
  const [routeTick, setRouteTick] = useState(0)
  const focusAppliedRef = useRef(false)
  const [socketOk, setSocketOk] = useState(false)
  const [liveOn, setLiveOn] = useState(false)
  const [liveBusy, setLiveBusy] = useState(false)
  /** Own GPS while live sharing — used for event direction lines. */
  const [myLiveCoords, setMyLiveCoords] = useState<{ lat: number; lng: number } | null>(null)
  const watchIdRef = useRef<number | null>(null)
  const lastLiveSentRef = useRef<{ t: number; lat: number; lng: number } | null>(null)
  const liveWatchActiveRef = useRef(false)
  const liveUpdateRef = useRef<(lat: number, lng: number) => boolean>(() => false)

  const stopLiveWatch = () => {
    liveWatchActiveRef.current = false
    lastLiveSentRef.current = null
    setMyLiveCoords(null)
    if (watchIdRef.current != null && navigator.geolocation) {
      navigator.geolocation.clearWatch(watchIdRef.current)
      watchIdRef.current = null
    }
  }

  const startLiveWatch = () => {
    if (!navigator.geolocation) return
    stopLiveWatch()
    liveWatchActiveRef.current = true
    watchIdRef.current = navigator.geolocation.watchPosition(
      (pos) => {
        const lat = pos.coords.latitude
        const lng = pos.coords.longitude
        setMyLiveCoords({ lat, lng })
        const now = Date.now()
        const prev = lastLiveSentRef.current
        const moved = !prev || haversineM(prev.lat, prev.lng, lat, lng) >= 25
        const due = !prev || now - prev.t >= 5000
        if (!moved && !due) return
        lastLiveSentRef.current = { t: now, lat, lng }
        liveUpdateRef.current(lat, lng)
      },
      () => {},
      { enableHighAccuracy: true, maximumAge: 5000, timeout: 15000 },
    )
  }

  const subscribe = mapReady
    ? {
        lat: center[0],
        lng: center[1],
        radius_km: zoomToRadius(zoom),
        status,
        q: search.trim(),
        professions: selectedProfessions,
      }
    : null

  const { liveStart, liveUpdate, liveStop } = useMapSocket({
    enabled: mapReady,
    authKey: user?.id || 'guest',
    subscribe,
    onNearby: (data) => {
      setUsers(data.users || [])
      setTotal(data.total || 0)
      setOnline(data.online || 0)
      if (typeof data.live === 'boolean') {
        setLiveOn(data.live)
        if (data.live && !liveWatchActiveRef.current) {
          startLiveWatch()
        } else if (!data.live) {
          stopLiveWatch()
        }
      }
    },
    onLive: (data) => {
      if (typeof data.live === 'boolean') setLiveOn(data.live)
    },
    onStatus: setSocketOk,
  })
  liveUpdateRef.current = liveUpdate

  useEffect(() => {
    api
      .get<{ success: boolean; data: Profession[] }>('/api/v1/professions')
      .then((r) => setProfessions(r.data || []))
      .catch(() => {})
  }, [])

  // Focus map on event pin when opened via ?lat=&lng=&event=&label=&type=
  useEffect(() => {
    const lat = Number(searchParams.get('lat'))
    const lng = Number(searchParams.get('lng'))
    if (!Number.isFinite(lat) || !Number.isFinite(lng)) {
      setEventFocus(null)
      setSoloEvent(false)
      focusAppliedRef.current = false
      return
    }
    const z = Number(searchParams.get('zoom'))
    const zoomTo = Number.isFinite(z) && z > 0 ? z : 15
    const label = searchParams.get('label') || undefined
    const type =
      searchParams.get('type') ||
      resolveEventCategory(null, label) ||
      undefined
    setEventFocus({
      lat,
      lng,
      id: searchParams.get('event') || undefined,
      label,
      type,
    })
    setSoloEvent(true)
    setCenter([lat, lng])
    setZoom(zoomTo)
    setViewTick((t) => t + 1)
    focusAppliedRef.current = true
  }, [searchParams])

  // Resolve missing type/label from event API (older events may lack ?type=)
  useEffect(() => {
    const id = eventFocus?.id
    if (!id) return
    if (eventFocus.type && eventFocus.label) return
    let cancelled = false
    void api
      .get<{ success: boolean; data: EventDetail }>(`/api/v1/events/${id}`)
      .then((r) => {
        if (cancelled || !r.data?.event) return
        const e = r.data.event
        const type = resolveEventCategory(e.category, e.title) || undefined
        setEventFocus((prev) => {
          if (!prev || prev.id !== id) return prev
          return {
            ...prev,
            label: prev.label || e.title,
            type: prev.type || type,
            lat: hasEventCoords(e) ? (e.lat as number) : prev.lat,
            lng: hasEventCoords(e) ? (e.lng as number) : prev.lng,
          }
        })
      })
      .catch(() => {})
    return () => {
      cancelled = true
    }
  }, [eventFocus?.id, eventFocus?.type, eventFocus?.label])

  // Going members once when focusing an event (live positions come from socket + GPS)
  useEffect(() => {
    const id = eventFocus?.id
    if (!id || !soloEvent || !loggedIn) {
      setEventMembers([])
      return
    }
    let cancelled = false
    void api
      .get<{
        success: boolean
        data: { event: { lat: number; lng: number }; members: EventMapMember[] }
      }>(`/api/v1/events/${id}/map-members`)
      .then((r) => {
        if (cancelled) return
        const members = (r.data?.members || []).filter(
          (m) =>
            typeof m.lat === 'number' &&
            typeof m.lng === 'number' &&
            Number.isFinite(m.lat) &&
            Number.isFinite(m.lng),
        )
        setEventMembers(members)
        if (r.data?.event?.lat != null && r.data?.event?.lng != null) {
          setEventFocus((prev) =>
            prev && prev.id === id
              ? { ...prev, lat: r.data.event.lat, lng: r.data.event.lng }
              : prev,
          )
        }
      })
      .catch(() => {
        if (!cancelled) setEventMembers([])
      })
    return () => {
      cancelled = true
    }
  }, [eventFocus?.id, soloEvent, loggedIn])

  // Plot published events with coordinates on the discover map
  useEffect(() => {
    if (!loggedIn) {
      setMapEvents([])
      return
    }
    let cancelled = false
    void api
      .get<{ success: boolean; data: EventSummary[] }>('/api/v1/events')
      .then((r) => {
        if (cancelled) return
        const list = (r.data || []).map((e) => ({
          ...e,
          lat: typeof e.lat === 'number' ? e.lat : Number(e.lat),
          lng: typeof e.lng === 'number' ? e.lng : Number(e.lng),
        }))
        setMapEvents(list)
      })
      .catch(() => {
        if (!cancelled) setMapEvents([])
      })
    return () => {
      cancelled = true
    }
  }, [loggedIn])

  useEffect(() => {
    if (!user) return
    // Don't override an explicit event focus from the URL.
    if (focusAppliedRef.current) return
    api
      .get<{ success: boolean; data: { lat?: number; lng?: number } }>('/api/v1/me/profile')
      .then((r) => {
        if (focusAppliedRef.current) return
        if (typeof r.data?.lat === 'number' && typeof r.data?.lng === 'number') {
          setCenter([r.data.lat, r.data.lng])
          setZoom(14)
          setViewTick((t) => t + 1)
        }
      })
      .catch(() => {})

    // Save GPS pin + enable discovery so the user counts toward USERS / appears on map.
    if (!navigator.geolocation) return
    navigator.geolocation.getCurrentPosition(
      (pos) => {
        if (focusAppliedRef.current) return
        void api
          .post('/api/v1/me/location', {
            lat: pos.coords.latitude,
            lng: pos.coords.longitude,
          })
          .then(() => {
            if (focusAppliedRef.current) return
            setCenter([pos.coords.latitude, pos.coords.longitude])
            setZoom(14)
            setViewTick((t) => t + 1)
          })
          .catch(() => {})
      },
      () => {},
      { enableHighAccuracy: true, timeout: 12000, maximumAge: 60000 },
    )
  }, [user])

  // HTTP fallback when WebSocket is down — one-shot only (no polling; socket is primary).
  useEffect(() => {
    if (!mapReady || socketOk) return
    const params = new URLSearchParams({
      lat: String(center[0]),
      lng: String(center[1]),
      radius_km: String(zoomToRadius(zoom)),
    })
    if (status) params.set('status', status)
    if (search.trim()) params.set('q', search.trim())
    for (const p of selectedProfessions) params.append('professions', p)
    void api
      .get<{
        success: boolean
        data: { users?: NearbyUser[]; total?: number; online?: number; live?: boolean }
      }>(`/api/v1/users/nearby?${params}`)
      .then((r) => {
        setUsers(r.data?.users || [])
        setTotal(r.data?.total || 0)
        setOnline(r.data?.online || 0)
        if (typeof r.data?.live === 'boolean') setLiveOn(r.data.live)
      })
      .catch(() => {})
  }, [mapReady, socketOk, center, zoom, status, search, selectedProfessions])

  useEffect(() => {
    if (!user) {
      setLiveOn(false)
      stopLiveWatch()
    }
  }, [user])

  useEffect(() => {
    return () => stopLiveWatch()
  }, [])

  const toggleLive = async () => {
    if (!loggedIn || liveBusy) return
    setLiveBusy(true)
    try {
      if (liveOn) {
        stopLiveWatch()
        liveStop()
        setLiveOn(false)
      } else {
        const coords = await new Promise<GeolocationCoordinates>((resolve, reject) => {
          if (!navigator.geolocation) {
            reject(new Error('Geolocation not supported'))
            return
          }
          navigator.geolocation.getCurrentPosition(
            (p) => resolve(p.coords),
            reject,
            { enableHighAccuracy: true, timeout: 10000 },
          )
        })
        const ok = liveStart(coords.latitude, coords.longitude)
        if (!ok) throw new Error('socket not connected')
        setLiveOn(true)
        setCenter([coords.latitude, coords.longitude])
        setZoom(16)
        setViewTick((t) => t + 1)
        startLiveWatch()
      }
    } catch {
      setLiveOn(false)
      stopLiveWatch()
    } finally {
      setLiveBusy(false)
    }
  }

  const displayName = (u: NearbyUser) => {
    if (loggedIn && u.full_name) return u.full_name
    return u.username ? `@${u.username}` : 'Member'
  }

  const displayMeta = (u: NearbyUser) => {
    const profession = u.profession || 'Professional'
    if (!loggedIn) return profession
    const bits = [profession]
    if (typeof u.distance_km === 'number') bits.push(`${u.distance_km} km`)
    bits.push(u.online ? 'Online' : 'Offline')
    return bits.join(' · ')
  }

  const openPerson = (u: NearbyUser) => {
    if (!u.username) return
    setSelected(u)
  }

  const publicEvents = useMemo(() => {
    const byId = new Map<string, EventSummary>()
    for (const e of mapEvents) {
      if (e.visibility === 'private') continue
      byId.set(e.id, e)
    }
    // Keep focused event visible in the count/list even if list API lagged.
    if (eventFocus?.id && hasEventCoords(eventFocus)) {
      const existing = byId.get(eventFocus.id)
      if (!existing) {
        byId.set(eventFocus.id, {
          id: eventFocus.id,
          creator_id: '',
          title: eventFocus.label || 'Event',
          category: eventFocus.type,
          event_date: '',
          location_name: '',
          lat: eventFocus.lat,
          lng: eventFocus.lng,
          visibility: 'public',
          member_count: 0,
        })
      }
    }
    return Array.from(byId.values())
  }, [mapEvents, eventFocus])

  const mappablePublicEvents = useMemo(
    () => publicEvents.filter(hasEventCoords),
    [publicEvents],
  )

  const eventsByCategory = useMemo(() => {
    return EVENT_TYPES.map((t) => ({
      ...t,
      events: mappablePublicEvents.filter(
        (e) => resolveEventCategory(e.category, e.title) === t.id,
      ),
    })).filter((g) => g.events.length > 0)
  }, [mappablePublicEvents])

  const focusEventOnMap = (ev: EventSummary) => {
    if (!hasEventCoords(ev)) return
    const type = resolveEventCategory(ev.category, ev.title) || undefined
    setEventFocus({
      lat: ev.lat as number,
      lng: ev.lng as number,
      id: ev.id,
      label: ev.title,
      type,
    })
    setSoloEvent(true)
    setCenter([ev.lat as number, ev.lng as number])
    setZoom(15)
    setViewTick((n) => n + 1)
    setEventsOpen(false)
    setFilterOpen(false)
  }

  const clearEventFocus = () => {
    setEventFocus(null)
    setSoloEvent(false)
    setEventMembers([])
    setEventsOpen(false)
  }

  const routeAttendees = useMemo(() => {
    return eventMembers.map((m) => {
      const nearby = users.find((u) => u.username === m.username)
      const isMe = !!user?.username && m.username === user.username
      // Presence: DB heartbeat, nearby stream, or self while map socket is up.
      const online = Boolean(m.online || nearby?.online || (isMe && loggedIn && socketOk))

      // Prefer own live GPS immediately while sharing.
      if (liveOn && myLiveCoords && isMe) {
        return {
          ...m,
          lat: myLiveCoords.lat,
          lng: myLiveCoords.lng,
          live: true,
          online: true,
        }
      }
      // Prefer other members' live pins from nearby stream.
      if (nearby?.live && typeof nearby.lat === 'number' && typeof nearby.lng === 'number') {
        return {
          ...m,
          lat: nearby.lat,
          lng: nearby.lng,
          live: true,
          online: nearby.online ?? online,
        }
      }
      return { ...m, online }
    })
  }, [eventMembers, liveOn, loggedIn, myLiveCoords, socketOk, user?.username, users])

  const routeBounds = useMemo((): [number, number][] => {
    if (!eventFocus || !soloEvent) return []
    const pts: [number, number][] = [[eventFocus.lat, eventFocus.lng]]
    for (const m of routeAttendees) {
      pts.push([m.lat, m.lng])
    }
    return pts
  }, [eventFocus, routeAttendees, soloEvent])

  // Fit once when the going-list set changes (not on every GPS tick)
  const memberKey = eventMembers.map((m) => m.id).join(',')
  useEffect(() => {
    if (!soloEvent || !memberKey) return
    setRouteTick((t) => t + 1)
  }, [soloEvent, memberKey, eventFocus?.id])

  const attendeeUsernames = useMemo(
    () => new Set(routeAttendees.map((m) => m.username)),
    [routeAttendees],
  )

  /** Own pin: live GPS + heartbeat class as soon as live sharing is on. */
  const mapUsers = useMemo(() => {
    return users.map((u) => {
      const isMe = u.is_me || (!!user?.username && u.username === user.username)
      if (!liveOn || !isMe) return u
      return {
        ...u,
        is_me: true,
        online: true,
        live: true,
        ...(myLiveCoords
          ? { lat: myLiveCoords.lat, lng: myLiveCoords.lng }
          : null),
      }
    })
  }, [liveOn, myLiveCoords, user?.username, users])

  const markers = useMemo(
    () =>
      mapUsers
        // In event focus mode, going members are shown as attendee pins — don't double-draw.
        .filter((u) => !(soloEvent && attendeeUsernames.has(u.username)))
        .map((u) => {
        const isOpen = selected?.username === u.username
        return (
          <Marker
            key={`${u.username}-${u.lat}-${u.lng}-${u.live ? 'live' : 'fix'}`}
            position={[u.lat, u.lng]}
            icon={markerIcon(u, loggedIn)}
            eventHandlers={{
              click: () => openPerson(u),
            }}
          >
            {isOpen && (
              <Popup
                className="fn-user-peek"
                offset={[0, -8]}
                closeButton
                autoPan
                autoPanPadding={[48, 48]}
                eventHandlers={{
                  remove: () => setSelected(null),
                }}
              >
                <MapUserPeek user={u} loggedIn={loggedIn} />
              </Popup>
            )}
          </Marker>
        )
      }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [mapUsers, loggedIn, selected?.username, soloEvent, attendeeUsernames],
  )

  const eventMarkers = useMemo(() => {
    const nodes: ReactNode[] = []

    if (!soloEvent) {
      for (const ev of mapEvents) {
        if (!hasEventCoords(ev)) continue
        if (eventFocus?.id && ev.id === eventFocus.id) continue
        const type = resolveEventCategory(ev.category, ev.title)
        nodes.push(
          <Marker
            key={`ev-${ev.id}`}
            position={[ev.lat as number, ev.lng as number]}
            icon={eventMapDivIcon(type, { label: ev.title, title: ev.title })}
            zIndexOffset={200}
            eventHandlers={{
              click: () => focusEventOnMap(ev),
            }}
          >
            <Popup>
              <div className="min-w-[160px] text-sm text-slate-900">
                <div className="font-semibold">{ev.title}</div>
                <div className="mt-0.5 text-xs text-slate-600">
                  {eventTypeLabel(ev.category, ev.title)}
                </div>
                {ev.location_name && (
                  <div className="mt-1 text-xs text-slate-500">{ev.location_name}</div>
                )}
                <Link to={`/e/${ev.id}`} className="mt-2 inline-block text-blue-600 underline">
                  Open event
                </Link>
              </div>
            </Popup>
          </Marker>,
        )
      }
    }

    if (eventFocus) {
      nodes.push(
        <Marker
          key={`ev-focus-${eventFocus.id || 'pin'}`}
          position={[eventFocus.lat, eventFocus.lng]}
          icon={eventMapDivIcon(eventFocus.type, {
            focused: true,
            label: eventFocus.label,
            title: eventFocus.label,
          })}
          zIndexOffset={500}
        >
          <Popup>
            <div className="min-w-[160px] text-sm text-slate-900">
              <div className="font-semibold">{eventFocus.label || 'Event location'}</div>
              <div className="mt-0.5 text-xs text-slate-600">
                {eventTypeLabel(eventFocus.type, eventFocus.label)}
              </div>
              {eventFocus.id && (
                <Link
                  to={`/e/${eventFocus.id}`}
                  className="mt-2 inline-block text-blue-600 underline"
                >
                  Open event
                </Link>
              )}
            </div>
          </Popup>
        </Marker>,
      )
    }

    return nodes
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [mapEvents, eventFocus, soloEvent])

  const attendeeLayers = useMemo(() => {
    if (!soloEvent || !eventFocus || routeAttendees.length === 0) return null
    const evLat = eventFocus.lat
    const evLng = eventFocus.lng
    return (
      <>
        {routeAttendees.map((m) => {
          const nearVenue = haversineM(m.lat, m.lng, evLat, evLng) < 50
          return (
            <Fragment key={`att-${m.id}`}>
              {!nearVenue && (
                <>
                  <Polyline
                    positions={[
                      [m.lat, m.lng],
                      [evLat, evLng],
                    ]}
                    pathOptions={{
                      color: '#991b1b',
                      weight: 7,
                      opacity: 0.28,
                      lineCap: 'round',
                    }}
                  />
                  <Polyline
                    positions={[
                      [m.lat, m.lng],
                      [evLat, evLng],
                    ]}
                    pathOptions={{
                      color: m.online ? '#22c55e' : '#ef4444',
                      weight: 3.5,
                      opacity: 0.95,
                      dashArray: '10 7',
                      lineCap: 'round',
                    }}
                    eventHandlers={{
                      click: () => {
                        setCenter([(m.lat + evLat) / 2, (m.lng + evLng) / 2])
                        setZoom(14)
                        setViewTick((t) => t + 1)
                        setRouteTick((t) => t + 1)
                      },
                    }}
                  />
                </>
              )}
              <Marker
                position={[m.lat, m.lng]}
                icon={eventMemberIcon(m.initials, !!m.online, !!m.live)}
                zIndexOffset={400}
              >
                <Popup>
                  <div className="min-w-[150px] text-sm text-slate-900">
                    <div className="font-semibold">{m.full_name}</div>
                    <div className="text-xs text-slate-500">
                      @{m.username}
                      {m.online ? ' · Online' : ' · Offline'}
                      {m.live ? ' · Live location' : ' · Profile location'}
                      {nearVenue ? ' · At venue' : ' · Going'}
                    </div>
                    {!nearVenue && (
                      <button
                        type="button"
                        className="mt-2 text-blue-600 underline"
                        onClick={() => setRouteTick((t) => t + 1)}
                      >
                        Show route on map
                      </button>
                    )}
                  </div>
                </Popup>
              </Marker>
            </Fragment>
          )
        })}
      </>
    )
  }, [soloEvent, eventFocus, routeAttendees])

  return (
    <div className="relative h-full w-full overflow-hidden">
      <div className="absolute left-1/2 top-3 z-[1000] flex -translate-x-1/2 items-center gap-2 rounded-full border border-white/10 bg-[#0b1224]/95 px-3 py-1.5 text-xs shadow-lg backdrop-blur lg:left-[calc(50%+9rem)]">
        <span
          className={`h-2 w-2 rounded-full ${socketOk ? 'bg-emerald-400' : 'bg-slate-500'}`}
          title={socketOk ? 'Realtime connected' : 'Connecting…'}
        />
        <span className="text-slate-400">
          USERS <strong className="text-white">{total}</strong>
        </span>
        <span className="text-slate-600">|</span>
        <span className="text-emerald-400">
          ONLINE <strong>{online}</strong>
        </span>
        <button
          type="button"
          onClick={() => {
            setEventsOpen((v) => !v)
            setFilterOpen(false)
          }}
          className={`ml-1 inline-flex items-center gap-1 rounded-full px-2.5 py-1 font-semibold ${
            eventsOpen || soloEvent
              ? 'bg-amber-400/20 text-amber-100'
              : 'bg-white/10 text-slate-100'
          }`}
        >
          <CalendarDays size={12} /> Events
          <span className="rounded-full bg-amber-400 px-1.5 text-[10px] font-bold text-slate-900">
            {publicEvents.length}
          </span>
        </button>
        <button
          type="button"
          onClick={() => {
            setFilterOpen((v) => !v)
            setEventsOpen(false)
          }}
          className={`inline-flex items-center gap-1 rounded-full px-2.5 py-1 font-semibold ${
            selectedProfessions.length
              ? 'bg-yellow-400/20 text-yellow-200'
              : 'bg-blue-500/20 text-blue-100'
          }`}
        >
          <Filter size={12} /> Filter
          {selectedProfessions.length > 0 && (
            <span className="rounded-full bg-yellow-400 px-1.5 text-[10px] font-bold text-slate-900">
              {selectedProfessions.length}
            </span>
          )}
        </button>
      </div>

      {eventsOpen && (
        <div className="absolute left-1/2 top-14 z-[1000] w-[min(92vw,22rem)] -translate-x-1/2 rounded-2xl border border-white/10 bg-[#0b1224] p-3 shadow-xl lg:left-[calc(50%+9rem)]">
          <div className="mb-2 flex items-center justify-between gap-2">
            <span className="text-xs font-bold uppercase tracking-wide text-slate-400">
              Public events
            </span>
            {soloEvent && (
              <button
                type="button"
                className="text-xs text-amber-300 underline"
                onClick={clearEventFocus}
              >
                Show all
              </button>
            )}
          </div>
          {mappablePublicEvents.length === 0 ? (
            <p className="px-1 py-3 text-sm text-slate-400">No public events with a location yet.</p>
          ) : (
            <div className="max-h-72 space-y-3 overflow-y-auto">
              {eventsByCategory.map((group) => {
                const Icon = group.Icon
                return (
                  <div key={group.id}>
                    <div
                      className="mb-1 flex items-center gap-1.5 px-1 text-[11px] font-semibold uppercase tracking-wide"
                      style={{ color: group.color }}
                    >
                      <Icon size={12} />
                      {group.label.replace(/ event$/i, '')}
                      <span className="text-slate-500">({group.events.length})</span>
                    </div>
                    <div className="space-y-1">
                      {group.events.map((ev) => {
                        const active = soloEvent && eventFocus?.id === ev.id
                        return (
                          <button
                            key={ev.id}
                            type="button"
                            onClick={() => focusEventOnMap(ev)}
                            className={`flex w-full items-start gap-2 rounded-lg px-2 py-2 text-left text-sm transition ${
                              active
                                ? 'bg-amber-400/15 text-amber-50 ring-1 ring-amber-400/40'
                                : 'hover:bg-white/5'
                            }`}
                          >
                            <span
                              className="mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-full"
                              style={{ background: `${group.color}33`, color: group.color }}
                            >
                              <Icon size={14} />
                            </span>
                            <span className="min-w-0 flex-1">
                              <span className="block truncate font-medium text-white">{ev.title}</span>
                              <span className="block truncate text-xs text-slate-400">
                                {ev.location_name || 'On map'}
                                {ev.event_date_label || ev.event_date
                                  ? ` · ${ev.event_date_label || ev.event_date}`
                                  : ''}
                              </span>
                            </span>
                          </button>
                        )
                      })}
                    </div>
                  </div>
                )
              })}
              {mappablePublicEvents.filter((e) => !resolveEventCategory(e.category, e.title))
                .length > 0 && (
                <div>
                  <div className="mb-1 px-1 text-[11px] font-semibold uppercase tracking-wide text-slate-500">
                    Other
                  </div>
                  {mappablePublicEvents
                    .filter((e) => !resolveEventCategory(e.category, e.title))
                    .map((ev) => (
                      <button
                        key={ev.id}
                        type="button"
                        onClick={() => focusEventOnMap(ev)}
                        className="flex w-full rounded-lg px-2 py-2 text-left text-sm hover:bg-white/5"
                      >
                        <span className="truncate font-medium text-white">{ev.title}</span>
                      </button>
                    ))}
                </div>
              )}
            </div>
          )}
        </div>
      )}

      {filterOpen && (
        <div className="absolute left-1/2 top-14 z-[1000] w-[min(92vw,22rem)] -translate-x-1/2 rounded-2xl border border-white/10 bg-[#0b1224] p-3 shadow-xl">
          <div className="mb-2 flex items-center justify-between">
            <span className="text-xs font-bold uppercase tracking-wide text-slate-400">
              Filter by designation
            </span>
            <button
              type="button"
              className="text-xs text-slate-400 underline"
              onClick={() => setSelectedProfessions([])}
            >
              Clear
            </button>
          </div>
          <div className="max-h-56 space-y-1 overflow-y-auto">
            {professions.map((p) => {
              const on = selectedProfessions.includes(p.slug)
              return (
                <label
                  key={p.slug}
                  className={`flex cursor-pointer items-center gap-2 rounded-lg px-2 py-1.5 text-sm ${
                    on ? 'bg-yellow-400/10 text-yellow-100' : 'hover:bg-white/5'
                  }`}
                >
                  <input
                    type="checkbox"
                    checked={on}
                    onChange={() =>
                      setSelectedProfessions((prev) =>
                        on ? prev.filter((s) => s !== p.slug) : [...prev, p.slug],
                      )
                    }
                  />
                  <span className="flex-1">{p.name}</span>
                  <span className="text-xs text-slate-500">{p.count}</span>
                </label>
              )
            })}
          </div>
        </div>
      )}

      <MapContainer
        center={center}
        zoom={zoom}
        className="h-full w-full"
        zoomControl={false}
        ref={() => {
          if (!mapReady) setMapReady(true)
        }}
      >
        <TileLayer
          attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
          url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
        />
        <Recenter center={center} zoom={zoom} tick={viewTick} />
        {soloEvent && routeBounds.length > 0 && (
          <FitBounds points={routeBounds} tick={routeTick} />
        )}
        <MapEvents
          onMove={(lat, lng, z) => {
            setCenter([lat, lng])
            setZoom(z)
          }}
        />
        <MapControls
          loggedIn={loggedIn}
          liveOn={liveOn}
          liveBusy={liveBusy}
          onToggleLive={() => {
            void toggleLive()
          }}
          panelOpen={panelOpen}
          onTogglePanel={() => setPanelOpen((v) => !v)}
          onLocate={() => {
            if (eventFocus) {
              setCenter([eventFocus.lat, eventFocus.lng])
              setZoom(15)
              setViewTick((t) => t + 1)
              setRouteTick((t) => t + 1)
              return
            }
            const me = users.find((u) => u.is_me)
            if (me) {
              setCenter([me.lat, me.lng])
              setZoom(16)
              setViewTick((t) => t + 1)
            }
          }}
        />
        {attendeeLayers}
        {eventMarkers}
        {markers}
      </MapContainer>

      <aside
        className={`absolute right-0 top-0 z-[1000] flex h-full w-[min(100%,22rem)] flex-col border-l border-white/10 bg-[#0b1224]/95 backdrop-blur transition ${
          panelOpen ? 'translate-x-0' : 'pointer-events-none translate-x-full'
        }`}
        aria-hidden={!panelOpen}
      >
        <div className="flex flex-col gap-2 border-b border-white/10 p-3">
          <div className="flex items-center gap-2">
            <input
              type="search"
              placeholder={
                loggedIn ? 'Search people...' : 'Search by username or designation...'
              }
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="min-w-0 flex-1 rounded-xl border border-white/10 bg-[#111827] px-3 py-2 text-sm outline-none"
            />
            <button
              type="button"
              className="rounded-full bg-white/10 p-2 hover:bg-white/20"
              onClick={() => setPanelOpen(false)}
              aria-label="Close"
            >
              <X size={16} />
            </button>
          </div>
          <select
            value={status}
            onChange={(e) => setStatus(e.target.value)}
            className="rounded-xl border border-white/10 bg-[#111827] px-3 py-2 text-sm font-semibold text-white"
            style={{ colorScheme: 'dark' }}
          >
            <option value="">All status</option>
            <option value="online">Online</option>
            <option value="offline">Offline</option>
          </select>
        </div>

        <div className="flex-1 overflow-y-auto p-2">
          {users.length === 0 ? (
            <p className="p-4 text-center text-sm text-slate-500">No people in this area yet.</p>
          ) : (
            users.map((u) => (
              <button
                key={u.username + String(u.lat)}
                type="button"
                onClick={() => openPerson(u)}
                className="mb-1 flex w-full items-center gap-3 rounded-xl px-2 py-2 text-left hover:bg-white/5"
              >
                <UserAvatar
                  name={loggedIn && u.full_name ? u.full_name : u.username}
                  initials={u.initials}
                  photoUrl={u.photo_url}
                  online={u.online}
                  size={36}
                />
                <span className="min-w-0 flex-1">
                  <span className="block truncate font-semibold">
                    {displayName(u)}
                    {u.is_me ? ' (You)' : ''}
                  </span>
                  <span className="block truncate text-xs text-slate-400">{displayMeta(u)}</span>
                </span>
              </button>
            ))
          )}
        </div>

        <div className="border-t border-white/10 px-4 py-3 text-xs text-slate-300">
          {users.length} nearby · {total} worldwide · {online} online
          {!loggedIn && (
            <div className="mt-2 text-slate-400">
              Guests see avatar, username &amp; designation.{' '}
              <Link to="/login" className="text-blue-300 underline">
                Log in
              </Link>{' '}
              for full profiles.
            </div>
          )}
        </div>
      </aside>
    </div>
  )
}
