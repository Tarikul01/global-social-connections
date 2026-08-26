import { useCallback, useEffect, useRef } from 'react'
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

type LivePayload = {
  ok?: boolean
  live?: boolean
  error?: string
}

type Options = {
  enabled: boolean
  /** Reconnect socket only when auth identity changes (login/logout). */
  authKey?: string
  subscribe: MapSubscribe | null
  onNearby: (data: NearbyPayload) => void
  onLive?: (data: LivePayload) => void
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

function round(n: number) {
  return Math.round(n * 1e5) / 1e5
}

/**
 * One long-lived map WebSocket.
 * Viewport/filter changes send `subscribe` — they do NOT reconnect.
 * Live location uses `live_start` / `live_update` / `live_stop` over the same socket.
 */
export function useMapSocket({
  enabled,
  authKey = '',
  subscribe,
  onNearby,
  onLive,
  onStatus,
}: Options) {
  const onNearbyRef = useRef(onNearby)
  const onLiveRef = useRef(onLive)
  const onStatusRef = useRef(onStatus)
  onNearbyRef.current = onNearby
  onLiveRef.current = onLive
  onStatusRef.current = onStatus

  const wsRef = useRef<WebSocket | null>(null)
  const subscribeRef = useRef(subscribe)
  subscribeRef.current = subscribe

  const sendRaw = useCallback((type: string, data?: unknown) => {
    const ws = wsRef.current
    if (!ws || ws.readyState !== WebSocket.OPEN) return false
    ws.send(JSON.stringify(data === undefined ? { type } : { type, data }))
    return true
  }, [])

  const sendSubscribe = useCallback((sub: MapSubscribe) => {
    sendRaw('subscribe', {
      lat: sub.lat,
      lng: sub.lng,
      radius_km: sub.radius_km,
      status: sub.status || '',
      q: sub.q || '',
      professions: sub.professions || [],
    })
  }, [sendRaw])

  // Connect once per enabled + authKey (login/logout).
  useEffect(() => {
    if (!enabled) return

    let closed = false
    let retryTimer: number | undefined
    let pingTimer: number | undefined
    let attempt = 0

    const connect = () => {
      if (closed) return
      const ws = new WebSocket(wsURL())
      wsRef.current = ws

      ws.onopen = () => {
        attempt = 0
        onStatusRef.current?.(true)
        const sub = subscribeRef.current
        if (sub) sendSubscribe(sub)
        pingTimer = window.setInterval(() => {
          if (ws.readyState === WebSocket.OPEN) {
            ws.send(JSON.stringify({ type: 'ping' }))
          }
        }, 25000)
      }

      ws.onmessage = (ev) => {
        try {
          const msg = JSON.parse(ev.data as string) as {
            type: string
            data?: NearbyPayload & LivePayload
          }
          if (msg.type === 'nearby' && msg.data) {
            onNearbyRef.current(msg.data)
          } else if (msg.type === 'live' && msg.data) {
            onLiveRef.current?.(msg.data)
          }
        } catch {
          /* ignore */
        }
      }

      ws.onclose = () => {
        if (wsRef.current === ws) wsRef.current = null
        onStatusRef.current?.(false)
        if (pingTimer) window.clearInterval(pingTimer)
        if (closed) return
        const delay = Math.min(10000, 1000 * 2 ** attempt)
        attempt += 1
        retryTimer = window.setTimeout(connect, delay)
      }

      ws.onerror = () => {
        ws.close()
      }
    }

    connect()

    return () => {
      closed = true
      onStatusRef.current?.(false)
      if (retryTimer) window.clearTimeout(retryTimer)
      if (pingTimer) window.clearInterval(pingTimer)
      wsRef.current?.close()
      wsRef.current = null
    }
  }, [enabled, authKey, sendSubscribe])

  // Viewport / filter change → subscribe only (no reconnect).
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
    if (!enabled || !subscribeRef.current) return
    sendSubscribe(subscribeRef.current)
  }, [enabled, subKey, sendSubscribe])

  const liveStart = useCallback(
    (lat: number, lng: number) => sendRaw('live_start', { lat, lng }),
    [sendRaw],
  )
  const liveUpdate = useCallback(
    (lat: number, lng: number) => sendRaw('live_update', { lat, lng }),
    [sendRaw],
  )
  const liveStop = useCallback(() => sendRaw('live_stop'), [sendRaw])

  return { liveStart, liveUpdate, liveStop }
}
