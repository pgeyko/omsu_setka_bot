import { Save, Trash2 } from 'lucide-react'
import LoadingSpinner from '../../shared/ui/LoadingSpinner'

interface GroupSettingsProps {
  metaForm: {
    chat_id: number
    title: string
    api_token: string
    omsu_group_id: number
    is_active: boolean
    is_vip: boolean
  }
  featuresForm: Record<string, boolean>
  featuresLoading: boolean
  onMetaChange: (updates: Partial<GroupSettingsProps['metaForm']>) => void
  onFeatureChange: (id: string, value: boolean) => void
  onSave: () => void
  onDelete: () => void
  savePending: boolean
  deletePending: boolean
}

export default function GroupSettings({
  metaForm, featuresForm, featuresLoading,
  onMetaChange, onFeatureChange,
  onSave, onDelete, savePending, deletePending,
}: GroupSettingsProps) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem', width: '100%', maxWidth: '100%', overflowX: 'hidden' }}>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(160px, 1fr))', gap: '1rem', width: '100%' }}>
        <div>
          <label style={{ display: 'block', marginBottom: '0.35rem', fontSize: '0.8rem', color: 'var(--text-muted)' }}>Название группы</label>
          <input className="input" value={metaForm.title} onChange={(e) => onMetaChange({ title: e.target.value })} />
        </div>
        <div>
          <label style={{ display: 'block', marginBottom: '0.35rem', fontSize: '0.8rem', color: 'var(--text-muted)' }}>Omsu Group ID (Setka)</label>
          <input className="input" type="number" value={metaForm.omsu_group_id || ''} onChange={(e) => onMetaChange({ omsu_group_id: Number(e.target.value) })} />
        </div>
      </div>

      <div style={{ display: 'flex', gap: '1rem', flexWrap: 'wrap', borderTop: '1px solid var(--glass-border)', borderBottom: '1px solid var(--glass-border)', padding: '1rem 0' }}>
        <label style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', cursor: 'pointer' }}>
          <input type="checkbox" checked={metaForm.is_active} onChange={(e) => onMetaChange({ is_active: e.target.checked })} />
          <span style={{ fontSize: '0.9rem' }}>Активна</span>
        </label>
        <label style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', cursor: 'pointer' }}>
          <input type="checkbox" checked={metaForm.is_vip} onChange={(e) => onMetaChange({ is_vip: e.target.checked })} />
          <span style={{ fontSize: '0.9rem' }}>VIP статус</span>
        </label>
      </div>

      <div>
        <h4 style={{ fontSize: '0.9rem', fontWeight: 600, marginBottom: '0.75rem' }}>Основные модули</h4>
        {featuresLoading ? (
          <LoadingSpinner />
        ) : (
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(160px, 1fr))', gap: '0.75rem', marginBottom: '1.25rem', width: '100%' }}>
            {[
              { id: 'enable_schedule', label: 'Расписание занятий' },
              { id: 'enable_summary', label: 'Суммаризация топиков' },
              { id: 'enable_voice_transcription', label: 'Расшифровка аудиосообщений' },
            ].map(f => (
              <label key={f.id} className="toggle-label">
                <input type="checkbox" checked={!!featuresForm[f.id]} onChange={(e) => onFeatureChange(f.id, e.target.checked)} />
                <span>{f.label}</span>
              </label>
            ))}
            <label className="toggle-label">
              <input type="checkbox" checked={!!featuresForm.enable_photo_processing}
                onChange={(e) => {
                  onFeatureChange('enable_photo_processing', e.target.checked)
                  if (!e.target.checked) onFeatureChange('photo_on_mention', false)
                }}
              />
              <span>Обработка фото</span>
            </label>
            {featuresForm.enable_photo_processing && (
              <label className="toggle-label">
                <input type="checkbox" checked={!!featuresForm.photo_on_mention}
                  onChange={(e) => onFeatureChange('photo_on_mention', e.target.checked)}
                />
                <span>Фото только по @упоминанию</span>
              </label>
            )}
          </div>
        )}

        <h4 style={{ fontSize: '0.9rem', fontWeight: 600, marginBottom: '0.75rem' }}>Локальная модерация</h4>
        {featuresLoading ? (
          <LoadingSpinner />
        ) : (
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(160px, 1fr))', gap: '0.75rem', width: '100%' }}>
            {[
              { id: 'enable_moderation', label: 'Общая модерация' },
              { id: 'enable_captcha', label: 'Математическая капча' },
              { id: 'enable_link_filter', label: 'Фильтр ссылок' },
              { id: 'enable_flood_control', label: 'Флуд-контроль' },
            ].map(f => (
              <label key={f.id} className="toggle-label">
                <input type="checkbox" checked={!!featuresForm[f.id]} onChange={(e) => onFeatureChange(f.id, e.target.checked)} />
                <span>{f.label}</span>
              </label>
            ))}
          </div>
        )}
      </div>

      <div style={{ display: 'flex', gap: '0.75rem', marginTop: '1rem', flexWrap: 'wrap', justifyContent: 'space-between' }}>
        <button className="btn btn-primary" onClick={onSave} disabled={savePending}>
          <Save size={16} /> Сохранить настройки
        </button>
        <button className="btn btn-danger" onClick={onDelete} disabled={deletePending}>
          <Trash2 size={16} /> Удалить группу
        </button>
      </div>
    </div>
  )
}
