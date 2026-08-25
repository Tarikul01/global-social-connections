import { useEffect, useRef } from 'react'
import { useMap } from 'react-leaflet'

/** Flies the map only when `tick` increases (explicit navigate), not on every pan. */
export function Recenter({
  center,
  zoom,
  tick,
}: {
  center: [number, number]
  zoom: number
  tick: number
}) {
  const map = useMap()
  const last = useRef(0)
  useEffect(() => {
    if (tick === last.current) return
    last.current = tick
    map.setView(center, zoom)
  }, [map, center, zoom, tick])
  return null
}
