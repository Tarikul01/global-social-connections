import { useEffect, useState } from 'react'
import { Outlet, useLocation } from 'react-router-dom'
import { Menu } from 'lucide-react'
import { SideNav } from './SideNav'

/** Shared chrome: menu on every screen (drawer on mobile, rail on desktop). */
export function AppShell() {
  const [navOpen, setNavOpen] = useState(false)
  const location = useLocation()
  const isMap = location.pathname === '/'

  // Close mobile drawer on navigate.
  useEffect(() => {
    setNavOpen(false)
  }, [location.pathname])

  return (
    <div className="relative flex h-full w-full overflow-hidden bg-[#050816]">
      <SideNav open={navOpen} onClose={() => setNavOpen(false)} />

      <button
        type="button"
        className="fixed left-3 top-3 z-[1000] flex h-10 w-10 items-center justify-center rounded-full bg-[#0b1224] text-white shadow-lg ring-1 ring-white/15 lg:hidden"
        onClick={() => setNavOpen(true)}
        title="Menu"
        aria-label="Open menu"
      >
        <Menu size={18} />
      </button>

      <div
        className={`min-h-0 min-w-0 flex-1 overflow-hidden lg:pl-72 ${
          isMap ? '' : 'overflow-y-auto pt-14 lg:pt-0'
        }`}
      >
        <Outlet />
      </div>
    </div>
  )
}
