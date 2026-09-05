import { MapContainer, Marker, TileLayer } from 'react-leaflet'
import { Link } from 'react-router-dom'
import { eventMapDivIcon } from '../lib/eventMapIcons'

type Props = {
  lat: number
  lng: number
  label?: string
  category?: string | null
  discoverHref?: string | null
  heightClass?: string
}

/** Read-only map preview of an event meetup pin. */
export function EventMapPreview({
  lat,
  lng,
  label,
  category,
  discoverHref,
  heightClass = 'h-48',
}: Props) {
  const icon = eventMapDivIcon(category, { focused: true, label })

  return (
    <div className="space-y-2">
      <div className={`${heightClass} overflow-hidden rounded-xl border border-white/10`}>
        <MapContainer
          center={[lat, lng]}
          zoom={15}
          className="h-full w-full"
          scrollWheelZoom={false}
          dragging
          zoomControl={false}
        >
          <TileLayer
            attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OSM</a>'
            url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
          />
          <Marker position={[lat, lng]} icon={icon} />
        </MapContainer>
      </div>
      <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-slate-500">
        <span>
          {label ? `${label} · ` : ''}
          {lat.toFixed(5)}, {lng.toFixed(5)}
        </span>
        {discoverHref && (
          <Link to={discoverHref} className="text-blue-300 underline">
            Open on discover map
          </Link>
        )}
      </div>
    </div>
  )
}
