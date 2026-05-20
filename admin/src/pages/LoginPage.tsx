import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Bot, Eye, EyeOff } from 'lucide-react'
import { api, ApiError } from '../api/client'
import { useAuthStore } from '../stores/authStore'
import { toast } from '../components/Toast'

export default function LoginPage() {
  const [secret, setSecret] = useState('')
  const [show, setShow] = useState(false)
  const [loading, setLoading] = useState(false)
  const setToken = useAuthStore((s) => s.setToken)
  const navigate = useNavigate()

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!secret) return
    setLoading(true)
    try {
      const res = await api.post<{ token: string }>('/api/auth/token', { admin_secret: secret })
      setToken(res.token)
      navigate('/', { replace: true })
    } catch (err) {
      toast(err instanceof ApiError ? err.message : 'Ошибка входа')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', padding: '1rem', background: 'var(--bg)' }}>
      <form onSubmit={handleSubmit} className="card" style={{ width: '100%', maxWidth: 380 }}>
        <div style={{ textAlign: 'center', marginBottom: '1.5rem' }}>
          <Bot size={40} color="var(--accent)" style={{ marginBottom: '0.5rem' }} />
          <h1 style={{ fontSize: '1.25rem', fontWeight: 600 }}>GroupBot Admin</h1>
          <p style={{ fontSize: '0.85rem', color: 'var(--text-muted)', marginTop: '0.25rem' }}>Введите admin_secret для входа</p>
        </div>
        <div style={{ position: 'relative' }}>
          <input className="input" type={show ? 'text' : 'password'} value={secret}
            onChange={(e) => setSecret(e.target.value)} placeholder="admin_secret" autoFocus
            style={{ paddingRight: '2.5rem' }} />
          <button type="button" onClick={() => setShow(!show)}
            style={{ position: 'absolute', right: 8, top: 6, background: 'none', border: 'none', color: 'var(--text-muted)', cursor: 'pointer' }}>
            {show ? <EyeOff size={18} /> : <Eye size={18} />}
          </button>
        </div>
        <button type="submit" className="btn btn-primary" disabled={loading || !secret}
          style={{ width: '100%', marginTop: '1rem', justifyContent: 'center' }}>
          {loading ? <div className="spinner" /> : 'Войти'}
        </button>
      </form>
    </div>
  )
}
