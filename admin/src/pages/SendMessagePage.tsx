import { useState } from 'react'
import { useQuery, useMutation } from '@tanstack/react-query'
import { Send, Loader2, MessageSquare } from 'lucide-react'
import { api } from '../api/client'
import { toast } from '../components/Toast'

interface GroupSummary { chat_id: number; title: string }

export default function SendMessagePage() {
  const [selectedChatId, setSelectedChatId] = useState<number | ''>('')
  const [msgText, setMsgText] = useState('')

  const groupsQ = useQuery<GroupSummary[]>({
    queryKey: ['groups'],
    queryFn: () => api.get<GroupSummary[]>('/api/groups'),
  })

  const sendMut = useMutation({
    mutationFn: ({ text, chat_id }: { text: string; chat_id: number }) =>
      api.post('/api/bot/send', { text, chat_id }),
    onSuccess: () => { setMsgText(''); toast('Сообщение отправлено', 'success') },
    onError: (err: any) => toast(`Ошибка: ${err.message}`, 'error'),
  })

  return (
    <div>
      <h2 style={{ marginBottom: '1.5rem', fontSize: '1.25rem' }}>Отправить сообщение</h2>

      <div style={{ maxWidth: 640 }}>
        <div className="card">
          <h3 style={{ fontSize: '1rem', marginBottom: '0.75rem', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
            <Send size={18} color="var(--accent)" /> Сообщение в группу
          </h3>
          <p style={{ fontSize: '0.85rem', color: 'var(--text-muted)', marginBottom: '1rem' }}>
            Отправляет текстовое сообщение от имени бота в выбранную группу.
          </p>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
            <select
              className="input"
              value={selectedChatId}
              onChange={(e) => setSelectedChatId(e.target.value ? Number(e.target.value) : '')}
              style={{ fontSize: '0.85rem' }}
            >
              <option value="">— Выберите группу —</option>
              {groupsQ.data?.map((g) => (
                <option key={g.chat_id} value={g.chat_id}>{g.title} ({g.chat_id})</option>
              ))}
            </select>
            <textarea className="input" rows={5} value={msgText}
              onChange={(e) => setMsgText(e.target.value)} placeholder="Текст сообщения от имени бота..."
              style={{ fontFamily: 'monospace', fontSize: '0.85rem' }} />
            <button
              className="btn btn-primary"
              onClick={() => sendMut.mutate({ text: msgText, chat_id: selectedChatId as number })}
              disabled={sendMut.isPending || !msgText || !selectedChatId}
              style={{ alignSelf: 'flex-start' }}
            >
              {sendMut.isPending ? <Loader2 size={16} className="spinner" /> : <Send size={16} />}
              {' '}{sendMut.isPending ? 'Отправка...' : 'Отправить'}
            </button>
          </div>
          {groupsQ.data && groupsQ.data.length === 0 && (
            <div style={{ marginTop: '0.75rem', fontSize: '0.85rem', color: 'var(--text-muted)' }}>
              <MessageSquare size={14} style={{ marginRight: '0.35rem', verticalAlign: 'middle' }} />
              Нет доступных групп. Сначала создайте группу.
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
