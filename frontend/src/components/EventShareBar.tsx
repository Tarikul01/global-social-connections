import { useState } from 'react'
import { Check, Copy, Share2 } from 'lucide-react'

type Props = {
  shareUrl: string
  title: string
  /** Prefer showing for hosts/creators; can also show for any viewer. */
  compact?: boolean
}

export function EventShareBar({ shareUrl, title, compact }: Props) {
  const [copied, setCopied] = useState(false)
  const [error, setError] = useState('')

  const copy = async () => {
    setError('')
    try {
      await navigator.clipboard.writeText(shareUrl)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 2000)
    } catch {
      setError('Could not copy — select the link manually')
    }
  }

  const nativeShare = async () => {
    setError('')
    if (navigator.share) {
      try {
        await navigator.share({
          title: title || 'FindNeighbour event',
          text: `Join “${title}” on FindNeighbour`,
          url: shareUrl,
        })
        return
      } catch {
        /* user cancelled or unsupported — fall through to copy */
      }
    }
    await copy()
  }

  const encoded = encodeURIComponent(shareUrl)
  const text = encodeURIComponent(`Join “${title}” on FindNeighbour`)
  const twitter = `https://twitter.com/intent/tweet?url=${encoded}&text=${text}`
  const facebook = `https://www.facebook.com/sharer/sharer.php?u=${encoded}`
  const whatsapp = `https://wa.me/?text=${text}%20${encoded}`
  const linkedin = `https://www.linkedin.com/sharing/share-offsite/?url=${encoded}`

  return (
    <div
      className={`rounded-xl border border-white/10 bg-[#111827]/80 ${
        compact ? 'p-3' : 'p-4'
      }`}
    >
      <div className="mb-2 flex items-center gap-2 text-sm font-semibold text-white">
        <Share2 size={16} className="text-yellow-400" />
        Share this event
      </div>
      <p className="mb-3 text-xs text-slate-400">
        Unique link — anyone with it can open this event
        {compact ? '' : ' (public events open for everyone; private need login).'}
      </p>

      <div className="flex gap-2">
        <input
          readOnly
          value={shareUrl}
          className="min-w-0 flex-1 truncate rounded-lg border border-white/10 bg-[#0b1224] px-3 py-2 text-xs text-slate-200 outline-none"
          onFocus={(e) => e.target.select()}
          aria-label="Event share link"
        />
        <button
          type="button"
          onClick={() => void copy()}
          className="inline-flex shrink-0 items-center gap-1.5 rounded-lg border border-white/15 px-3 py-2 text-xs font-semibold text-white hover:bg-white/5"
        >
          {copied ? <Check size={14} className="text-emerald-400" /> : <Copy size={14} />}
          {copied ? 'Copied' : 'Copy'}
        </button>
      </div>

      <div className="mt-3 flex flex-wrap gap-2">
        <button
          type="button"
          onClick={() => void nativeShare()}
          className="rounded-lg bg-yellow-400 px-3 py-1.5 text-xs font-bold text-slate-900 hover:bg-yellow-300"
        >
          Share…
        </button>
        <a
          href={whatsapp}
          target="_blank"
          rel="noreferrer"
          className="rounded-lg border border-white/15 px-3 py-1.5 text-xs font-medium text-slate-200 hover:bg-white/5"
        >
          WhatsApp
        </a>
        <a
          href={twitter}
          target="_blank"
          rel="noreferrer"
          className="rounded-lg border border-white/15 px-3 py-1.5 text-xs font-medium text-slate-200 hover:bg-white/5"
        >
          X / Twitter
        </a>
        <a
          href={facebook}
          target="_blank"
          rel="noreferrer"
          className="rounded-lg border border-white/15 px-3 py-1.5 text-xs font-medium text-slate-200 hover:bg-white/5"
        >
          Facebook
        </a>
        <a
          href={linkedin}
          target="_blank"
          rel="noreferrer"
          className="rounded-lg border border-white/15 px-3 py-1.5 text-xs font-medium text-slate-200 hover:bg-white/5"
        >
          LinkedIn
        </a>
      </div>
      {error && <p className="mt-2 text-xs text-red-300">{error}</p>}
    </div>
  )
}

/** Build the canonical client-side share URL (matches /e/:id). */
export function eventShareUrl(eventId: string) {
  if (typeof window === 'undefined') return `/e/${eventId}`
  return `${window.location.origin}/e/${eventId}`
}
