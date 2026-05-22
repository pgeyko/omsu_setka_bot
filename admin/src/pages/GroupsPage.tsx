import { useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Plus, Trash2, Save, Users, ChevronRight, Settings, Brain, FileText, Bot, ArrowLeft } from 'lucide-react'
import { api } from '../api/client'
import Modal from '../components/Modal'
import { toast } from '../components/Toast'
import LoadingSpinner from '../components/LoadingSpinner'

interface Group {
  chat_id: number;
  title: string;
  api_token: string;
  omsu_group_id: number;
  is_active: boolean;
  is_vip: boolean;
  created_at?: string;
}

export default function GroupsPage() {
  const qc = useQueryClient()
  const [selectedGroupId, setSelectedGroupId] = useState<number | null>(null)
  const [activeTab, setActiveTab] = useState<'settings' | 'persona' | 'prompt' | 'knowledge'>('settings')
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false)

  const [newGroupForm, setNewGroupForm] = useState<Omit<Group, 'created_at'>>({
    chat_id: 0,
    title: '',
    api_token: '',
    omsu_group_id: 0,
    is_active: true,
    is_vip: false,
  })

  const [metaForm, setMetaForm] = useState<Omit<Group, 'created_at'>>({
    chat_id: 0,
    title: '',
    api_token: '',
    omsu_group_id: 0,
    is_active: false,
    is_vip: false,
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

  const [personaContent, setPersonaContent] = useState('')
  const [promptContent, setPromptContent] = useState('')
  const [knowledgeContent, setKnowledgeContent] = useState('')

  const { data: groups, isLoading: groupsLoading } = useQuery<Group[]>({
    queryKey: ['groups'],
    queryFn: () => api.get<Group[]>('/api/groups'),
  })

  const activeGroup = groups?.find(g => g.chat_id === selectedGroupId)

  const featuresQ = useQuery<{ features: Record<string, boolean> }>({
    queryKey: ['groups', selectedGroupId, 'features'],
    queryFn: () => api.get<{ features: Record<string, boolean> }>(`/api/groups/${selectedGroupId}/context/features`),
    enabled: selectedGroupId !== null,
  })

  const personaQ = useQuery<{ content: string }>({
    queryKey: ['groups', selectedGroupId, 'persona'],
    queryFn: () => api.get<{ content: string }>(`/api/groups/${selectedGroupId}/context/persona`),
    enabled: selectedGroupId !== null,
  })

  const promptQ = useQuery<{ content: string }>({
    queryKey: ['groups', selectedGroupId, 'system-prompt'],
    queryFn: () => api.get<{ content: string }>(`/api/groups/${selectedGroupId}/context/system-prompt`),
    enabled: selectedGroupId !== null,
  })

  const knowledgeQ = useQuery<{ content: string }>({
    queryKey: ['groups', selectedGroupId, 'knowledge'],
    queryFn: () => api.get<{ content: string }>(`/api/groups/${selectedGroupId}/context/knowledge`),
    enabled: selectedGroupId !== null,
  })

  useEffect(() => {
    if (activeGroup) {
      setMetaForm({
        chat_id: activeGroup.chat_id,
        title: activeGroup.title,
        api_token: activeGroup.api_token || '',
        omsu_group_id: activeGroup.omsu_group_id || 0,
        is_active: activeGroup.is_active,
        is_vip: activeGroup.is_vip,
      })
    }
  }, [activeGroup])

  useEffect(() => {
    if (featuresQ.data?.features) {
      setFeaturesForm(featuresQ.data.features)
    }
  }, [featuresQ.data])

  useEffect(() => {
    if (personaQ.data) {
      setPersonaContent(personaQ.data.content)
    }
  }, [personaQ.data])

  useEffect(() => {
    if (promptQ.data) {
      setPromptContent(promptQ.data.content)
    }
  }, [promptQ.data])

  useEffect(() => {
    if (knowledgeQ.data) {
      setKnowledgeContent(knowledgeQ.data.content)
    }
  }, [knowledgeQ.data])

  const createMut = useMutation({
    mutationFn: (newGroup: Omit<Group, 'created_at'>) => api.post<Group>('/api/groups', newGroup),
    onSuccess: (data) => {
      qc.invalidateQueries({ queryKey: ['groups'] })
      setIsCreateModalOpen(false)
      setSelectedGroupId(data.chat_id)
      setNewGroupForm({
        chat_id: 0,
        title: '',
        api_token: '',
        omsu_group_id: 0,
        is_active: true,
        is_vip: false,
      })
      toast('Группа добавлена', 'success')
    },
    onError: (err: any) => {
      toast(`Ошибка: ${err.message}`, 'error')
    }
  })

  const updateMut = useMutation({
    mutationFn: ({ chat_id, data }: { chat_id: number; data: Partial<Group> }) =>
      api.put<Group>(`/api/groups/${chat_id}`, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['groups'] })
      toast('Настройки обновлены', 'success')
    },
    onError: (err: any) => {
      toast(`Ошибка: ${err.message}`, 'error')
    }
  })

  const deleteMut = useMutation({
    mutationFn: (chat_id: number) => api.delete(`/api/groups/${chat_id}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['groups'] })
      setSelectedGroupId(null)
      toast('Группа удалена', 'success')
    },
    onError: (err: any) => {
      toast(`Ошибка: ${err.message}`, 'error')
    }
  })

  const saveFeaturesMut = useMutation({
    mutationFn: ({ chat_id, features }: { chat_id: number; features: Record<string, boolean> }) =>
      api.put(`/api/groups/${chat_id}/context/features`, features),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['groups', selectedGroupId, 'features'] })
      toast('Модули сохранены', 'success')
    },
    onError: (err: any) => {
      toast(`Ошибка: ${err.message}`, 'error')
    }
  })

  const savePersonaMut = useMutation({
    mutationFn: ({ chat_id, content }: { chat_id: number; content: string }) =>
      api.put(`/api/groups/${chat_id}/context/persona`, { content }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['groups', selectedGroupId, 'persona'] })
      toast('Файл личности сохранен', 'success')
    },
    onError: (err: any) => {
      toast(`Ошибка: ${err.message}`, 'error')
    }
  })

  const savePromptMut = useMutation({
    mutationFn: ({ chat_id, content }: { chat_id: number; content: string }) =>
      api.put(`/api/groups/${chat_id}/context/system-prompt`, { content }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['groups', selectedGroupId, 'system-prompt'] })
      toast('Системный промпт сохранен', 'success')
    },
    onError: (err: any) => {
      toast(`Ошибка: ${err.message}`, 'error')
    }
  })

  const saveKnowledgeMut = useMutation({
    mutationFn: ({ chat_id, content }: { chat_id: number; content: string }) =>
      api.put(`/api/groups/${chat_id}/context/knowledge`, { content }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['groups', selectedGroupId, 'knowledge'] })
      toast('База знаний сохранена', 'success')
    },
    onError: (err: any) => {
      toast(`Ошибка: ${err.message}`, 'error')
    }
  })

  return (
    <div style={{ display: 'flex', gap: '1.5rem', minHeight: 'calc(100vh - 120px)' }}>
      {/* Left Pane: Group List */}
      <div className={`groups-list-pane ${selectedGroupId ? 'hidden-mobile' : ''}`} style={{ width: '280px', display: 'flex', flexDirection: 'column', gap: '0.75rem', flexShrink: 0 }}>
        <button className="btn btn-primary" onClick={() => setIsCreateModalOpen(true)} style={{ justifyContent: 'center' }}>
          <Plus size={16} /> Добавить группу
        </button>
        <div className="card" style={{ flex: 1, padding: '0.75rem', overflowY: 'auto' }}>
          {groupsLoading ? (
            <LoadingSpinner />
          ) : groups && groups.length > 0 ? (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '0.35rem' }}>
              {groups.map(group => (
                <button
                  key={group.chat_id}
                  onClick={() => {
                    setSelectedGroupId(group.chat_id)
                    setActiveTab('settings')
                  }}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    width: '100%',
                    padding: '0.75rem',
                    borderRadius: 'var(--radius-sm)',
                    border: 'none',
                    background: 'transparent',
                    color: 'var(--text)',
                    textAlign: 'left',
                    transition: 'var(--transition)',
                    cursor: 'pointer',
                  }}
                  onMouseEnter={(e) => { e.currentTarget.style.background = 'var(--glass-bg)' }}
                  onMouseLeave={(e) => { e.currentTarget.style.background = 'transparent' }}
                >
                  <div style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', marginRight: '0.5rem' }}>
                    <div style={{ fontWeight: 500, fontSize: '0.875rem' }}>{group.title}</div>
                    <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>ID: {group.chat_id}</div>
                  </div>
                  <ChevronRight size={16} style={{ flexShrink: 0, opacity: 0.3 }} />
                </button>
              ))}
            </div>
          ) : (
            <div className="empty-state" style={{ padding: '2rem 1rem' }}>
              <Users size={32} />
              <div>Группы не найдены</div>
            </div>
          )}
        </div>
      </div>

      {/* Right Pane: Detailed Group Context Management */}
      <div className={`groups-detail-pane ${!selectedGroupId ? 'hidden-mobile' : ''}`} style={{ flex: 1, display: 'flex', flexDirection: 'column' }}>
        {selectedGroupId ? (
          <div className="card" style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: '1rem' }}>
            {/* Header with back button on mobile */}
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', borderBottom: '1px solid var(--glass-border)', paddingBottom: '0.75rem' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                <button className="groups-back-btn" onClick={() => setSelectedGroupId(null)} style={{ display: 'none', padding: '0.35rem', border: 'none', background: 'none', cursor: 'pointer', color: 'var(--text)' }}>
                  <ArrowLeft size={20} />
                </button>
                <div>
                  <h3 style={{ fontSize: '1.1rem', fontWeight: 600 }}>{activeGroup?.title}</h3>
                  <span style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>Chat ID: {selectedGroupId}</span>
                </div>
              </div>
              <div style={{ display: 'flex', gap: '0.5rem' }}>
                {activeGroup?.is_vip && <span className="badge badge-active">VIP</span>}
                <span className={`badge ${activeGroup?.is_active ? 'badge-active' : 'badge-inactive'}`}>
                  {activeGroup?.is_active ? 'Активна' : 'Неактивна'}
                </span>
              </div>
            </div>

            {/* Tabs */}
            <div style={{ display: 'flex', gap: '0.25rem', borderBottom: '1px solid var(--glass-border)', paddingBottom: '0.25rem', overflowX: 'auto' }}>
              {[
                { id: 'settings', label: 'Настройки', icon: Settings },
                { id: 'persona', label: 'Личность', icon: Bot },
                { id: 'prompt', label: 'Промпт', icon: FileText },
                { id: 'knowledge', label: 'База знаний', icon: Brain },
              ].map(tab => {
                const Icon = tab.icon;
                const isActive = activeTab === tab.id;
                return (
                  <button
                    key={tab.id}
                    onClick={() => setActiveTab(tab.id as any)}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: '0.35rem',
                      padding: '0.5rem 0.75rem',
                      borderRadius: 'var(--radius-sm)',
                      border: 'none',
                      background: isActive ? 'var(--accent-glass)' : 'transparent',
                      color: isActive ? 'var(--accent)' : 'var(--text)',
                      fontSize: '0.85rem',
                      fontWeight: isActive ? 500 : 400,
                      cursor: 'pointer',
                      transition: 'var(--transition)',
                      whiteSpace: 'nowrap',
                    }}
                  >
                    <Icon size={14} />
                    {tab.label}
                  </button>
                );
              })}
            </div>

            {/* Tab Contents */}
            <div style={{ flex: 1, display: 'flex', flexDirection: 'column', marginTop: '0.5rem' }}>
              {activeTab === 'settings' && (
                <div style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}>
                  <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem' }}>
                    <div>
                      <label style={{ display: 'block', marginBottom: '0.35rem', fontSize: '0.8rem', color: 'var(--text-muted)' }}>Название группы</label>
                      <input className="input" value={metaForm.title} onChange={(e) => setMetaForm({ ...metaForm, title: e.target.value })} />
                    </div>
                    <div>
                      <label style={{ display: 'block', marginBottom: '0.35rem', fontSize: '0.8rem', color: 'var(--text-muted)' }}>Omsu Group ID (Setka)</label>
                      <input className="input" type="number" value={metaForm.omsu_group_id || ''} onChange={(e) => setMetaForm({ ...metaForm, omsu_group_id: Number(e.target.value) })} />
                    </div>
                  </div>

                  <div style={{ display: 'flex', gap: '2rem', borderTop: '1px solid var(--glass-border)', borderBottom: '1px solid var(--glass-border)', padding: '1rem 0' }}>
                    <label style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', cursor: 'pointer' }}>
                      <input type="checkbox" checked={metaForm.is_active} onChange={(e) => setMetaForm({ ...metaForm, is_active: e.target.checked })} />
                      <span style={{ fontSize: '0.9rem' }}>Активна (бот обрабатывает сообщения)</span>
                    </label>
                    <label style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', cursor: 'pointer' }}>
                      <input type="checkbox" checked={metaForm.is_vip} onChange={(e) => setMetaForm({ ...metaForm, is_vip: e.target.checked })} />
                      <span style={{ fontSize: '0.9rem' }}>VIP статус</span>
                    </label>
                  </div>

                  <div>
                    <h4 style={{ fontSize: '0.9rem', fontWeight: 600, marginBottom: '0.75rem' }}>Основные модули</h4>
                    {featuresQ.isLoading ? (
                      <LoadingSpinner />
                    ) : (
                      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.75rem', marginBottom: '1.25rem' }}>
                        {[
                          { id: 'enable_schedule', label: 'Расписание занятий' },
                          { id: 'enable_summary', label: 'Суммаризация топиков' },
                          { id: 'enable_voice_transcription', label: 'Расшифровка аудиосообщений' },
                        ].map(f => (
                          <label key={f.id} style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', cursor: 'pointer', padding: '0.5rem 0.75rem', background: 'var(--glass-bg)', borderRadius: 'var(--radius-sm)', border: '1px solid var(--glass-border)' }}>
                            <input
                              type="checkbox"
                              checked={!!featuresForm[f.id]}
                              onChange={(e) => setFeaturesForm({ ...featuresForm, [f.id]: e.target.checked })}
                            />
                            <span style={{ fontSize: '0.85rem' }}>{f.label}</span>
                          </label>
                        ))}
                        {/* Photo: three-state toggle */}
                        <div
                          onClick={() => {
                            const auto = featuresForm.enable_photo_processing
                            const mention = featuresForm.photo_on_mention
                            if (!auto && !mention) {
                              setFeaturesForm({ ...featuresForm, enable_photo_processing: true, photo_on_mention: false })
                            } else if (auto && !mention) {
                              setFeaturesForm({ ...featuresForm, enable_photo_processing: true, photo_on_mention: true })
                            } else {
                              setFeaturesForm({ ...featuresForm, enable_photo_processing: false, photo_on_mention: false })
                            }
                          }}
                          style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', cursor: 'pointer', padding: '0.5rem 0.75rem', background: 'var(--glass-bg)', borderRadius: 'var(--radius-sm)', border: '1px solid var(--glass-border)', userSelect: 'none' }}
                        >
                          <span style={{ fontSize: '0.85rem' }}>
                            Обработка фото:{' '}
                            {!featuresForm.enable_photo_processing && !featuresForm.photo_on_mention ? '❌' :
                             featuresForm.enable_photo_processing && !featuresForm.photo_on_mention ? '✅' :
                             '✅ @'}
                          </span>
                        </div>
                      </div>
                    )}

                    <h4 style={{ fontSize: '0.9rem', fontWeight: 600, marginBottom: '0.75rem' }}>Локальная модерация</h4>
                    {featuresQ.isLoading ? (
                      <LoadingSpinner />
                    ) : (
                      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.75rem' }}>
                        {[
                          { id: 'enable_moderation', label: 'Общая модерация (активировать защиту)' },
                          { id: 'enable_captcha', label: 'Математическая капча' },
                          { id: 'enable_link_filter', label: 'Фильтр ссылок для новых пользователей' },
                          { id: 'enable_flood_control', label: 'Флуд-контроль (ограничение частоты)' },
                        ].map(f => (
                          <label key={f.id} style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', cursor: 'pointer', padding: '0.5rem 0.75rem', background: 'var(--glass-bg)', borderRadius: 'var(--radius-sm)', border: '1px solid var(--glass-border)' }}>
                            <input
                              type="checkbox"
                              checked={!!featuresForm[f.id]}
                              onChange={(e) => setFeaturesForm({ ...featuresForm, [f.id]: e.target.checked })}
                            />
                            <span style={{ fontSize: '0.85rem' }}>{f.label}</span>
                          </label>
                        ))}
                      </div>
                    )}
                  </div>

                  <div style={{ display: 'flex', gap: '0.5rem', marginTop: '1rem', flexWrap: 'wrap' }}>
                    <button
                      className="btn btn-primary"
                      onClick={() => {
                        updateMut.mutate({ chat_id: selectedGroupId, data: metaForm })
                        saveFeaturesMut.mutate({ chat_id: selectedGroupId, features: featuresForm })
                      }}
                      disabled={updateMut.isPending || saveFeaturesMut.isPending}
                    >
                      <Save size={16} /> Сохранить настройки
                    </button>
                    <button
                      className="btn btn-danger"
                      onClick={() => {
                        if (confirm('Вы уверены, что хотите удалить эту группу из бота?')) {
                          deleteMut.mutate(selectedGroupId)
                        }
                      }}
                      disabled={deleteMut.isPending}
                      style={{ marginLeft: 'auto' }}
                    >
                      <Trash2 size={16} /> Удалить группу
                    </button>
                  </div>
                </div>
              )}

              {activeTab === 'persona' && (
                <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem', flex: 1 }}>
                  <label style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>Конфигурация личности группы (`persona.md` в формате Markdown)</label>
                  {personaQ.isLoading ? (
                    <LoadingSpinner />
                  ) : (
                    <textarea
                      className="input"
                      style={{ flex: 1, fontFamily: 'monospace', fontSize: '0.85rem', minHeight: '350px', whiteSpace: 'pre', overflowY: 'auto' }}
                      value={personaContent}
                      onChange={(e) => setPersonaContent(e.target.value)}
                    />
                  )}
                  <button
                    className="btn btn-primary"
                    onClick={() => savePersonaMut.mutate({ chat_id: selectedGroupId, content: personaContent })}
                    disabled={savePersonaMut.isPending}
                    style={{ alignSelf: 'flex-start' }}
                  >
                    <Save size={16} /> {savePersonaMut.isPending ? 'Сохранение...' : 'Сохранить личность'}
                  </button>
                </div>
              )}

              {activeTab === 'prompt' && (
                <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem', flex: 1 }}>
                  <label style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>Системный промпт группы (`system_prompt.txt` в текстовом формате)</label>
                  {promptQ.isLoading ? (
                    <LoadingSpinner />
                  ) : (
                    <textarea
                      className="input"
                      style={{ flex: 1, fontFamily: 'monospace', fontSize: '0.85rem', minHeight: '350px', whiteSpace: 'pre', overflowY: 'auto' }}
                      value={promptContent}
                      onChange={(e) => setPromptContent(e.target.value)}
                    />
                  )}
                  <button
                    className="btn btn-primary"
                    onClick={() => savePromptMut.mutate({ chat_id: selectedGroupId, content: promptContent })}
                    disabled={savePromptMut.isPending}
                    style={{ alignSelf: 'flex-start' }}
                  >
                    <Save size={16} /> {savePromptMut.isPending ? 'Сохранение...' : 'Сохранить промпт'}
                  </button>
                </div>
              )}

              {activeTab === 'knowledge' && (
                <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem', flex: 1 }}>
                  <label style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>Локальная база знаний группы (`knowledge_base.txt` в текстовом формате)</label>
                  {knowledgeQ.isLoading ? (
                    <LoadingSpinner />
                  ) : (
                    <textarea
                      className="input"
                      style={{ flex: 1, fontFamily: 'monospace', fontSize: '0.85rem', minHeight: '350px', whiteSpace: 'pre', overflowY: 'auto' }}
                      value={knowledgeContent}
                      onChange={(e) => setKnowledgeContent(e.target.value)}
                    />
                  )}
                  <button
                    className="btn btn-primary"
                    onClick={() => saveKnowledgeMut.mutate({ chat_id: selectedGroupId, content: knowledgeContent })}
                    disabled={saveKnowledgeMut.isPending}
                    style={{ alignSelf: 'flex-start' }}
                  >
                    <Save size={16} /> {saveKnowledgeMut.isPending ? 'Сохранение...' : 'Сохранить базу знаний'}
                  </button>
                </div>
              )}
            </div>
          </div>
        ) : (
          <div className="card" style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <div className="empty-state">
              <Users size={48} />
              <h3 style={{ marginTop: '0.5rem', fontWeight: 600 }}>Выберите группу для управления</h3>
              <p style={{ marginTop: '0.25rem', fontSize: '0.875rem' }}>Или нажмите кнопку «Добавить группу» слева</p>
            </div>
          </div>
        )}
      </div>

      {/* Create Modal */}
      <Modal open={isCreateModalOpen} onClose={() => setIsCreateModalOpen(false)} title="Добавить группу">
        <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
          <div>
            <label style={{ display: 'block', marginBottom: '0.35rem', fontSize: '0.85rem', color: 'var(--text-muted)' }}>Telegram Chat ID</label>
            <input
              className="input"
              type="number"
              value={newGroupForm.chat_id || ''}
              onChange={(e) => setNewGroupForm({ ...newGroupForm, chat_id: Number(e.target.value) })}
              placeholder="Например, -100123456789"
            />
          </div>
          <div>
            <label style={{ display: 'block', marginBottom: '0.35rem', fontSize: '0.85rem', color: 'var(--text-muted)' }}>Название группы</label>
            <input
              className="input"
              value={newGroupForm.title}
              onChange={(e) => setNewGroupForm({ ...newGroupForm, title: e.target.value })}
              placeholder="Например, ИТБ-101"
            />
          </div>
          <div>
            <label style={{ display: 'block', marginBottom: '0.35rem', fontSize: '0.85rem', color: 'var(--text-muted)' }}>Omsu Group ID (Setka)</label>
            <input
              className="input"
              type="number"
              value={newGroupForm.omsu_group_id || ''}
              onChange={(e) => setNewGroupForm({ ...newGroupForm, omsu_group_id: Number(e.target.value) })}
            />
          </div>
          <div style={{ display: 'flex', gap: '1rem', marginTop: '0.5rem' }}>
            <label style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', cursor: 'pointer' }}>
              <input
                type="checkbox"
                checked={newGroupForm.is_active}
                onChange={(e) => setNewGroupForm({ ...newGroupForm, is_active: e.target.checked })}
              />
              <span style={{ fontSize: '0.85rem' }}>Активна</span>
            </label>
            <label style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', cursor: 'pointer' }}>
              <input
                type="checkbox"
                checked={newGroupForm.is_vip}
                onChange={(e) => setNewGroupForm({ ...newGroupForm, is_vip: e.target.checked })}
              />
              <span style={{ fontSize: '0.85rem' }}>VIP</span>
            </label>
          </div>

          <button
            className="btn btn-primary"
            onClick={() => {
              if (!newGroupForm.chat_id || !newGroupForm.title) {
                toast('Chat ID и Название обязательны', 'error')
                return
              }
              createMut.mutate(newGroupForm)
            }}
            disabled={createMut.isPending}
            style={{ marginTop: '0.5rem', justifyContent: 'center' }}
          >
            {createMut.isPending ? 'Создание...' : 'Добавить'}
          </button>
        </div>
      </Modal>
    </div>
  )
}
