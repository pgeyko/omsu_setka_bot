import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Edit2, Trash2, Save } from 'lucide-react'
import { api } from '../api/client'
import DataTable from '../components/DataTable'
import Modal from '../components/Modal'
import { toast } from '../components/Toast'

interface PromptItem { name: string; size: number }
interface PromptContent { name: string; content: string }

const promptNames: Record<string, string> = {
  classify: 'Классификация сообщений',
  forward_intent: 'Намерение пересылки',
  topic_command: 'Команды топиков',
  schedule_announce: 'Объявления расписания',
}

export default function PromptsPage() {
  const qc = useQueryClient()
  const [page, setPage] = useState(0)
  const [editing, setEditing] = useState<PromptContent | null>(null)
  const [content, setContent] = useState('')

  const listQ = useQuery({
    queryKey: ['prompts'],
    queryFn: () => api.get<PromptItem[]>('/api/prompts'),
  })

  const saveMut = useMutation({
    mutationFn: () => api.put(`/api/prompts/${editing!.name}`, { content }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['prompts'] }); setEditing(null); toast('Промпт сохранён', 'success') },
  })

  const deleteMut = useMutation({
    mutationFn: (name: string) => api.delete(`/api/prompts/${name}`),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['prompts'] }); toast('Промпт удалён', 'success') },
  })

  const openEdit = async (item: PromptItem) => {
    const data = await api.get<PromptContent>(`/api/prompts/${item.name}`)
    setEditing(data)
    setContent(data.content)
  }

  return (
    <div>
      <h2 style={{ marginBottom: '1rem', fontSize: '1.25rem' }}>Системные промпты</h2>

      <DataTable
        columns={[
          { key: 'name', header: 'Имя', render: (item: PromptItem) => promptNames[item.name] || item.name },
          { key: 'size', header: 'Размер (байт)', width: '120px', align: 'right' },
          { key: 'actions', header: '', width: '100px', render: (item: PromptItem) => (
            <div style={{ display: 'flex', gap: '0.25rem' }}>
              <button className="btn btn-sm" onClick={() => openEdit(item)} title="Редактировать"><Edit2 size={14} /></button>
              <button className="btn btn-sm" onClick={() => deleteMut.mutate(item.name)} title="Удалить"><Trash2 size={14} /></button>
            </div>
          )},
        ]}
        data={listQ.data ?? []}
        meta={{ total: (listQ.data || []).length, limit: 100, offset: 0 }}
        loading={listQ.isLoading}
        onPageChange={setPage}
        emptyMessage="Файлы промптов не найдены"
      />

      <Modal open={editing !== null} onClose={() => setEditing(null)} title={`Редактирование: ${editing?.name}`} wide>
        {editing && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
            <label style={{ fontSize: '0.85rem', color: 'var(--text-muted)' }}>Содержимое (.txt)</label>
            <textarea className="input" rows={15} value={content} onChange={(e) => setContent(e.target.value)}
              style={{ fontFamily: 'monospace', fontSize: '0.8rem' }} />
            <button className="btn btn-primary" onClick={() => saveMut.mutate()} disabled={saveMut.isPending}>
              <Save size={16} /> {saveMut.isPending ? 'Сохранение...' : 'Сохранить и перезагрузить'}
            </button>
          </div>
        )}
      </Modal>
    </div>
  )
}
