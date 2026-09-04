import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { AuthProvider, useAuth } from './auth/AuthContext'
import { MapPage } from './pages/MapPage'
import { LoginPage } from './pages/LoginPage'
import { RegisterPage } from './pages/RegisterPage'
import { ProfilePage } from './pages/ProfilePage'
import { EventsPage } from './pages/EventsPage'
import { EventCreatePage } from './pages/EventCreatePage'
import { EventDetailPage } from './pages/EventDetailPage'
import { PlaceholderPage } from './pages/PlaceholderPage'

function Protected({ children }: { children: React.ReactNode }) {
  const { user, loading } = useAuth()
  if (loading) {
    return (
      <div className="flex h-full items-center justify-center text-slate-400">Loading…</div>
    )
  }
  if (!user) return <Navigate to="/login" replace />
  return children
}

export default function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <Routes>
          <Route path="/" element={<MapPage />} />
          <Route path="/login" element={<LoginPage />} />
          <Route path="/register" element={<RegisterPage />} />
          <Route path="/u/:username" element={<ProfilePage />} />
          <Route
            path="/profile"
            element={
              <Protected>
                <ProfilePage />
              </Protected>
            }
          />
          <Route
            path="/events"
            element={
              <Protected>
                <EventsPage />
              </Protected>
            }
          />
          <Route
            path="/events/new"
            element={
              <Protected>
                <EventCreatePage />
              </Protected>
            }
          />
          <Route
            path="/events/:id"
            element={
              <Protected>
                <EventDetailPage />
              </Protected>
            }
          />
          <Route
            path="/messages"
            element={
              <Protected>
                <PlaceholderPage title="Messages" />
              </Protected>
            }
          />
          <Route
            path="/connections"
            element={
              <Protected>
                <PlaceholderPage title="Connections" />
              </Protected>
            }
          />
          <Route
            path="/notifications"
            element={
              <Protected>
                <PlaceholderPage title="Notifications" />
              </Protected>
            }
          />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </BrowserRouter>
    </AuthProvider>
  )
}
