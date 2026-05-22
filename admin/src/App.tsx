import { Routes, Route, Navigate } from 'react-router-dom'
import { useAuthStore } from './stores/authStore'
import Layout from './components/Layout'
import LoginPage from './pages/LoginPage'
import PersonaPage from './pages/PersonaPage'
import TopicsPage from './pages/TopicsPage'
import PermissionsPage from './pages/PermissionsPage'
import PromptsPage from './pages/PromptsPage'
import StatsPage from './pages/StatsPage'
import DiagnosticsPage from './pages/DiagnosticsPage'
import SchedulePage from './pages/SchedulePage'
import GroupsPage from './pages/GroupsPage'
import SuperadminsPage from './pages/SuperadminsPage'

function PrivateRoute({ children }: { children: React.ReactNode }) {
  const token = useAuthStore((s) => s.token)
  if (!token) return <Navigate to="/login" replace />
  return <>{children}</>
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/" element={<PrivateRoute><Layout /></PrivateRoute>}>
        <Route index element={<PersonaPage />} />
        <Route path="groups" element={<GroupsPage />} />
        <Route path="superadmins" element={<SuperadminsPage />} />
        <Route path="topics" element={<TopicsPage />} />
        <Route path="permissions" element={<PermissionsPage />} />
        <Route path="prompts" element={<PromptsPage />} />
        <Route path="stats" element={<StatsPage />} />
        <Route path="diagnostics" element={<DiagnosticsPage />} />
        <Route path="schedule" element={<SchedulePage />} />
      </Route>
    </Routes>
  )
}
