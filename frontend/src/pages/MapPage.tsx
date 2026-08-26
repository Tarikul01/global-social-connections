import { useEffect, useMemo, useRef, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { MapContainer, TileLayer, Marker, useMap, useMapEvents } from 'react-leaflet'
import L from 'leaflet'
import { Filter, LocateFixed, MapPin, Menu, Minus, Plus, Radio, Users, X } from 'lucide-react'
import { useAuth } from '../auth/AuthContext'
import { api, type NearbyUser, type Profession } from '../lib/api'
import { SideNav } from '../components/SideNav'
import { Recenter } from '../components/Recenter'
import { useMapSocket } from '../hooks/useMapSocket'

function markerIcon(u: NearbyUser, loggedIn: boolean) {
  const status = u.online ? 'online' : 'offline'
  const me = u.is_me ? ' me' : ''
  const label = loggedIn && u.full_name ? u.full_name : u.username ? `@${u.username}` : '?'
  const inner = u.photo_url
    ? `<img src="${escapeAttr(u.photo_url)}" alt="">`
    : escapeHtml(u.initials || initialsFrom(label))
  const html = `<div class="disc-marker ${status}${me}">${inner}<span class="dot"></span></div>`
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

function initialsFrom(name: string) {
  const parts = name.replace('@', '').trim().split(/\s+/)
  if (!parts[0]) return '?'
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase()
  return (parts[0][0] + parts[1][0]).toUpperCase()
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
  const navigate = useNavigate()
  const loggedIn = !!user
  const [navOpen, setNavOpen] = useState(false)
  const [panelOpen, setPanelOpen] = useState(false)
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

  useMapSocket({
    enabled: mapReady,
    subscribe,
    onNearby: (data) => {
      setUsers(data.users || [])
      setTotal(data.total || 0)
      setOnline(data.online || 0)
    },
    onStatus: setSocketOk,
  })

  useEffect(() => {
    api
      .get<{ success: boolean; data: Profession[] }>('/api/v1/professions')
      .then((r) => setProfessions(r.data || []))
      .catch(() => {})
    api
      .get<{ success: boolean; data: { total: number; online: number } }>('/api/v1/stats')
      .then((r) => {
        setTotal(r.data.total)
        setOnline(r.data.online)
      })
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
  }, [user])

  const stopLiveWatch = () => {
    if (watchIdRef.current != null && navigator.geolocation) {
      navigator.geolocation.clearWatch(watchIdRef.current)
      watchIdRef.current = null
    }
  }

  const startLiveWatch = () => {
    if (!navigator.geolocation) return
    stopLiveWatch()
    watchIdRef.current = navigator.geolocation.watchPosition(
      (pos) => {
        const lat = pos.coords.latitude
        const lng = pos.coords.longitude
        api
          .post('/api/v1/live-location', { lat, lng })
          .then(() => {
            setCenter([lat, lng])
          })
          .catch(() => {})
      },
      () => {},
      { enableHighAccuracy: true, maximumAge: 3000, timeout: 15000 },
    )
  }

  useEffect(() => {
    if (!user) {
      setLiveOn(false)
      stopLiveWatch()
      return
    }
    api
      .get<{ ok?: boolean; live?: boolean }>('/api/v1/live-location/status')
      .then((r) => {
        const on = !!r.live
        setLiveOn(on)
        if (on) startLiveWatch()
      })
      .catch(() => setLiveOn(false))
    // eslint-disable-next-line react-hooks/exhaustive-deps
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
        await api.delete('/api/v1/live-location')
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
        await api.post('/api/v1/live-location/start', {
          lat: coords.latitude,
          lng: coords.longitude,
        })
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
    if (loggedIn) navigate(`/u/${u.username}`)
    else navigate(`/login?next=${encodeURIComponent(`/u/${u.username}`)}`)
  }

  const markers = useMemo(
    () =>
      users.map((u) => (
        <Marker
          key={`${u.username}-${u.lat}-${u.lng}`}
          position={[u.lat, u.lng]}
          icon={markerIcon(u, loggedIn)}
          eventHandlers={{ click: () => openPerson(u) }}
        />
      )),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [users, loggedIn],
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
                <div
                  className={`disc-marker ${u.online ? 'online' : 'offline'}`}
                  style={{ width: 36, height: 36, fontSize: 11, flexShrink: 0 }}
                >
                  {u.photo_url ? (
                    <img src={u.photo_url} alt="" />
                  ) : (
                    u.initials || '?'
                  )}
                  <span className="dot" />
                </div>
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
