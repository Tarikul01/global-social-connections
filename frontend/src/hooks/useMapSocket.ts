import { useEffect, useRef } from 'react'
import type { NearbyUser } from '../lib/api'

export type MapSubscribe = {
  lat: number
  lng: number
  radius_km: number
  status?: string
  q?: string
  professions?: string[]
}

type NearbyPayload = {
  users: NearbyUser[]
  total: number
  online: number
  live?: boolean
}

type Options = {
  enabled: boolean
  /** Reconnect socket when auth identity changes (login/logout). */
  authKey?: string
  subscribe: MapSubscribe | null
  onNearby: (data: NearbyPayload) => void
  onStatus?: (connected: boolean) => void
}

function wsURL() {
  const base = import.meta.env.VITE_API_URL as string | undefined
  if (base) {
    const u = new URL(base)
    u.protocol = u.protocol === 'https:' ? 'wss:' : 'ws:'
    u.pathname = '/api/v1/ws'
    u.search = ''
    return u.toString()
  }
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${proto}//${window.location.host}/api/v1/ws`
}

/**
 * Map realtime over WebSocket. Replaces HTTP long-polling.
 * Server pushes `nearby` snapshots on subscribe and when the map changes
 * (live location / login / logout).
 */
export function useMapSocket({ enabled, authKey = '', subscribe, onNearby, onStatus }: Options) {
  const onNearbyRef = useRef(onNearby)
  const onStatusRef = useRef(onStatus)
  onNearbyRef.current = onNearby
  onStatusRef.current = onStatus

  const subKey = subscribe
    ? JSON.stringify({
        lat: round(subscribe.lat),
        lng: round(subscribe.lng),
        radius_km: subscribe.radius_km,
        status: subscribe.status || '',
        q: subscribe.q || '',
        professions: subscribe.professions || [],
      })
    : ''

  useEffect(() => {
    if (!enabled || !subscribe) return

    let closed = false
    let ws: WebSocket | null = null
    let retryTimer: number | undefined
    let pingTimer: number | undefined
    let attempt = 0

    const sendSubscribe = (socket: WebSocket) => {
      if (socket.readyState !== WebSocket.OPEN) return
      socket.send(
        JSON.stringify({
          type: 'subscribe',
          data: {
            lat: subscribe.lat,
            lng: subscribe.lng,
            radius_km: subscribe.radius_km,
            status: subscribe.status || '',
            q: subscribe.q || '',
            professions: subscribe.professions || [],
          },
        }),
      )
    }

    const connect = () => {
      if (closed) return
      ws = new WebSocket(wsURL())

      ws.onopen = () => {
        attempt = 0
        onStatusRef.current?.(true)
        sendSubscribe(ws!)
        pingTimer = window.setInterval(() => {
          if (ws?.readyState === WebSocket.OPEN) {
            ws.send(JSON.stringify({ type: 'ping' }))
          }
        }, 25000)
      }

      ws.onmessage = (ev) => {
        try {
          const msg = JSON.parse(ev.data as string) as {
            type: string
            data?: NearbyPayload
          }
          if (msg.type === 'nearby' && msg.data) {
            onNearbyRef.current(msg.data)
          }
        } catch {
          /* ignore malformed */
        }
      }

      ws.onclose = () => {
        onStatusRef.current?.(false)
        if (pingTimer) window.clearInterval(pingTimer)
        if (closed) return
        const delay = Math.min(10000, 1000 * 2 ** attempt)
        attempt += 1
        retryTimer = window.setTimeout(connect, delay)
      }

      ws.onerror = () => {
        ws?.close()
      }
    }

    connect()

    return () => {
      closed = true
      onStatusRef.current?.(false)
      if (retryTimer) window.clearTimeout(retryTimer)
      if (pingTimer) window.clearInterval(pingTimer)
      ws?.close()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- subKey + authKey drive reconnect
  }, [enabled, subKey, authKey])
}

function round(n: number) {
  return Math.round(n * 1e5) / 1e5
}
