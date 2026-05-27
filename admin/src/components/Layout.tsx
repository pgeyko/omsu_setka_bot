import { useState, useMemo } from 'react'
import { Outlet, NavLink, useNavigate, useLocation } from 'react-router-dom'
import { LogOut, Menu, Sun, Moon, X } from 'lucide-react'
import { useAuthStore } from '../stores/authStore'
import { navItems, titleMap } from '../shared/constants/navigation'
import { useTheme } from '../shared/hooks/useTheme'
import ErrorBoundary from './ErrorBoundary'
import Toast from './Toast'

function SidebarContent({ onNavClick }: { onNavClick?: () => void }) {
  const { dark, toggleTheme } = useTheme()
  const clearToken = useAuthStore((s) => s.clearToken)
  const navigate = useNavigate()

  const logout = async () => {
    const token = useAuthStore.getState().token
    if (token) {
      try {
        await fetch('/api/auth/logout', {
          method: 'POST',
          headers: { Authorization: `Bearer ${token}` },
        })
      } catch { /* ignore */ }
    }
    clearToken()
    navigate('/login')
  }

  return (
    <>
      <div style={{ padding: '1rem', borderBottom: '1px solid var(--glass-border)', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
        <span style={{ fontWeight: 600, fontSize: '1.1rem' }}>GroupBot</span>
      </div>
      <nav style={{ flex: 1, padding: '0.5rem', display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
        {navItems.map(({ to, icon: Icon, label }) => (
          <NavLink key={to} to={to} end={to === '/'} onClick={onNavClick}
            style={({ isActive }) => ({
              display: 'flex', alignItems: 'center', gap: '0.6rem',
              padding: '0.6rem 0.75rem', borderRadius: 'var(--radius-sm)',
              fontSize: '0.9rem', color: isActive ? 'var(--accent)' : 'var(--text)',
              background: isActive ? 'var(--accent-glass)' : 'transparent',
              transition: 'var(--transition)',
            })}>
            <Icon size={18} /> {label}
          </NavLink>
        ))}
      </nav>
      <div style={{ padding: '0.5rem', borderTop: '1px solid var(--glass-border)', display: 'flex', gap: '0.25rem' }}>
        <button className="btn btn-sm" onClick={toggleTheme} title="Сменить тему">
          {dark ? <Sun size={16} /> : <Moon size={16} />}
        </button>
        <button className="btn btn-sm" onClick={logout} title="Выйти" style={{ marginLeft: 'auto' }}>
          <LogOut size={16} /> Выйти
        </button>
      </div>
    </>
  )
}

export default function Layout() {
  const [sidebarOpen, setSidebarOpen] = useState(false)
  const location = useLocation()
  const currentTitle = useMemo(() => titleMap[location.pathname] || 'GroupBot', [location.pathname])

  return (
    <div className="app-layout">
      <aside className="sidebar-desktop">
        <SidebarContent />
      </aside>

      <aside className={`sidebar-mobile ${sidebarOpen ? 'open' : ''}`}>
        <div style={{ padding: '0.5rem', display: 'flex', justifyContent: 'flex-end' }}>
          <button className="btn btn-sm" onClick={() => setSidebarOpen(false)}>
            <X size={18} />
          </button>
        </div>
        <SidebarContent onNavClick={() => setSidebarOpen(false)} />
      </aside>

      {sidebarOpen && <div className="mobile-overlay" onClick={() => setSidebarOpen(false)} />}

      <div className="content-area">
        <header style={{
          height: 'var(--header-h)', display: 'flex', alignItems: 'center', gap: '0.5rem',
          padding: '0 1.5rem', borderBottom: '1px solid var(--glass-border)',
          background: 'var(--bg-card)',
          position: 'sticky', top: 0, zIndex: 50,
        }}>
          <button className="btn btn-sm mobile-menu-toggle" onClick={() => setSidebarOpen(!sidebarOpen)}>
            <Menu size={18} />
          </button>
          <span style={{ fontWeight: 500, fontSize: '0.95rem' }}>{currentTitle}</span>
        </header>
        <main style={{ flex: 1, padding: '1.5rem', maxWidth: 'var(--max-content)', width: '100%', margin: '0 auto' }}>
          <ErrorBoundary><Outlet /></ErrorBoundary>
        </main>
      </div>

      <Toast />
    </div>
  )
}
