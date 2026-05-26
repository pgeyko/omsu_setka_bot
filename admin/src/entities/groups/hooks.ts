import { useQuery } from '@tanstack/react-query'
import { api } from '../../shared/api/client'

export interface Group {
  chat_id: number
  title: string
  api_token: string
  omsu_group_id: number
  is_active: boolean
  is_vip: boolean
  created_at?: string
}

export function useGroups() {
  return useQuery<Group[]>({
    queryKey: ['groups'],
    queryFn: () => api.get<Group[]>('/api/groups'),
  })
}

export function useGroupFeatures(chatID: number | null) {
  return useQuery<{ features: Record<string, boolean> }>({
    queryKey: ['groups', chatID, 'features'],
    queryFn: () => api.get<{ features: Record<string, boolean> }>(`/api/groups/${chatID}/context/features`),
    enabled: chatID !== null,
  })
}

export function useGroupPrompt(chatID: number | null) {
  return useQuery<{ content: string }>({
    queryKey: ['groups', chatID, 'system-prompt'],
    queryFn: () => api.get<{ content: string }>(`/api/groups/${chatID}/context/system-prompt`),
    enabled: chatID !== null,
  })
}

export function useGroupKnowledge(chatID: number | null) {
  return useQuery<{ content: string }>({
    queryKey: ['groups', chatID, 'knowledge'],
    queryFn: () => api.get<{ content: string }>(`/api/groups/${chatID}/context/knowledge`),
    enabled: chatID !== null,
  })
}
