import { useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Save, RotateCcw } from 'lucide-react'
import { api } from '../api/client'
import { toast } from '../components/Toast'
import LoadingSpinner from '../components/LoadingSpinner'

interface Persona { name: string; system_prompt: string; signature: string }

export default function PersonaPage() {
  const qc = useQueryClient()
  const [form, setForm] = useState<Persona | null>(null)

  const { data, isLoading } = useQuery({
    queryKey: ['persona'],
    queryFn: () => api.get<Persona>('/api/persona'),
  })

  useEffect(() => {
    if (data && form === null) setForm(data)
  }, [data, form])

  const mutation = useMutation({
    mutationFn: (body: Partial<Persona>) => api.put('/api/persona', body),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['persona'] }); toast('Личность обновлена', 'success') },
  })

  const resetMutation = useMutation({
    mutationFn: () => api.post('/api/persona/reset'),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['persona'] }); toast('Сброшено к умолчанию', 'success') },
  })

  if (isLoading) return <LoadingSpinner />

  const persona = form || data || { name: '', system_prompt: '', signature: '' }

  return (
    <div>
      <h2 style={{ marginBottom: '1.5rem', fontSize: '1.25rem' }}>Личность бота</h2>
      <div className="card" style={{ maxWidth: 640 }}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
          <div>
            <label style={{ display: 'block', marginBottom: '0.35rem', fontSize: '0.85rem', color: 'var(--text-muted)' }}>Имя</label>
            <input className="input" value={persona.name} onChange={(e) => setForm({ ...persona, name: e.target.value })} maxLength={128} />
          </div>
          <div>
            <label style={{ display: 'block', marginBottom: '0.35rem', fontSize: '0.85rem', color: 'var(--text-muted)' }}>Подпись (необязательно)</label>
            <input className="input" value={persona.signature} onChange={(e) => setForm({ ...persona, signature: e.target.value })} />
          </div>
          <div>
            <label style={{ display: 'block', marginBottom: '0.35rem', fontSize: '0.85rem', color: 'var(--text-muted)' }}>Системный промпт</label>
            <textarea className="input" rows={10} value={persona.system_prompt}
              onChange={(e) => setForm({ ...persona, system_prompt: e.target.value })} />
          </div>
          <div style={{ display: 'flex', gap: '0.5rem' }}>
            <button className="btn btn-primary" onClick={() => mutation.mutate({ name: persona.name, system_prompt: persona.system_prompt, signature: persona.signature })} disabled={mutation.isPending}>
              <Save size={16} /> {mutation.isPending ? 'Сохранение...' : 'Сохранить'}
            </button>
            <button className="btn" onClick={() => resetMutation.mutate()} disabled={resetMutation.isPending}>
              <RotateCcw size={16} /> Сбросить
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
