import { Link } from 'react-router-dom'

export function PlaceholderPage({ title }: { title: string }) {
  return (
    <div className="flex min-h-full flex-col items-center justify-center gap-3 bg-[#050816] p-6">
      <h1 className="text-2xl font-bold">{title}</h1>
      <p className="text-sm text-slate-400">Connected to the Go API — UI coming next.</p>
      <Link to="/" className="text-blue-300 underline">
        Back to map
      </Link>
    </div>
  )
}
