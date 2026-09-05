import { useEffect, useMemo, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { MapContainer, TileLayer, Marker, Popup, useMap, useMapEvents } from 'react-leaflet'
import L from 'leaflet'
import { Filter, LocateFixed, MapPin, Menu, Minus, Plus, Radio, Users, X } from 'lucide-react'
import { useAuth } from '../auth/AuthContext'
import { api, type NearbyUser, type Profession } from '../lib/api'
import { SideNav } from '../components/SideNav'
import { Recenter } from '../components/Recenter'
import { useMapSocket } from '../hooks/useMapSocket'
import { UserAvatar } from '../components/UserAvatar'
import { MapUserPeek } from '../components/MapUserPeek'
import { userShortform } from '../lib/avatar'

function markerIcon(u: NearbyUser, loggedIn: boolean) {
  const status = u.online ? 'online' : 'offline'
  const me = u.is_me ? ' me' : ''
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
  const html = `<div class="disc-marker ${status}${me}"><span class="disc-initials">${short}</span>${img}<span class="dot"></span></div>`
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

export function MapPage() {
  const { user } = useAuth()
  const loggedIn = !!user
  const [navOpen, setNavOpen] = useState(false)
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
  const [center, setCenter] = useState<[number, number]>([23.8103, 90.4125])
  const [zoom, setZoom] = useState(12)
  const [viewTick, setViewTick] = useState(0)
  const [mapReady, setMapReady] = useState(false)
  const [socketOk, setSocketOk] = useState(false)
  const [liveOn, setLiveOn] = useState(false)
  const [liveBusy, setLiveBusy] = useState(false)
  const watchIdRef = useRef<number | null>(null)
  const lastLiveSentRef = useRef<{ t: number; lat: number; lng: number } | null>(null)
  const liveWatchActiveRef = useRef(false)
  const liveUpdateRef = useRef<(lat: number, lng: number) => boolean>(() => false)

  const stopLiveWatch = () => {
    liveWatchActiveRef.current = false
    lastLiveSentRef.current = null
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

  useEffect(() => {
    if (!user) return
    api
      .get<{ success: boolean; data: { lat?: number; lng?: number } }>('/api/v1/me/profile')
      .then((r) => {
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
        void api
          .post('/api/v1/me/location', {
            lat: pos.coords.latitude,
            lng: pos.coords.longitude,
          })
          .then(() => {
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

  // HTTP fallback when WebSocket is down (or slow to connect).
  useEffect(() => {
    if (!mapReady || socketOk) return
    const load = () => {
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
    }
    load()
    const id = window.setInterval(load, 8000)
    return () => window.clearInterval(id)
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

  const markers = useMemo(
    () =>
      users.map((u) => {
        const isOpen = selected?.username === u.username
        return (
          <Marker
            key={`${u.username}-${u.lat}-${u.lng}`}
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
    [users, loggedIn, selected?.username],
  )

  return (
    <div className="relative h-full w-full overflow-hidden">
      <SideNav open={navOpen} onClose={() => setNavOpen(false)} />

      <button
        type="button"
        className="absolute left-3 top-3 z-[1000] flex h-10 w-10 items-center justify-center rounded-full bg-[#0b1224] text-white shadow-lg ring-1 ring-white/15"
        onClick={() => setNavOpen(true)}
        title="Menu"
      >
        <Menu size={18} />
      </button>

      <div className="absolute left-1/2 top-3 z-[1000] flex -translate-x-1/2 items-center gap-2 rounded-full border border-white/10 bg-[#0b1224]/95 px-3 py-1.5 text-xs shadow-lg backdrop-blur">
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
          onClick={() => setFilterOpen((v) => !v)}
          className={`ml-1 inline-flex items-center gap-1 rounded-full px-2.5 py-1 font-semibold ${
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
            const me = users.find((u) => u.is_me)
            if (me) {
              setCenter([me.lat, me.lng])
              setZoom(16)
              setViewTick((t) => t + 1)
            }
          }}
        />
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
