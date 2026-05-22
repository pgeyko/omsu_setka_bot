import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Shield, Plus, Trash2, Loader2, RefreshCw, UserCheck } from 'lucide-react'
import { api } from '../api/client'
import { toast } from '../components/Toast'

interface Superadmin {
  user_id: number
  note: string
  created_at: string
}

interface RegisterResult {
  status: string
  registered_groups?: number[]
}

export default function SuperadminsPage() {
  const qc = useQueryClient()
  const [newUserId, setNewUserId] = useState('')
  const [newNote, setNewNote] = useState('')

  const { data: admins, isLoading } = useQuery<Superadmin[]>({
    queryKey: ['superadmins'],
    queryFn: () => api.get<Superadmin[]>('/api/admin/superadmins'),
  })

  const addMut = useMutation({
    mutationFn: ({ user_id, note }: { user_id: number; note: string }) =>
      api.post('/api/admin/superadmins', { user_id, note }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['superadmins'] })
      setNewUserId('')
      setNewNote('')
      toast('Суперадмин добавлен', 'success')
    },
    onError: (err: any) => toast(`Ошибка: ${err.message}`, 'error'),
  })

  const removeMut = useMutation({
    mutationFn: (user_id: number) => api.delete(`/api/admin/superadmins/${user_id}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['superadmins'] })
      toast('Суперадмин удалён', 'success')
    },
    onError: (err: any) => toast(`Ошибка: ${err.message}`, 'error'),
  })

  const registerMut = useMutation({
    mutationFn: () => api.post<RegisterResult>('/api/admin/groups/register-webhooks'),
    onSuccess: (data) => {
      const msg =
        data.status === 'no_active_groups_to_register'
          ? 'Нет активных групп для регистрации'
          : `Вебхуки зарегистрированы для ${data.registered_groups?.length ?? 0} групп`
      toast(msg, 'success')
    },
    onError: (err: any) => toast(`Ошибка: ${err.message}`, 'error'),
  })

  const handleAdd = () => {
    const uid = parseInt(newUserId, 10)
    if (!uid || isNaN(uid)) {
      toast('Введите корректный Telegram User ID', 'error')
      return
    }
    addMut.mutate({ user_id: uid, note: newNote })
  }

  return (
    <div>
      <h2 style={{ marginBottom: '1.5rem', fontSize: '1.25rem' }}>Суперадмины и вебхуки</h2>

      <div style={{ display: 'flex', flexDirection: 'column', gap: '1.5rem', maxWidth: 680 }}>

        {/* Register webhooks */}
        <div className="card">
          <h3 style={{ fontSize: '1rem', marginBottom: '0.75rem', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
            <RefreshCw size={18} color="var(--accent)" /> Регистрация вебхуков в Setka
          </h3>
          <p style={{ fontSize: '0.85rem', color: 'var(--text-muted)', marginBottom: '0.75rem' }}>
            Регистрирует webhook-URL всех активных групп в omsu_mirror (Setka).
            Запускайте после добавления новых групп или смены адреса бота.
          </p>
          <button
            className="btn btn-primary btn-sm"
            onClick={() => registerMut.mutate()}
            disabled={registerMut.isPending}
          >
            {registerMut.isPending ? <Loader2 size={16} className="spinner" /> : <RefreshCw size={16} />}
            {' '}{registerMut.isPending ? 'Регистрация...' : 'Зарегистрировать вебхуки'}
          </button>
          {registerMut.data && (
            <div style={{ marginTop: '0.75rem', fontSize: '0.85rem', color: 'var(--success)' }}>
              ✓ {registerMut.data.status === 'no_active_groups_to_register'
                ? 'Нет активных групп'
                : `Зарегистрировано групп: ${registerMut.data.registered_groups?.join(', ')}`}
            </div>
          )}
        </div>

        {/* Superadmin management */}
        <div className="card">
          <h3 style={{ fontSize: '1rem', marginBottom: '0.75rem', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
            <Shield size={18} color="var(--accent)" /> Суперадмины бота
          </h3>
          <p style={{ fontSize: '0.85rem', color: 'var(--text-muted)', marginBottom: '1rem' }}>
            Суперадмины могут управлять настройками бота через Telegram-команды в любой группе.
          </p>

          <div style={{ display: 'flex', gap: '0.5rem', marginBottom: '1rem', flexWrap: 'wrap' }}>
            <input
              id="superadmin-uid"
              className="input"
              type="number"
              placeholder="Telegram User ID"
              value={newUserId}
              onChange={(e) => setNewUserId(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleAdd()}
              style={{ flex: '1 1 160px' }}
            />
            <input
              id="superadmin-note"
              className="input"
              placeholder="Примечание (опционально)"
              value={newNote}
              onChange={(e) => setNewNote(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleAdd()}
              style={{ flex: '2 1 220px' }}
            />
            <button
              id="superadmin-add-btn"
              className="btn btn-primary btn-sm"
              onClick={handleAdd}
              disabled={addMut.isPending || !newUserId}
            >
              {addMut.isPending ? <Loader2 size={16} className="spinner" /> : <Plus size={16} />}
              {' '}Добавить
            </button>
          </div>

          {isLoading ? (
            <div style={{ fontSize: '0.85rem', color: 'var(--text-muted)' }}>Загрузка...</div>
          ) : admins && admins.length > 0 ? (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
              {admins.map((a) => (
                <div key={a.user_id} style={{
                  display: 'flex', alignItems: 'center', gap: '0.75rem',
                  padding: '0.6rem 0.75rem',
                  borderRadius: 'var(--radius-sm)',
                  background: 'var(--glass-bg)',
                  border: '1px solid var(--glass-border)',
                }}>
                  <UserCheck size={16} color="var(--accent)" />
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{ fontSize: '0.9rem', fontWeight: 500 }}>ID: {a.user_id}</div>
                    {a.note && (
                      <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        {a.note}
                      </div>
                    )}
                    <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                      Добавлен: {a.created_at}
                    </div>
                  </div>
                  <button
                    className="btn btn-sm"
                    style={{ color: 'var(--danger)', flexShrink: 0 }}
                    onClick={() => {
                      if (confirm(`Удалить суперадмина ${a.user_id}?`)) removeMut.mutate(a.user_id)
                    }}
                    disabled={removeMut.isPending}
                    title="Удалить суперадмина"
                  >
                    <Trash2 size={14} />
                  </button>
                </div>
              ))}
            </div>
          ) : (
            <div style={{ fontSize: '0.85rem', color: 'var(--text-muted)', textAlign: 'center', padding: '1.5rem 0' }}>
              Суперадмины не добавлены
            </div>
          )}
        </div>

      </div>
    </div>
  )
}
