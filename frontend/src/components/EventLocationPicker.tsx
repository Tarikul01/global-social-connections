import { useEffect, useState } from 'react'
import { MapContainer, Marker, TileLayer, useMap, useMapEvents } from 'react-leaflet'
import L from 'leaflet'

const pinIcon = L.divIcon({
  className: 'leaflet-div-icon',
  html: `<div style="width:18px;height:18px;border-radius:50%;background:#facc15;border:3px solid #0b1224;box-shadow:0 2px 8px rgba(0,0,0,.45)"></div>`,
  iconSize: [18, 18],
  iconAnchor: [9, 9],
})

function ClickPick({
  lat,
  lng,
  onPick,
}: {
  lat: number
  lng: number
  onPick: (lat: number, lng: number) => void
}) {
  useMapEvents({
    click(e) {
      onPick(e.latlng.lat, e.latlng.lng)
    },
  })
  return <Marker position={[lat, lng]} icon={pinIcon} draggable eventHandlers={{
    dragend(e) {
      const m = e.target as L.Marker
      const p = m.getLatLng()
      onPick(p.lat, p.lng)
    },
  }} />
}

function FlyTo({ center, tick }: { center: [number, number]; tick: number }) {
  const map = useMap()
  useEffect(() => {
    if (tick <= 0) return
    map.flyTo(center, Math.max(map.getZoom(), 14), { duration: 0.6 })
  }, [map, center, tick])
  return null
}

export function EventLocationPicker({
  lat,
  lng,
  onChange,
  onUseMyLocation,
  heightClass = 'h-56',
}: {
  lat: number
  lng: number
  onChange: (lat: number, lng: number) => void
  onUseMyLocation?: () => void
  heightClass?: string
}) {
  const [mapKey, setMapKey] = useState(0)
  const [flyTick, setFlyTick] = useState(0)

  useEffect(() => {
    setMapKey((k) => k + 1)
  }, [])

  const pick = (a: number, b: number) => {
    onChange(Number(a.toFixed(6)), Number(b.toFixed(6)))
  }

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between gap-3">
        <div>
          <div className="text-sm text-slate-300">Meetup spot on map</div>
          <p className="text-xs text-slate-500">Click or drag the pin to set the location.</p>
        </div>
        {onUseMyLocation && (
          <button type="button" onClick={() => {
            onUseMyLocation()
            setFlyTick((t) => t + 1)
          }} className="shrink-0 text-sm text-blue-300 underline">
            Use my location
          </button>
        )}
      </div>
      <div className={`${heightClass} overflow-hidden rounded-xl border border-white/10`}>
        <MapContainer
          key={mapKey}
          center={[lat, lng]}
          zoom={14}
          className="h-full w-full"
          scrollWheelZoom
        >
          <TileLayer
            attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OSM</a>'
            url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
          />
          <FlyTo center={[lat, lng]} tick={flyTick} />
          <ClickPick lat={lat} lng={lng} onPick={pick} />
        </MapContainer>
      </div>
      <p className="text-xs text-slate-500">
        Pin: {lat.toFixed(5)}, {lng.toFixed(5)}
      </p>
    </div>
  )
}
