import { ChevronRight, Users, Plus } from 'lucide-react'
import { Group } from './hooks'
import LoadingSpinner from '../../shared/ui/LoadingSpinner'

interface GroupListProps {
  groups: Group[] | undefined
  loading: boolean
  selectedId: number | null
  onSelect: (id: number) => void
  onAdd: () => void
}

export default function GroupList({ groups, loading, selectedId, onSelect, onAdd }: GroupListProps) {
  return (
    <div className={`groups-list-pane ${selectedId ? 'hidden-mobile' : ''}`}
      style={{ width: '280px', display: 'flex', flexDirection: 'column', gap: '0.75rem', flexShrink: 0 }}>
      <button className="btn btn-primary" onClick={onAdd} style={{ justifyContent: 'center' }}>
        <Plus size={16} /> Добавить группу
      </button>
      <div className="card" style={{ flex: 1, padding: '0.75rem', overflowY: 'auto' }}>
        {loading ? (
          <LoadingSpinner />
        ) : groups && groups.length > 0 ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '0.35rem' }}>
            {groups.map(group => (
              <button
                key={group.chat_id}
                onClick={() => onSelect(group.chat_id)}
                style={{
                  display: 'flex', alignItems: 'center', justifyContent: 'space-between',
                  width: '100%', padding: '0.75rem', borderRadius: 'var(--radius-sm)',
                  border: 'none', background: 'transparent', color: 'var(--text)',
                  textAlign: 'left', transition: 'var(--transition)', cursor: 'pointer',
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
  )
}
