import { Link, NavLink } from 'react-router-dom'
import {
  Bell,
  Calendar,
  Handshake,
  LogOut,
  Map,
  MessageCircle,
  X,
} from 'lucide-react'
import { useAuth } from '../auth/AuthContext'
import { UserAvatar } from './UserAvatar'

type Props = {
  open: boolean
  onClose: () => void
}

const linkClass = ({ isActive }: { isActive: boolean }) =>
  `flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium transition ${
    isActive ? 'bg-white/10 text-white' : 'text-slate-300 hover:bg-white/5 hover:text-white'
  }`

export function SideNav({ open, onClose }: Props) {
  const { user, logout } = useAuth()

  return (
    <>
      {/* Mobile backdrop */}
      <div
        className={`fixed inset-0 z-[1100] bg-black/50 transition lg:hidden ${
          open ? 'opacity-100' : 'pointer-events-none opacity-0'
        }`}
        onClick={onClose}
        aria-hidden={!open}
      />
      <aside
        className={`fixed left-0 top-0 z-[1101] flex h-full w-72 flex-col border-r border-white/10 bg-[#0b1224] transition-transform lg:translate-x-0 ${
          open ? 'translate-x-0' : '-translate-x-full'
        }`}
      >
        <div className="flex items-center justify-between border-b border-white/10 px-4 py-4">
          <Link to="/" className="flex items-center gap-2" onClick={onClose}>
            <span className="flex h-9 w-9 items-center justify-center rounded-xl bg-yellow-400 text-sm font-extrabold text-slate-900">
              FN
            </span>
            <div>
              <div className="text-sm font-bold">FindNeighbour</div>
              <div className="text-[11px] text-slate-400">Global Social Connections</div>
            </div>
          </Link>
          <button
            type="button"
            onClick={onClose}
            className="rounded-full bg-white/10 p-1.5 text-white hover:bg-white/20 lg:hidden"
            aria-label="Close menu"
          >
            <X size={16} />
          </button>
        </div>

        <nav className="flex flex-1 flex-col gap-1 overflow-y-auto p-3">
          <NavLink to="/" end className={linkClass} onClick={onClose}>
            <Map size={18} /> Discover map
          </NavLink>
          {user ? (
            <>
              <NavLink to="/events" className={linkClass} onClick={onClose}>
                <Calendar size={18} /> Events
              </NavLink>
              <NavLink to="/messages" className={linkClass} onClick={onClose}>
                <MessageCircle size={18} /> Messages
              </NavLink>
              <NavLink to="/connections" className={linkClass} onClick={onClose}>
                <Handshake size={18} /> Connections
              </NavLink>
              <NavLink to="/notifications" className={linkClass} onClick={onClose}>
                <Bell size={18} /> Notifications
              </NavLink>
              <NavLink to="/profile" className={linkClass} onClick={onClose}>
                <UserAvatar
                  name={user.username}
                  size={22}
                  fontSize={9}
                  online
                  showDot={false}
                  className="!border-2"
                />
                @{user.username}
              </NavLink>
              <button
                type="button"
                className="mt-2 flex items-center gap-3 rounded-xl px-3 py-2.5 text-left text-sm font-medium text-slate-300 hover:bg-white/5 hover:text-white"
                onClick={async () => {
                  await logout()
                  onClose()
                }}
              >
                <LogOut size={18} /> Logout
              </button>
            </>
          ) : (
            <>
              <NavLink to="/login" className={linkClass} onClick={onClose}>
                Login
              </NavLink>
              <NavLink to="/register" className={linkClass} onClick={onClose}>
                Register
              </NavLink>
            </>
          )}
        </nav>
      </aside>
    </>
  )
}
