import { Routes, Route, Navigate } from 'react-router-dom'
import { useAuthStore } from './stores/authStore'
import Layout from './components/Layout'
import LoginPage from './pages/LoginPage'
import GroupsPage from './pages/GroupsPage'
import TopicsPage from './pages/TopicsPage'
import PermissionsPage from './pages/PermissionsPage'
import PromptsPage from './pages/PromptsPage'
import StatsPage from './pages/StatsPage'
import SettingsPage from './pages/SettingsPage'
import SendMessagePage from './pages/SendMessagePage'
import SchedulePage from './pages/SchedulePage'

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
        <Route index element={<Navigate to="/stats" replace />} />
        <Route path="groups" element={<GroupsPage />} />
        <Route path="topics" element={<TopicsPage />} />
        <Route path="permissions" element={<PermissionsPage />} />
        <Route path="prompts" element={<PromptsPage />} />
        <Route path="stats" element={<StatsPage />} />
        <Route path="settings" element={<SettingsPage />} />
        <Route path="send-message" element={<SendMessagePage />} />
        <Route path="schedule" element={<SchedulePage />} />
      </Route>
    </Routes>
  )
}
