import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Cpu, Globe, Mic, Camera, UserPlus, Shield, Plus, Trash2, Loader2, RefreshCw, UserCheck, Activity } from 'lucide-react'
import { api } from '../api/client'
import { toast } from '../components/Toast'

interface Config {
  skip_fallback_model: boolean
  global_voice_transcription: boolean
  global_photo_processing: boolean
  group_registration_restricted: boolean
}
interface Superadmin { user_id: number; note: string; created_at: string }
interface RegisterResult { status: string; registered_groups?: number[] }

const configToggles = [
  { key: 'skip_fallback_model' as const, label: 'Fallback-модели', icon: Globe, descOn: 'пропускать', descOff: 'использовать', danger: true },
  { key: 'global_voice_transcription' as const, label: 'Расшифровка голоса', icon: Mic, descOn: 'вкл (глобально)', descOff: 'выкл (глобально)', danger: false },
  { key: 'global_photo_processing' as const, label: 'Обработка фото', icon: Camera, descOn: 'вкл (глобально)', descOff: 'выкл (глобально)', danger: false },
  { key: 'group_registration_restricted' as const, label: 'Регистрация групп', icon: UserPlus, descOn: 'только суперадмин', descOff: 'через /init', danger: true },
]

export default function SettingsPage() {
  const qc = useQueryClient()
  const [newUserId, setNewUserId] = useState('')
  const [newNote, setNewNote] = useState('')

  const configQ = useQuery({
    queryKey: ['config'],
    queryFn: () => api.get<Config>('/api/config'),
  })

  const adminsQ = useQuery<Superadmin[]>({
    queryKey: ['superadmins'],
    queryFn: () => api.get<Superadmin[]>('/api/admin/superadmins'),
  })

  const configMut = useMutation({
    mutationFn: (cfg: Config) => api.put<Config>('/api/config', cfg),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['config'] }); toast('Настройки обновлены', 'success') },
    onError: (err: any) => toast(`Ошибка: ${err.message}`, 'error'),
  })

  const addMut = useMutation({
    mutationFn: ({ user_id, note }: { user_id: number; note: string }) =>
      api.post('/api/admin/superadmins', { user_id, note }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['superadmins'] })
      setNewUserId(''); setNewNote('')
      toast('Суперадмин добавлен', 'success')
    },
    onError: (err: any) => toast(`Ошибка: ${err.message}`, 'error'),
  })

  const removeMut = useMutation({
    mutationFn: (user_id: number) => api.delete(`/api/admin/superadmins/${user_id}`),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['superadmins'] }); toast('Суперадмин удалён', 'success') },
    onError: (err: any) => toast(`Ошибка: ${err.message}`, 'error'),
  })

  const registerMut = useMutation({
    mutationFn: () => api.post<RegisterResult>('/api/admin/groups/register-webhooks'),
    onSuccess: (data) => {
      const msg = data.status === 'no_active_groups_to_register'
        ? 'Нет активных групп для регистрации'
        : `Вебхуки зарегистрированы для ${data.registered_groups?.length ?? 0} групп`
      toast(msg, 'success')
    },
    onError: (err: any) => toast(`Ошибка: ${err.message}`, 'error'),
  })

  const syncMut = useMutation({
    mutationFn: () => api.post('/api/admin/sync-trigger'),
    onSuccess: () => toast('Синхронизация запущена', 'success'),
    onError: (err: any) => toast(`Ошибка: ${err.message}`, 'error'),
  })

  const cfg = configQ.data ?? { skip_fallback_model: false, global_voice_transcription: true, global_photo_processing: true, group_registration_restricted: false }

  const handleAdd = () => {
    const uid = parseInt(newUserId, 10)
    if (!uid || isNaN(uid)) { toast('Введите корректный Telegram User ID', 'error'); return }
    addMut.mutate({ user_id: uid, note: newNote })
  }

  return (
    <div>
      <h2 style={{ marginBottom: '1.5rem', fontSize: '1.25rem' }}>Настройки</h2>

      <div style={{ display: 'flex', flexDirection: 'column', gap: '1.5rem', maxWidth: 680 }}>

        {/* Global config */}
        <div className="card">
          <h3 style={{ fontSize: '1rem', marginBottom: '0.75rem', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
            <Cpu size={18} color="var(--accent)" /> Глобальные настройки
          </h3>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
            {configToggles.map(t => {
              const Icon = t.icon
              const isOn = cfg[t.key]
              return (
                <button key={t.key} className="btn btn-sm" style={{ justifyContent: 'flex-start' }}
                  onClick={() => configMut.mutate({ ...cfg, [t.key]: !isOn })}
                  disabled={configMut.isPending || configQ.isLoading}>
                  <Icon size={16} color={isOn ? (t.danger ? 'var(--danger)' : 'var(--accent)') : 'var(--text-muted)'} />
                  <span style={{ flex: 1, textAlign: 'left' }}>{t.label}</span>
                  <span style={{ fontSize: '0.8rem', color: isOn ? (t.danger ? 'var(--danger)' : 'var(--success)') : 'var(--text-muted)' }}>
                    {isOn ? t.descOn : t.descOff}
                  </span>
                </button>
              )
            })}
          </div>
        </div>

        {/* Setka integration */}
        <div className="card">
          <h3 style={{ fontSize: '1rem', marginBottom: '0.75rem', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
            <RefreshCw size={18} color="var(--accent)" /> Интеграция с omsu_setka
          </h3>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
            <div>
              <p style={{ fontSize: '0.85rem', color: 'var(--text-muted)', marginBottom: '0.5rem' }}>
                Регистрирует webhook-URL всех активных групп в omsu_setka.
              </p>
              <button className="btn btn-primary btn-sm" onClick={() => registerMut.mutate()} disabled={registerMut.isPending}>
                {registerMut.isPending ? <Loader2 size={16} className="spinner" /> : <RefreshCw size={16} />}
                {' '}{registerMut.isPending ? 'Регистрация...' : 'Зарегистрировать вебхуки'}
              </button>
              {registerMut.data && (
                <div style={{ marginTop: '0.5rem', fontSize: '0.85rem', color: 'var(--success)' }}>
                  ✓ {registerMut.data.status === 'no_active_groups_to_register'
                    ? 'Нет активных групп'
                    : `Зарегистрировано групп: ${registerMut.data.registered_groups?.join(', ')}`}
                </div>
              )}
            </div>
            <div style={{ borderTop: '1px solid var(--glass-border)', paddingTop: '0.75rem' }}>
              <p style={{ fontSize: '0.85rem', color: 'var(--text-muted)', marginBottom: '0.5rem' }}>
                Запускает принудительную синхронизацию расписания в omsu_setka.
              </p>
              <button className="btn btn-sm" onClick={() => syncMut.mutate()} disabled={syncMut.isPending}>
                {syncMut.isPending ? <Loader2 size={16} className="spinner" /> : <Activity size={16} />}
                {' '}{syncMut.isPending ? 'Синхронизация...' : 'Запустить синхронизацию'}
              </button>
            </div>
          </div>
        </div>

        {/* Superadmins */}
        <div className="card">
          <h3 style={{ fontSize: '1rem', marginBottom: '0.75rem', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
            <Shield size={18} color="var(--accent)" /> Суперадмины бота
          </h3>
          <p style={{ fontSize: '0.85rem', color: 'var(--text-muted)', marginBottom: '1rem' }}>
            Суперадмины могут управлять настройками бота через Telegram-команды в любой группе.
          </p>

          <div style={{ display: 'flex', gap: '0.5rem', marginBottom: '1rem', flexWrap: 'wrap' }}>
            <input className="input" type="number" placeholder="Telegram User ID"
              value={newUserId} onChange={(e) => setNewUserId(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleAdd()}
              style={{ flex: '1 1 160px' }} />
            <input className="input" placeholder="Примечание (опционально)"
              value={newNote} onChange={(e) => setNewNote(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleAdd()}
              style={{ flex: '2 1 220px' }} />
            <button className="btn btn-primary btn-sm" onClick={handleAdd}
              disabled={addMut.isPending || !newUserId}>
              {addMut.isPending ? <Loader2 size={16} className="spinner" /> : <Plus size={16} />} Добавить
            </button>
          </div>

          {adminsQ.isLoading ? (
            <div style={{ fontSize: '0.85rem', color: 'var(--text-muted)' }}>Загрузка...</div>
          ) : adminsQ.data && adminsQ.data.length > 0 ? (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
              {adminsQ.data.map((a) => (
                <div key={a.user_id} className="toggle-label" style={{ justifyContent: 'space-between' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', flex: 1, minWidth: 0 }}>
                    <UserCheck size={16} color="var(--accent)" />
                    <div style={{ minWidth: 0 }}>
                      <div style={{ fontSize: '0.9rem', fontWeight: 500 }}>ID: {a.user_id}</div>
                      {a.note && <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{a.note}</div>}
                      <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>Добавлен: {a.created_at}</div>
                    </div>
                  </div>
                  <button className="btn btn-sm" style={{ color: 'var(--danger)', flexShrink: 0 }}
                    onClick={() => { if (confirm(`Удалить суперадмина ${a.user_id}?`)) removeMut.mutate(a.user_id) }}
                    disabled={removeMut.isPending} title="Удалить">
                    <Trash2 size={14} />
                  </button>
                </div>
              ))}
            </div>
          ) : (
            <div className="empty-state" style={{ padding: '1.5rem 0' }}>Суперадмины не добавлены</div>
          )}
        </div>

      </div>
    </div>
  )
}
