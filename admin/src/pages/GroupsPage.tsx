import { useState } from 'react'
import { Users } from 'lucide-react'
import GroupList from '../entities/groups/GroupList'
import GroupDetail from '../entities/groups/GroupDetail'
import CreateGroupModal from '../entities/groups/CreateGroupModal'
import { useGroups } from '../entities/groups/hooks'

export default function GroupsPage() {
  const [selectedGroupId, setSelectedGroupId] = useState<number | null>(null)
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false)
  const { data: groups, isLoading: groupsLoading } = useGroups()

  const activeGroup = groups?.find(g => g.chat_id === selectedGroupId)

  return (
    <div style={{ display: 'flex', gap: '1.5rem', minHeight: 'calc(100vh - 120px)' }}>
      <GroupList
        groups={groups}
        loading={groupsLoading}
        selectedId={selectedGroupId}
        onSelect={(id) => setSelectedGroupId(id)}
        onAdd={() => setIsCreateModalOpen(true)}
      />

      {selectedGroupId && activeGroup ? (
        <GroupDetail
          group={activeGroup}
          chatID={selectedGroupId}
          onBack={() => setSelectedGroupId(null)}
        />
      ) : (
        <div className={`groups-detail-pane ${!selectedGroupId ? 'hidden-mobile' : ''}`}
          style={{ flex: 1, display: 'flex', flexDirection: 'column' }}>
          <div className="card" style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <div className="empty-state">
              <Users size={48} />
              <h3 style={{ marginTop: '0.5rem', fontWeight: 600 }}>Выберите группу для управления</h3>
              <p style={{ marginTop: '0.25rem', fontSize: '0.875rem' }}>Или нажмите кнопку «Добавить группу» слева</p>
            </div>
          </div>
        </div>
      )}

      <CreateGroupModal
        open={isCreateModalOpen}
        onClose={() => setIsCreateModalOpen(false)}
        onCreated={(chatID) => setSelectedGroupId(chatID)}
      />
    </div>
  )
}
