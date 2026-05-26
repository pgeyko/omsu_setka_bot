import { useState, useEffect } from 'react'
import { useQueryClient, useMutation } from '@tanstack/react-query'
import { ArrowLeft, Settings as SettingsIcon, FileText, Brain, Save } from 'lucide-react'
import { api } from '../../shared/api/client'
import { toast } from '../../shared/ui/Toast'
import LoadingSpinner from '../../shared/ui/LoadingSpinner'
import GroupSettings from './GroupSettings'
import { Group, useGroupFeatures, useGroupPrompt, useGroupKnowledge } from './hooks'

interface GroupDetailProps {
  group: Group | undefined
  chatID: number
  onBack: () => void
}

type Tab = 'settings' | 'prompt' | 'knowledge'

export default function GroupDetail({ group, chatID, onBack }: GroupDetailProps) {
  const qc = useQueryClient()
  const [activeTab, setActiveTab] = useState<Tab>('settings')

  const [metaForm, setMetaForm] = useState({
    chat_id: chatID,
    title: group?.title || '',
    api_token: group?.api_token || '',
    omsu_group_id: group?.omsu_group_id || 0,
    is_active: group?.is_active || false,
    is_vip: group?.is_vip || false,
  })

  const [featuresForm, setFeaturesForm] = useState<Record<string, boolean>>({
    enable_schedule: true,
    enable_summary: true,
    enable_moderation: true,
    enable_captcha: true,
    enable_link_filter: true,
    enable_flood_control: true,
    enable_voice_transcription: true,
    enable_photo_processing: true,
  })

  const [promptContent, setPromptContent] = useState('')
  const [knowledgeContent, setKnowledgeContent] = useState('')

  const featuresQ = useGroupFeatures(chatID)
  const promptQ = useGroupPrompt(chatID)
  const knowledgeQ = useGroupKnowledge(chatID)

  useEffect(() => {
    if (group) {
      setMetaForm({
        chat_id: group.chat_id,
        title: group.title,
        api_token: group.api_token || '',
        omsu_group_id: group.omsu_group_id || 0,
        is_active: group.is_active,
        is_vip: group.is_vip,
      })
    }
  }, [group])

  useEffect(() => { if (featuresQ.data?.features) setFeaturesForm(featuresQ.data.features) }, [featuresQ.data])
  useEffect(() => { if (promptQ.data) setPromptContent(promptQ.data.content) }, [promptQ.data])
  useEffect(() => { if (knowledgeQ.data) setKnowledgeContent(knowledgeQ.data.content) }, [knowledgeQ.data])

  const updateMut = useMutation({
    mutationFn: (data: Partial<Group>) => api.put(`/api/groups/${chatID}`, data),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['groups'] }); toast('Настройки обновлены', 'success') },
    onError: (err: any) => toast(`Ошибка: ${err.message}`, 'error'),
  })

  const deleteMut = useMutation({
    mutationFn: () => api.delete(`/api/groups/${chatID}`),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['groups'] }); onBack(); toast('Группа удалена', 'success') },
    onError: (err: any) => toast(`Ошибка: ${err.message}`, 'error'),
  })

  const saveFeaturesMut = useMutation({
    mutationFn: (features: Record<string, boolean>) => api.put(`/api/groups/${chatID}/context/features`, features),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['groups', chatID, 'features'] }); toast('Модули сохранены', 'success') },
    onError: (err: any) => toast(`Ошибка: ${err.message}`, 'error'),
  })

  const savePromptMut = useMutation({
    mutationFn: (content: string) => api.put(`/api/groups/${chatID}/context/system-prompt`, { content }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['groups', chatID, 'system-prompt'] }); toast('Системный промпт сохранен', 'success') },
    onError: (err: any) => toast(`Ошибка: ${err.message}`, 'error'),
  })

  const saveKnowledgeMut = useMutation({
    mutationFn: (content: string) => api.put(`/api/groups/${chatID}/context/knowledge`, { content }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['groups', chatID, 'knowledge'] }); toast('База знаний сохранена', 'success') },
    onError: (err: any) => toast(`Ошибка: ${err.message}`, 'error'),
  })

  const tabs: { id: Tab; label: string; icon: typeof SettingsIcon }[] = [
    { id: 'settings', label: 'Настройки', icon: SettingsIcon },
    { id: 'prompt', label: 'Промпт', icon: FileText },
    { id: 'knowledge', label: 'База знаний', icon: Brain },
  ]

  return (
    <div className={`groups-detail-pane`} style={{ flex: 1, display: 'flex', flexDirection: 'column' }}>
      <div className="card" style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: '1rem' }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', borderBottom: '1px solid var(--glass-border)', paddingBottom: '0.75rem' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
            <button className="groups-back-btn" onClick={onBack}
              style={{ padding: '0.35rem', border: 'none', background: 'none', cursor: 'pointer', color: 'var(--text)' }}>
              <ArrowLeft size={20} />
            </button>
            <div>
              <h3 style={{ fontSize: '1.1rem', fontWeight: 600 }}>{group?.title}</h3>
              <span style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>Chat ID: {chatID}</span>
            </div>
          </div>
          <div style={{ display: 'flex', gap: '0.5rem' }}>
            {group?.is_vip && <span className="badge badge-active">VIP</span>}
            <span className={`badge ${group?.is_active ? 'badge-active' : 'badge-inactive'}`}>
              {group?.is_active ? 'Активна' : 'Неактивна'}
            </span>
          </div>
        </div>

        <div style={{ display: 'flex', gap: '0.25rem', borderBottom: '1px solid var(--glass-border)', paddingBottom: '0.25rem', overflowX: 'auto' }}>
          {tabs.map(tab => {
            const Icon = tab.icon
            const isActive = activeTab === tab.id
            return (
              <button key={tab.id} onClick={() => setActiveTab(tab.id)}
                style={{
                  display: 'flex', alignItems: 'center', gap: '0.35rem', padding: '0.5rem 0.75rem',
                  borderRadius: 'var(--radius-sm)', border: 'none',
                  background: isActive ? 'var(--accent-glass)' : 'transparent',
                  color: isActive ? 'var(--accent)' : 'var(--text)', fontSize: '0.85rem',
                  fontWeight: isActive ? 500 : 400, cursor: 'pointer', transition: 'var(--transition)', whiteSpace: 'nowrap',
                }}>
                <Icon size={14} /> {tab.label}
              </button>
            )
          })}
        </div>

        <div style={{ flex: 1, display: 'flex', flexDirection: 'column', marginTop: '0.5rem' }}>
          {activeTab === 'settings' && (
            <GroupSettings
              metaForm={metaForm}
              featuresForm={featuresForm}
              featuresLoading={featuresQ.isLoading}
              onMetaChange={(updates) => setMetaForm(prev => ({ ...prev, ...updates }))}
              onFeatureChange={(id, value) => setFeaturesForm(prev => ({ ...prev, [id]: value }))}
              onSave={() => {
                updateMut.mutate(metaForm)
                saveFeaturesMut.mutate(featuresForm)
              }}
              onDelete={() => { if (confirm('Вы уверены, что хотите удалить эту группу из бота?')) deleteMut.mutate() }}
              savePending={updateMut.isPending || saveFeaturesMut.isPending}
              deletePending={deleteMut.isPending}
            />
          )}

          {activeTab === 'prompt' && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem', flex: 1 }}>
              <label style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>Системный промпт группы (`system_prompt.txt`)</label>
              {promptQ.isLoading ? (
                <LoadingSpinner />
              ) : (
                <textarea className="input"
                  style={{ flex: 1, fontFamily: 'monospace', fontSize: '0.85rem', minHeight: '350px', whiteSpace: 'pre', overflowY: 'auto' }}
                  value={promptContent} onChange={(e) => setPromptContent(e.target.value)}
                />
              )}
              <button className="btn btn-primary"
                onClick={() => savePromptMut.mutate(promptContent)}
                disabled={savePromptMut.isPending}
                style={{ alignSelf: 'flex-start' }}>
                <Save size={16} /> {savePromptMut.isPending ? 'Сохранение...' : 'Сохранить промпт'}
              </button>
            </div>
          )}

          {activeTab === 'knowledge' && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem', flex: 1 }}>
              <label style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>База знаний группы (`knowledge_base.txt`)</label>
              {knowledgeQ.isLoading ? (
                <LoadingSpinner />
              ) : (
                <textarea className="input"
                  style={{ flex: 1, fontFamily: 'monospace', fontSize: '0.85rem', minHeight: '350px', whiteSpace: 'pre', overflowY: 'auto' }}
                  value={knowledgeContent} onChange={(e) => setKnowledgeContent(e.target.value)}
                />
              )}
              <button className="btn btn-primary"
                onClick={() => saveKnowledgeMut.mutate(knowledgeContent)}
                disabled={saveKnowledgeMut.isPending}
                style={{ alignSelf: 'flex-start' }}>
                <Save size={16} /> {saveKnowledgeMut.isPending ? 'Сохранение...' : 'Сохранить базу знаний'}
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
