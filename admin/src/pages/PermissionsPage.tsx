import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Info } from 'lucide-react'
import { api } from '../api/client'
import { toast } from '../components/Toast'
import LoadingSpinner from '../components/LoadingSpinner'
import Select from '../components/Select'

interface Permission { command: string; allowed_role: string }

const commandNames: Record<string, string> = {
  forward: 'Пересылка сообщений',
  summary: 'Саммари (что пропустил?)',
  topic_crud: 'Управление топиками',
  announcement: 'Объявления расписания',
}

export default function PermissionsPage() {
  const qc = useQueryClient()

  const q = useQuery({
    queryKey: ['permissions'],
    queryFn: () => api.get<Permission[]>('/api/permissions'),
  })

  const mut = useMutation({
    mutationFn: ({ command, allowed_role }: Permission) => api.put(`/api/permissions/${command}`, { allowed_role }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['permissions'] }); toast('Права обновлены', 'success') },
  })

  if (q.isLoading) return <LoadingSpinner />

  return (
    <div>
      <h2 style={{ marginBottom: '0.5rem', fontSize: '1.25rem' }}>Действия</h2>

      <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'flex-start', marginBottom: '1.5rem', padding: '0.75rem', borderRadius: 'var(--radius-sm)', background: 'var(--accent-glass)', fontSize: '0.85rem', color: 'var(--text-muted)' }}>
        <Info size={18} color="var(--accent)" style={{ flexShrink: 0, marginTop: '1px' }} />
        <span>
          Управление доступом к командам бота. <b>«все»</b> — команду может использовать любой участник группы, <b>«админ»</b> — только администраторы (создатель и администраторы группы).
        </span>
      </div>

      <div className="card" style={{ padding: 0, maxWidth: 480 }}>
        {q.data?.map((p) => (
          <div key={p.command} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '0.75rem 1rem', borderBottom: '1px solid var(--glass-border)' }}>
            <span style={{ fontWeight: 500, fontSize: '0.9rem' }}>{commandNames[p.command] || p.command}</span>
            <Select
              value={p.allowed_role}
              options={[
                { value: 'everyone', label: 'все' },
                { value: 'admin', label: 'админ' },
              ]}
              onChange={(role) => mut.mutate({ command: p.command, allowed_role: role })}
            />
          </div>
        )) || <div className="empty-state">Нет данных</div>}
      </div>
    </div>
  )
}
