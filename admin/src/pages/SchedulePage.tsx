import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api } from '../api/client'
import DataTable from '../components/DataTable'

interface Snapshot { id: number; created_at: string }
interface Anomaly { id: number; snapshot_id: number; type: string; details: string; notified: boolean; created_at: string }

const snapshotCols = [
  { key: 'id', header: 'ID', width: '60px' },
  { key: 'created_at', header: 'Создан' },
]

const anomalyTypeNames: Record<string, string> = {
  ANOMALY_BUILDING: 'Смена корпуса',
  ANOMALY_ROOM: 'Смена аудитории',
  ANOMALY_SUBJECT: 'Замена предмета',
  ANOMALY_CANCEL: 'Отмена пары',
}

const anomalyCols = [
  { key: 'id', header: 'ID', width: '60px' },
  { key: 'type', header: 'Тип', width: '160px', render: (a: Anomaly) => {
    const colors: Record<string, string> = { ANOMALY_BUILDING: '#e74c3c', ANOMALY_ROOM: '#f39c12', ANOMALY_SUBJECT: '#3498db', ANOMALY_CANCEL: '#e74c3c' }
    return <span style={{ color: colors[a.type] || 'var(--text)' }}>{anomalyTypeNames[a.type] || a.type.replace('ANOMALY_', '')}</span>
  }},
  { key: 'notified', header: 'Уведомлён', width: '100px', render: (a: Anomaly) => <span className={`badge badge-${a.notified ? 'active' : 'inactive'}`}>{a.notified ? 'Да' : 'Нет'}</span> },
  { key: 'created_at', header: 'Создан' },
]

export default function SchedulePage() {
  const [tab, setTab] = useState<'snapshots' | 'anomalies'>('anomalies')
  const [page, setPage] = useState(0)
  const limit = 20

  const snapshots = useQuery({
    queryKey: ['snapshots', page],
    queryFn: () => api.getPaginated<Snapshot>('/api/schedule/snapshots', { limit, offset: page * limit }),
    enabled: tab === 'snapshots',
  })

  const anomalies = useQuery({
    queryKey: ['anomalies', page],
    queryFn: () => api.getPaginated<Anomaly>('/api/schedule/anomalies', { limit, offset: page * limit }),
    enabled: tab === 'anomalies',
  })

  return (
    <div>
      <h2 style={{ marginBottom: '1rem', fontSize: '1.25rem' }}>Изменения расписания</h2>

      <div style={{ display: 'flex', gap: '0.5rem', marginBottom: '1rem' }}>
        {(['anomalies', 'snapshots'] as const).map((t) => (
          <button key={t} className={`btn btn-sm ${tab === t ? 'btn-primary' : ''}`} onClick={() => { setTab(t); setPage(0) }}>
            {t === 'snapshots' ? 'Снэпшоты' : 'Аномалии'}
          </button>
        ))}
      </div>

      {tab === 'snapshots' ? (
        <DataTable columns={snapshotCols} data={snapshots.data?.data ?? []}
          meta={snapshots.data?.meta ?? { total: 0, limit, offset: 0 }}
          loading={snapshots.isLoading} onPageChange={setPage} emptyMessage="Нет снэпшотов" />
      ) : (
        <DataTable columns={anomalyCols} data={anomalies.data?.data ?? []}
          meta={anomalies.data?.meta ?? { total: 0, limit, offset: 0 }}
          loading={anomalies.isLoading} onPageChange={setPage} emptyMessage="Аномалий не обнаружено" />
      )}
    </div>
  )
}
