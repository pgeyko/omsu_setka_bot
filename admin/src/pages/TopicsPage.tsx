import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Plus, Trash2, Lock, Unlock, Filter, Pencil } from 'lucide-react'
import { api } from '../api/client'
import DataTable from '../components/DataTable'
import Modal from '../components/Modal'
import { toast } from '../components/Toast'

interface Topic { id: number; tg_thread_id: number; name: string; slug: string; description: string; is_active: boolean; created_at: string; group_id?: number }
interface GroupSummary { chat_id: number; title: string }

const columns = [
  { key: 'id', header: 'ID', width: '60px' },
  { key: 'name', header: 'Название' },
  { key: 'slug', header: 'Slug', width: '120px' },
  { key: 'tg_thread_id', header: 'Thread ID', width: '100px' },
  { key: 'is_active', header: 'Статус', width: '80px', render: (t: Topic) => <span className={`badge badge-${t.is_active ? 'active' : 'inactive'}`}>{t.is_active ? 'Активен' : 'Закрыт'}</span> },
  { key: 'created_at', header: 'Создан', width: '140px' },
]

export default function TopicsPage() {
  const qc = useQueryClient()
  const [page, setPage] = useState(0)
  const limit = 20
  const [modal, setModal] = useState<{ type: 'create' | 'edit'; topic?: Topic } | null>(null)
  const [form, setForm] = useState({ name: '', slug: '', tg_thread_id: 0, description: '', group_id: 0 })
  // FIX-14: group filter
  const [filterGroupId, setFilterGroupId] = useState<number | ''>('')

  const groupsQ = useQuery<GroupSummary[]>({
    queryKey: ['groups'],
    queryFn: () => api.get<GroupSummary[]>('/api/groups'),
  })

  const topicsUrl = filterGroupId ? `/api/topics?group_id=${filterGroupId}` : '/api/topics'
  const q = useQuery({
    queryKey: ['topics', page, filterGroupId],
    queryFn: () => api.getPaginated<Topic>(topicsUrl, { limit, offset: page * limit }),
  })

  const createMut = useMutation({
    mutationFn: () => api.post('/api/topics', form),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['topics'] }); setModal(null); toast('Топик создан', 'success') },
    onError: (err: any) => toast(`Ошибка: ${err.message}`, 'error'),
  })

  const updateMut = useMutation({
    mutationFn: ({ id, data }: { id: number; data: Partial<typeof form> }) => api.put(`/api/topics/${id}`, data),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['topics'] }); setModal(null); toast('Топик обновлён', 'success') },
    onError: (err: any) => toast(`Ошибка: ${err.message}`, 'error'),
  })

  const toggleMut = useMutation({
    mutationFn: ({ id, active }: { id: number; active: boolean }) => api.post(`/api/topics/${id}/${active ? 'open' : 'close'}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['topics'] }),
    onError: (err: any) => toast(`Ошибка: ${err.message}`, 'error'),
  })

  const deleteMut = useMutation({
    mutationFn: (id: number) => api.delete(`/api/topics/${id}`),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['topics'] }); toast('Топик удалён', 'success') },
    onError: (err: any) => toast(`Ошибка: ${err.message}`, 'error'),
  })

  const openCreate = () => {
    setForm({ name: '', slug: '', tg_thread_id: 0, description: '', group_id: filterGroupId ? Number(filterGroupId) : 0 })
    setModal({ type: 'create' })
  }

  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1rem', flexWrap: 'wrap', gap: '0.5rem' }}>
        <h2 style={{ fontSize: '1.25rem' }}>Топики</h2>
        <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'center' }}>
          {/* FIX-14: group filter dropdown */}
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.35rem', fontSize: '0.85rem', color: 'var(--text-muted)' }}>
            <Filter size={14} />
            <select
              id="topics-group-filter"
              className="input"
              value={filterGroupId}
              onChange={(e) => { setFilterGroupId(e.target.value ? Number(e.target.value) : ''); setPage(0) }}
              style={{ padding: '0.3rem 0.5rem', fontSize: '0.8rem', width: 'auto' }}
            >
              <option value="">Все группы</option>
              {groupsQ.data?.map((g) => (
                <option key={g.chat_id} value={g.chat_id}>{g.title}</option>
              ))}
            </select>
          </div>
          <button id="topics-create-btn" className="btn btn-primary btn-sm" onClick={openCreate}>
            <Plus size={16} /> Создать
          </button>
        </div>
      </div>

      <DataTable
        columns={[...columns, { key: 'actions', header: '', width: '100px', render: (t: Topic) => (
          <div style={{ display: 'flex', gap: '0.25rem' }}>
            <button className="btn btn-sm" onClick={() => { setForm({ name: t.name, slug: t.slug, tg_thread_id: t.tg_thread_id, description: t.description || '', group_id: t.group_id || 0 }); setModal({ type: 'edit', topic: t }) }} title="Редактировать">
              <Pencil size={14} />
            </button>
            <button className="btn btn-sm" onClick={() => toggleMut.mutate({ id: t.id, active: !t.is_active })} title={t.is_active ? 'Закрыть' : 'Открыть'}>
              {t.is_active ? <Lock size={14} /> : <Unlock size={14} />}
            </button>
            <button className="btn btn-sm" onClick={() => deleteMut.mutate(t.id)} title="Удалить"><Trash2 size={14} /></button>
          </div>
        )}]}
        data={q.data?.data ?? []}
        meta={q.data?.meta ?? { total: 0, limit, offset: 0 }}
        loading={q.isLoading}
        onPageChange={setPage}
        emptyMessage="Топики ещё не созданы"
      />

      <Modal open={modal !== null} onClose={() => setModal(null)} title={modal?.type === 'edit' ? 'Редактировать топик' : 'Создать топик'}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
          {modal?.type === 'create' && (
            <div>
              <label style={{ display: 'block', marginBottom: '0.35rem', fontSize: '0.85rem', color: 'var(--text-muted)' }}>Группа</label>
              <select className="input" value={form.group_id || ''} onChange={(e) => setForm({ ...form, group_id: Number(e.target.value) })}>
                <option value="">— Выберите группу —</option>
                {groupsQ.data?.map((g) => (
                  <option key={g.chat_id} value={g.chat_id}>{g.title}</option>
                ))}
              </select>
            </div>
          )}
          <div><label style={{ display: 'block', marginBottom: '0.35rem', fontSize: '0.85rem', color: 'var(--text-muted)' }}>Название</label><input className="input" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} /></div>
          <div><label style={{ display: 'block', marginBottom: '0.35rem', fontSize: '0.85rem', color: 'var(--text-muted)' }}>Slug</label><input className="input" value={form.slug} onChange={(e) => setForm({ ...form, slug: e.target.value })} /></div>
          {modal?.type === 'create' && (
            <div><label style={{ display: 'block', marginBottom: '0.35rem', fontSize: '0.85rem', color: 'var(--text-muted)' }}>Thread ID</label><input className="input" type="number" value={form.tg_thread_id || ''} onChange={(e) => setForm({ ...form, tg_thread_id: Number(e.target.value) })} /></div>
          )}
          <div><label style={{ display: 'block', marginBottom: '0.35rem', fontSize: '0.85rem', color: 'var(--text-muted)' }}>Описание</label><textarea className="input" rows={3} value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })} /></div>
          {modal?.type === 'edit' ? (
            <button className="btn btn-primary" onClick={() => updateMut.mutate({ id: modal.topic!.id, data: { name: form.name, slug: form.slug, description: form.description } })} disabled={updateMut.isPending}>
              {updateMut.isPending ? 'Сохранение...' : 'Сохранить'}
            </button>
          ) : (
            <button className="btn btn-primary" onClick={() => createMut.mutate()} disabled={createMut.isPending || !form.group_id}>{createMut.isPending ? 'Создание...' : 'Создать'}</button>
          )}
        </div>
      </Modal>
    </div>
  )
}
