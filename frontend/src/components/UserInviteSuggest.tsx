import { useEffect, useRef, useState } from 'react'
import { api } from '../lib/api'

export type SuggestUser = {
  id: string
  username: string
  email: string
  full_name: string
}

type Props = {
  /** API path that accepts ?q= and returns { success, data: SuggestUser[] } */
  searchPath: string
  selected: SuggestUser[]
  onChange: (users: SuggestUser[]) => void
  placeholder?: string
  /** Single-select mode: pick one and clear (for host invite form) */
  single?: boolean
  onPickSingle?: (user: SuggestUser) => void
  disabled?: boolean
}

export function UserInviteSuggest({
  searchPath,
  selected,
  onChange,
  placeholder = 'Type a name, username, or email',
  single,
  onPickSingle,
  disabled,
}: Props) {
  const [q, setQ] = useState('')
  const [results, setResults] = useState<SuggestUser[]>([])
  const [open, setOpen] = useState(false)
  const [loading, setLoading] = useState(false)
  const wrapRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const onDoc = (e: MouseEvent) => {
      if (!wrapRef.current?.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', onDoc)
    return () => document.removeEventListener('mousedown', onDoc)
  }, [])

  useEffect(() => {
    const term = q.trim()
    if (term.length < 1) {
      setResults([])
      return
    }
    let cancelled = false
    const t = window.setTimeout(() => {
      setLoading(true)
      void api
        .get<{ success: boolean; data: SuggestUser[] }>(
          `${searchPath}${searchPath.includes('?') ? '&' : '?'}q=${encodeURIComponent(term)}`,
        )
        .then((r) => {
          if (cancelled) return
          const selectedIds = new Set(selected.map((u) => u.id))
          setResults((r.data || []).filter((u) => !selectedIds.has(u.id)))
          setOpen(true)
        })
        .catch(() => {
          if (!cancelled) setResults([])
        })
        .finally(() => {
          if (!cancelled) setLoading(false)
        })
    }, 220)
    return () => {
      cancelled = true
      window.clearTimeout(t)
    }
  }, [q, searchPath, selected])

  const add = (u: SuggestUser) => {
    if (single) {
      onPickSingle?.(u)
      setQ('')
      setResults([])
      setOpen(false)
      return
    }
    if (selected.some((s) => s.id === u.id)) return
    onChange([...selected, u])
    setQ('')
    setResults([])
    setOpen(false)
  }

  const remove = (id: string) => onChange(selected.filter((u) => u.id !== id))

  return (
    <div ref={wrapRef} className="relative space-y-2">
      {!single && selected.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {selected.map((u) => (
            <button
              key={u.id}
              type="button"
              onClick={() => remove(u.id)}
              className="inline-flex items-center gap-1 rounded-lg border border-white/15 bg-[#111827] px-2 py-1 text-xs text-slate-200"
              title="Remove"
            >
              <span>{u.full_name || u.username}</span>
              <span className="text-slate-500">@{u.username}</span>
              <span className="text-slate-500">×</span>
            </button>
          ))}
        </div>
      )}
      <input
        value={q}
        disabled={disabled}
        onChange={(e) => {
          setQ(e.target.value)
          setOpen(true)
        }}
        onFocus={() => q.trim() && setOpen(true)}
        placeholder={placeholder}
        className="w-full rounded-xl border border-white/10 bg-[#111827] px-3 py-2.5 text-sm outline-none focus:border-yellow-400 disabled:opacity-60"
        autoComplete="off"
      />
      {open && (results.length > 0 || loading || q.trim().length > 0) && (
        <ul className="absolute z-20 mt-1 max-h-48 w-full overflow-auto rounded-xl border border-white/10 bg-[#0b1224] shadow-xl">
          {loading && (
            <li className="px-3 py-2 text-xs text-slate-500">Searching…</li>
          )}
          {!loading && results.length === 0 && q.trim() && (
            <li className="px-3 py-2 text-xs text-slate-500">No matching users</li>
          )}
          {results.map((u) => (
            <li key={u.id}>
              <button
                type="button"
                className="flex w-full flex-col items-start gap-0.5 px-3 py-2 text-left hover:bg-white/5"
                onClick={() => add(u)}
              >
                <span className="text-sm text-white">
                  {u.full_name || u.username}{' '}
                  <span className="text-slate-500">@{u.username}</span>
                </span>
                {u.email && <span className="text-xs text-slate-500">{u.email}</span>}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
