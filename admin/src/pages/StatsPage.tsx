import { ReactNode } from 'react'
import { useQuery, useMutation } from '@tanstack/react-query'
import { BarChart3, Activity, Send, MessageSquare, Cpu, RefreshCw, CheckCircle, XCircle } from 'lucide-react'
import { api } from '../api/client'
import LoadingSpinner from '../components/LoadingSpinner'

interface Tokens { total_today: number; by_provider: Record<string, number> }
interface Requests { total_today: number; by_type: Record<string, number> }
interface Forwards { auto_forwards: number; manual_forwards: number }
interface Messages { total_processed: number; skipped: number; forwarded: number; low_confidence: number }
interface ProviderStat { name: string; requests: number; tokens: number; cost_usd: number; last_used: string }

interface TestResult {
  success: boolean
  latency?: string
  response?: string
  error?: string
}

interface ModelResult {
  name: string
  type: string
  model: string
  priority: number
  active: boolean
  test?: TestResult
}

interface TestAllResponse {
  results: Record<string, ModelResult[]>
  summary: { total: number; ok: number; failed: number }
}

function StatCard({ icon, label, value, sub }: { icon: ReactNode; label: string; value: string | number; sub?: string }) {
  return (
    <div className="card" style={{ minWidth: 160 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', marginBottom: '0.5rem' }}>
        {icon}
        <span style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>{label}</span>
      </div>
      <div style={{ fontSize: '1.5rem', fontWeight: 700 }}>{value}</div>
      {sub && <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)', marginTop: '0.25rem' }}>{sub}</div>}
    </div>
  )
}

const typeNames: Record<string, string> = {
  classify: 'Классификация',
  forward_intent: 'Намерение пересылки',
  topic_command: 'Команды топиков',
  schedule_announce: 'Объявления',
  summary: 'Саммари',
}

const providerTypeNames: Record<string, string> = {
  groq: 'Groq',
  gemini: 'Gemini',
  openrouter: 'OpenRouter',
}

function ModelTestRow({ m }: { m: ModelResult }) {
  const ok = m.test?.success
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', padding: '0.6rem 1rem', borderBottom: '1px solid var(--glass-border)', fontSize: '0.85rem' }}>
      {ok ? <CheckCircle size={16} color="var(--success)" /> : <XCircle size={16} color="var(--danger)" />}
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{ fontWeight: 500, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{m.name}</div>
        <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{m.model}</div>
      </div>
      {m.test?.latency && <span style={{ color: 'var(--text-muted)', fontSize: '0.8rem', whiteSpace: 'nowrap' }}>{m.test.latency}</span>}
      {m.test?.response && <span style={{ color: 'var(--text-muted)', fontSize: '0.8rem', maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>«{m.test.response}»</span>}
      {!ok && m.test?.error && <span style={{ color: 'var(--danger)', fontSize: '0.8rem', maxWidth: 300, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={m.test.error}>{m.test.error}</span>}
    </div>
  )
}

export default function StatsPage() {
  const tokens = useQuery({ queryKey: ['stats', 'tokens'], queryFn: () => api.get<Tokens>('/api/stats/tokens') })
  const requests = useQuery({ queryKey: ['stats', 'requests'], queryFn: () => api.get<Requests>('/api/stats/requests') })
  const forwards = useQuery({ queryKey: ['stats', 'forwards'], queryFn: () => api.get<Forwards>('/api/stats/forwards') })
  const messages = useQuery({ queryKey: ['stats', 'messages'], queryFn: () => api.get<Messages>('/api/stats/messages') })
  const providerStats = useQuery({ queryKey: ['stats', 'providers'], queryFn: () => api.get<ProviderStat[]>('/api/stats/providers') })

  const testMutation = useMutation({
    mutationFn: () => api.post<TestAllResponse>('/api/stats/test-all-models'),
  })

  const loading = tokens.isLoading || requests.isLoading || forwards.isLoading || messages.isLoading

  if (loading) return <LoadingSpinner />

  const testData = testMutation.data

  return (
    <div>
      <h2 style={{ marginBottom: '1.5rem', fontSize: '1.25rem' }}>Статистика</h2>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))', gap: '1rem', marginBottom: '2rem' }}>
        <StatCard icon={<BarChart3 size={20} color="var(--accent)" />} label="Токенов сегодня" value={tokens.data?.total_today ?? 0} />
        <StatCard icon={<Activity size={20} color="var(--success)" />} label="LLM запросов" value={requests.data?.total_today ?? 0} />
        <StatCard icon={<Send size={20} color="var(--warning)" />} label="Автопересылок" value={forwards.data?.auto_forwards ?? 0} />
        <StatCard icon={<MessageSquare size={20} />} label="Обработано сообщений" value={messages.data?.total_processed ?? 0}
          sub={`${messages.data?.forwarded ?? 0} переслано / ${messages.data?.skipped ?? 0} пропущено`} />
      </div>

      <h3 style={{ fontSize: '1rem', marginBottom: '0.75rem' }}>LLM Провайдеры</h3>
      {providerStats.data && providerStats.data.length > 0 ? (
        <div className="card" style={{ overflow: 'hidden', padding: 0, maxWidth: 640 }}>
          {providerStats.data.map((p, i) => (
            <div key={p.name} style={{ display: 'flex', alignItems: 'center', gap: '1rem', padding: '0.75rem 1rem', borderBottom: i < providerStats.data.length - 1 ? '1px solid var(--glass-border)' : 'none' }}>
              <Cpu size={16} color="var(--accent)" />
              <div style={{ flex: 1 }}>
                <div style={{ fontSize: '0.9rem', fontWeight: 500 }}>{p.name}</div>
                <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>использован: {p.last_used || 'никогда'}</div>
              </div>
              <div style={{ textAlign: 'right', fontSize: '0.85rem' }}>
                <div>{p.requests} запр.</div>
                <div style={{ color: 'var(--text-muted)' }}>{p.tokens} токенов</div>
              </div>
            </div>
          ))}
        </div>
      ) : (
        <div className="empty-state" style={{ textAlign: 'left', padding: '1rem' }}>Нет данных о провайдерах</div>
      )}

      <div style={{ marginTop: '1.5rem' }}>
        <button className="btn" onClick={() => testMutation.mutate()} disabled={testMutation.isPending}
          style={{ display: 'inline-flex', alignItems: 'center', gap: '0.5rem' }}>
          {testMutation.isPending ? <div className="spinner" style={{ width: 16, height: 16, borderWidth: 2, margin: 0, display: 'inline-block' }} /> : <RefreshCw size={16} />}
          {testMutation.isPending ? 'Тестирование...' : 'Тест всех моделей'}
        </button>
      </div>

      {testMutation.isError && (
        <div className="card" style={{ marginTop: '1rem', padding: '1rem', color: 'var(--danger)' }}>
          Ошибка: {testMutation.error instanceof Error ? testMutation.error.message : 'Неизвестная ошибка'}
        </div>
      )}

      {testData && (
        <div style={{ marginTop: '1.5rem' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '1rem', marginBottom: '0.75rem' }}>
            <h3 style={{ fontSize: '1rem', margin: 0 }}>Результаты тестирования</h3>
            <span style={{ fontSize: '0.85rem', color: 'var(--text-muted)' }}>
              {testData.summary.ok}/{testData.summary.total} успешно
              {testData.summary.failed > 0 && (
                <span style={{ color: 'var(--danger)', marginLeft: '0.5rem' }}>
                  ({testData.summary.failed} ошибок)
                </span>
              )}
            </span>
          </div>

          {Object.entries(testData.results).map(([type, models]) => (
            <div key={type} style={{ marginBottom: '1rem' }}>
              <h4 style={{ fontSize: '0.9rem', marginBottom: '0.5rem', color: 'var(--text-muted)' }}>
                {providerTypeNames[type] || type}
              </h4>
              <div className="card" style={{ overflow: 'hidden', padding: 0, maxWidth: 800 }}>
                {models.map((m) => (
                  <ModelTestRow key={m.name} m={m} />
                ))}
              </div>
            </div>
          ))}
        </div>
      )}

      <h3 style={{ fontSize: '1rem', margin: '1.5rem 0 0.75rem' }}>По типам запросов</h3>
      {requests.data && Object.keys(requests.data.by_type).length > 0 ? (
        <div className="card" style={{ overflow: 'hidden', padding: 0, maxWidth: 480 }}>
          {Object.entries(requests.data.by_type).map(([type, count], i, arr) => (
            <div key={type} style={{ display: 'flex', justifyContent: 'space-between', padding: '0.5rem 1rem', borderBottom: i < arr.length - 1 ? '1px solid var(--glass-border)' : 'none' }}>
              <span>{typeNames[type] || type}</span>
              <span style={{ fontWeight: 600 }}>{count}</span>
            </div>
          ))}
        </div>
      ) : (
        <div className="empty-state" style={{ textAlign: 'left', padding: '1rem' }}>Нет данных</div>
      )}
    </div>
  )
}
