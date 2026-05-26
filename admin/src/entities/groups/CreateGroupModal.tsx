import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../../shared/api/client'
import { toast } from '../../shared/ui/Toast'
import Modal from '../../shared/ui/Modal'
import { Group } from './hooks'

interface CreateGroupModalProps {
  open: boolean
  onClose: () => void
  onCreated: (chatID: number) => void
}

export default function CreateGroupModal({ open, onClose, onCreated }: CreateGroupModalProps) {
  const qc = useQueryClient()
  const [form, setForm] = useState({
    chat_id: 0,
    title: '',
    omsu_group_id: 0,
    is_active: true,
    is_vip: false,
  })

  const createMut = useMutation({
    mutationFn: (data: typeof form) => api.post<Group>('/api/groups', data),
    onSuccess: (data) => {
      qc.invalidateQueries({ queryKey: ['groups'] })
      onClose()
      onCreated(data.chat_id)
      setForm({ chat_id: 0, title: '', omsu_group_id: 0, is_active: true, is_vip: false })
      toast('Группа добавлена', 'success')
    },
    onError: (err: any) => toast(`Ошибка: ${err.message}`, 'error'),
  })

  return (
    <Modal open={open} onClose={onClose} title="Добавить группу">
      <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
        <div>
          <label style={{ display: 'block', marginBottom: '0.35rem', fontSize: '0.85rem', color: 'var(--text-muted)' }}>Telegram Chat ID</label>
          <input className="input" type="number" value={form.chat_id || ''}
            onChange={(e) => setForm(f => ({ ...f, chat_id: Number(e.target.value) }))}
            placeholder="Например, -100123456789" />
        </div>
        <div>
          <label style={{ display: 'block', marginBottom: '0.35rem', fontSize: '0.85rem', color: 'var(--text-muted)' }}>Название группы</label>
          <input className="input" value={form.title}
            onChange={(e) => setForm(f => ({ ...f, title: e.target.value }))}
            placeholder="Например, ИТБ-101" />
        </div>
        <div>
          <label style={{ display: 'block', marginBottom: '0.35rem', fontSize: '0.85rem', color: 'var(--text-muted)' }}>Omsu Group ID (Setka)</label>
          <input className="input" type="number" value={form.omsu_group_id || ''}
            onChange={(e) => setForm(f => ({ ...f, omsu_group_id: Number(e.target.value) }))} />
        </div>
        <div style={{ display: 'flex', gap: '1rem', marginTop: '0.5rem' }}>
          <label style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', cursor: 'pointer' }}>
            <input type="checkbox" checked={form.is_active}
              onChange={(e) => setForm(f => ({ ...f, is_active: e.target.checked }))} />
            <span style={{ fontSize: '0.85rem' }}>Активна</span>
          </label>
          <label style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', cursor: 'pointer' }}>
            <input type="checkbox" checked={form.is_vip}
              onChange={(e) => setForm(f => ({ ...f, is_vip: e.target.checked }))} />
            <span style={{ fontSize: '0.85rem' }}>VIP</span>
          </label>
        </div>
        <button className="btn btn-primary"
          onClick={() => {
            if (!form.chat_id || !form.title) {
              toast('Chat ID и Название обязательны', 'error')
              return
            }
            createMut.mutate(form)
          }}
          disabled={createMut.isPending}
          style={{ marginTop: '0.5rem', justifyContent: 'center' }}>
          {createMut.isPending ? 'Создание...' : 'Добавить'}
        </button>
      </div>
    </Modal>
  )
}
