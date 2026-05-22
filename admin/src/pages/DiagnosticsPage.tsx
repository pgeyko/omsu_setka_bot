import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Activity, Send, MessageSquare, Cpu, CheckCircle, XCircle, Loader2, ToggleLeft, ToggleRight } from 'lucide-react'
import { api } from '../api/client'
import { toast } from '../components/Toast'

interface ProviderStatus {
  name: string; model: string; reachable: boolean; latency?: string; error?: string
}
interface TestResult {
  provider: string; model: string; response: string; latency: string
}
// FIX-07: All 3 config fields now tracked.
interface Config {
  skip_fallback_model: boolean
  global_voice_transcription: boolean
  global_photo_processing: boolean
}
interface GroupSummary { chat_id: number; title: string }

export default function DiagnosticsPage() {
  const qc = useQueryClient()
  const [testPrompt, setTestPrompt] = useState('Ответь одним словом: ты работаешь?')
  const [msgText, setMsgText] = useState('')
  // FIX-09: group selector for bot/send.
  const [selectedChatId, setSelectedChatId] = useState<number | ''>('')

  const configQ = useQuery({
    queryKey: ['config'],
    queryFn: () => api.get<Config>('/api/config'),
  })

  // FIX-09: fetch groups list for dropdown.
  const groupsQ = useQuery<GroupSummary[]>({
    queryKey: ['groups'],
    queryFn: () => api.get<GroupSummary[]>('/api/groups'),
  })

  // FIX-07: mutate sends full Config object.
  const configMut = useMutation({
    mutationFn: (cfg: Config) => api.put<Config>('/api/config', cfg),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['config'] }); toast('Настройки обновлены', 'success') },
    onError: (err: any) => toast(`Ошибка: ${err.message}`, 'error'),
  })

  const checkMut = useMutation({
    mutationFn: () => api.get<ProviderStatus[]>('/api/stats/check-providers'),
    onSuccess: () => toast('Проверка завершена', 'success'),
  })

  const testMut = useMutation({
    mutationFn: (prompt: string) => api.post<TestResult>('/api/stats/test-model', { prompt }),
    onSuccess: () => toast('Модель ответила', 'success'),
    onError: (err: any) => toast(`Ошибка: ${err.message}`, 'error'),
  })

  // FIX-09: pass chat_id in body.
  const sendMut = useMutation({
    mutationFn: ({ text, chat_id }: { text: string; chat_id: number }) =>
      api.post('/api/bot/send', { text, chat_id }),
    onSuccess: () => { setMsgText(''); toast('Сообщение отправлено', 'success') },
    onError: (err: any) => toast(`Ошибка: ${err.message}`, 'error'),
  })

  const cfg = configQ.data ?? { skip_fallback_model: false, global_voice_transcription: true, global_photo_processing: true }

  return (
    <div>
      <h2 style={{ marginBottom: '1.5rem', fontSize: '1.25rem' }}>Диагностика</h2>

      <div style={{ display: 'flex', flexDirection: 'column', gap: '1.5rem', maxWidth: 640 }}>

        {/* LLM Strategy */}
        <div className="card">
          <h3 style={{ fontSize: '1rem', marginBottom: '0.75rem', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
            <Cpu size={18} color="var(--accent)" /> Стратегия провайдеров
          </h3>
          <ol style={{ fontSize: '0.85rem', color: 'var(--text-muted)', marginLeft: '1.25rem', lineHeight: '1.8', marginBottom: '0.75rem' }}>
            <li><b>gemini-primary</b> → 3.1-flash-lite {!cfg.skip_fallback_model ? <span style={{color:'var(--text)'}}>→ 2.5-flash-lite</span> : <span style={{color:'var(--danger)'}}>( пропущен)</span>}</li>
            <li><b>gemini-reserve</b> → 3.1-flash-lite {!cfg.skip_fallback_model ? <span style={{color:'var(--text)'}}>→ 2.5-flash-lite</span> : <span style={{color:'var(--danger)'}}>( пропущен)</span>}</li>
            <li><b>deepseek</b> → deepseek-chat</li>
          </ol>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
            {/* skip_fallback_model */}
            <button id="toggle-fallback" className="btn btn-sm" style={{ justifyContent: 'flex-start' }}
              onClick={() => configMut.mutate({ ...cfg, skip_fallback_model: !cfg.skip_fallback_model })}
              disabled={configMut.isPending || configQ.isLoading}>
              {cfg.skip_fallback_model ? <ToggleRight size={18} color="var(--danger)" /> : <ToggleLeft size={18} />}
              {' '}{cfg.skip_fallback_model ? 'Fallback-модели: пропускать' : 'Fallback-модели: использовать'}
            </button>
            {/* global_voice_transcription — FIX-07 */}
            <button id="toggle-voice" className="btn btn-sm" style={{ justifyContent: 'flex-start' }}
              onClick={() => configMut.mutate({ ...cfg, global_voice_transcription: !cfg.global_voice_transcription })}
              disabled={configMut.isPending || configQ.isLoading}>
              {cfg.global_voice_transcription ? <ToggleRight size={18} color="var(--accent)" /> : <ToggleLeft size={18} />}
              {' '}{cfg.global_voice_transcription ? 'Расшифровка голоса: вкл (глобально)' : 'Расшифровка голоса: выкл (глобально)'}
            </button>
            {/* global_photo_processing — FIX-07 */}
            <button id="toggle-photo" className="btn btn-sm" style={{ justifyContent: 'flex-start' }}
              onClick={() => configMut.mutate({ ...cfg, global_photo_processing: !cfg.global_photo_processing })}
              disabled={configMut.isPending || configQ.isLoading}>
              {cfg.global_photo_processing ? <ToggleRight size={18} color="var(--accent)" /> : <ToggleLeft size={18} />}
              {' '}{cfg.global_photo_processing ? 'Обработка фото: вкл (глобально)' : 'Обработка фото: выкл (глобально)'}
            </button>
          </div>
        </div>

        {/* Provider check */}
        <div className="card">
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '0.75rem' }}>
            <h3 style={{ fontSize: '1rem', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
              <Cpu size={18} color="var(--accent)" /> Провайдеры
            </h3>
            <button className="btn btn-sm btn-primary" onClick={() => checkMut.mutate()} disabled={checkMut.isPending}>
              {checkMut.isPending ? <Loader2 size={16} className="spinner" /> : <Activity size={16} />}
              {' '}{checkMut.isPending ? 'Проверка...' : 'Проверить'}
            </button>
          </div>
          {checkMut.data && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
              {checkMut.data.map((p) => (
                <div key={p.name} style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', padding: '0.6rem 0.75rem', borderRadius: 'var(--radius-sm)', background: p.reachable ? 'rgba(46,204,113,0.08)' : 'rgba(231,76,60,0.08)' }}>
                  {p.reachable ? <CheckCircle size={18} color="var(--success)" /> : <XCircle size={18} color="var(--danger)" />}
                  <div style={{ flex: 1 }}>
                    <div style={{ fontSize: '0.9rem', fontWeight: 500 }}>{p.name}</div>
                    <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>{p.model}</div>
                  </div>
                  <span style={{ fontSize: '0.8rem', color: p.reachable ? 'var(--success)' : 'var(--danger)' }}>
                    {p.reachable ? p.latency : p.error}
                  </span>
                </div>
              ))}
            </div>
          )}
          {!checkMut.data && !checkMut.isPending && (
            <div style={{ fontSize: '0.85rem', color: 'var(--text-muted)' }}>Нажмите «Проверить» для проверки доступности</div>
          )}
        </div>

        {/* Test model */}
        <div className="card">
          <h3 style={{ fontSize: '1rem', marginBottom: '0.75rem', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
            <MessageSquare size={18} color="var(--accent)" /> Тест модели
          </h3>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
            <textarea className="input" rows={3} value={testPrompt}
              onChange={(e) => setTestPrompt(e.target.value)} placeholder="Введите промпт..."
              style={{ fontFamily: 'monospace', fontSize: '0.8rem' }} />
            <div style={{ display: 'flex', gap: '0.5rem' }}>
              <button className="btn btn-primary btn-sm" onClick={() => testMut.mutate(testPrompt)} disabled={testMut.isPending || !testPrompt}>
                {testMut.isPending ? <Loader2 size={16} className="spinner" /> : <Send size={16} />}
                {' '}{testMut.isPending ? 'Ожидание...' : 'Отправить'}
              </button>
              <button className="btn btn-sm" onClick={() => setTestPrompt('Ответь одним словом: ты работаешь?')}>Сброс</button>
            </div>
          </div>
          {testMut.data && (
            <div style={{ marginTop: '0.75rem', padding: '0.75rem', borderRadius: 'var(--radius-sm)', background: 'var(--glass-bg)', fontSize: '0.85rem' }}>
              <div style={{ display: 'flex', gap: '1rem', marginBottom: '0.5rem', color: 'var(--text-muted)', fontSize: '0.8rem' }}>
                <span>Провайдер: {testMut.data.provider}</span>
                <span>Модель: {testMut.data.model}</span>
                <span>Время: {testMut.data.latency}</span>
              </div>
              <div style={{ whiteSpace: 'pre-wrap', fontFamily: 'monospace', fontSize: '0.8rem' }}>{testMut.data.response}</div>
            </div>
          )}
        </div>

        {/* Send to group — FIX-09 */}
        <div className="card">
          <h3 style={{ fontSize: '1rem', marginBottom: '0.75rem', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
            <Send size={18} color="var(--accent)" /> Отправить в группу
          </h3>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
            <select
              id="send-group-select"
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
            <textarea className="input" rows={3} value={msgText}
              onChange={(e) => setMsgText(e.target.value)} placeholder="Текст сообщения от имени бота..."
              style={{ fontFamily: 'monospace', fontSize: '0.8rem' }} />
            <button
              id="send-message-btn"
              className="btn btn-primary btn-sm"
              onClick={() => sendMut.mutate({ text: msgText, chat_id: selectedChatId as number })}
              disabled={sendMut.isPending || !msgText || !selectedChatId}
              style={{ alignSelf: 'flex-start' }}
            >
              {sendMut.isPending ? <Loader2 size={16} className="spinner" /> : <Send size={16} />}
              {' '}{sendMut.isPending ? 'Отправка...' : 'Отправить'}
            </button>
          </div>
        </div>

      </div>
    </div>
  )
}
