import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Plus, Trash2, Lock, Unlock } from 'lucide-react'
import { api } from '../api/client'
import DataTable from '../components/DataTable'
import Modal from '../components/Modal'
import { toast } from '../components/Toast'

interface Topic { id: number; tg_thread_id: number; name: string; slug: string; description: string; is_active: boolean; created_at: string }

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
  const [modal, setModal] = useState<{ type: 'create'; topic?: Topic } | null>(null)
  const [form, setForm] = useState({ name: '', slug: '', tg_thread_id: 0, description: '' })

  const q = useQuery({
    queryKey: ['topics', page],
    queryFn: () => api.getPaginated<Topic>('/api/topics', { limit, offset: page * limit }),
  })

  const createMut = useMutation({
    mutationFn: () => api.post('/api/topics', form),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['topics'] }); setModal(null); toast('Топик создан', 'success') },
  })

  const toggleMut = useMutation({
    mutationFn: ({ id, active }: { id: number; active: boolean }) => api.post(`/api/topics/${id}/${active ? 'open' : 'close'}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['topics'] }),
  })

  const deleteMut = useMutation({
    mutationFn: (id: number) => api.delete(`/api/topics/${id}`),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['topics'] }); toast('Топик удалён', 'success') },
  })

  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1rem' }}>
        <h2 style={{ fontSize: '1.25rem' }}>Топики</h2>
        <button className="btn btn-primary btn-sm" onClick={() => { setForm({ name: '', slug: '', tg_thread_id: 0, description: '' }); setModal({ type: 'create' }) }}>
          <Plus size={16} /> Создать
        </button>
      </div>

      <DataTable
        columns={[...columns, { key: 'actions', header: '', width: '100px', render: (t: Topic) => (
          <div style={{ display: 'flex', gap: '0.25rem' }}>
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

      <Modal open={modal !== null} onClose={() => setModal(null)} title="Создать топик">
        <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
          <div><label style={{ display: 'block', marginBottom: '0.35rem', fontSize: '0.85rem', color: 'var(--text-muted)' }}>Название</label><input className="input" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} /></div>
          <div><label style={{ display: 'block', marginBottom: '0.35rem', fontSize: '0.85rem', color: 'var(--text-muted)' }}>Slug</label><input className="input" value={form.slug} onChange={(e) => setForm({ ...form, slug: e.target.value })} /></div>
          <div><label style={{ display: 'block', marginBottom: '0.35rem', fontSize: '0.85rem', color: 'var(--text-muted)' }}>Thread ID</label><input className="input" type="number" value={form.tg_thread_id || ''} onChange={(e) => setForm({ ...form, tg_thread_id: Number(e.target.value) })} /></div>
          <div><label style={{ display: 'block', marginBottom: '0.35rem', fontSize: '0.85rem', color: 'var(--text-muted)' }}>Описание</label><textarea className="input" rows={3} value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })} /></div>
          <button className="btn btn-primary" onClick={() => createMut.mutate()} disabled={createMut.isPending}>{createMut.isPending ? 'Создание...' : 'Создать'}</button>
        </div>
      </Modal>
    </div>
  )
}
