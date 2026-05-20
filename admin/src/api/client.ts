import { useAuthStore } from '../stores/authStore'

export class ApiError extends Error {
  code: string
  status: number
  constructor(code: string, message: string, status: number) {
    super(message)
    this.code = code
    this.status = status
    this.name = 'ApiError'
  }
}

let onError: ((err: ApiError) => void) | null = null
export function setGlobalErrorHandler(handler: (err: ApiError) => void) {
  onError = handler
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const token = useAuthStore.getState().token
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options?.headers as Record<string, string>),
  }
  if (token) headers['Authorization'] = `Bearer ${token}`

  const res = await fetch(path, { ...options, headers })
  const json = await res.json()

  if (!json.success) {
    const err = new ApiError(json.error?.code || 'UNKNOWN', json.error?.message || 'Unknown error', res.status)
    if (onError) onError(err)
    throw err
  }
  return json.data as T
}

export interface PaginationMeta {
  total: number
  limit: number
  offset: number
}

export interface PaginatedResponse<T> {
  data: T[]
  meta: PaginationMeta
}

export const api = {
  get: <T>(path: string) => request<T>(path),
  getPaginated: <T>(path: string, params?: { limit?: number; offset?: number }): Promise<PaginatedResponse<T>> => {
    const qs = new URLSearchParams()
    if (params?.limit !== undefined) qs.set('limit', String(params.limit))
    if (params?.offset !== undefined) qs.set('offset', String(params.offset))
    const q = qs.toString()
    const fullPath = path + (q ? '?' + q : '')
    return requestFull<PaginatedResponse<T>>(fullPath)
  },
  post: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: 'POST', body: body ? JSON.stringify(body) : undefined }),
  put: <T>(path: string, body: unknown) =>
    request<T>(path, { method: 'PUT', body: JSON.stringify(body) }),
  delete: <T>(path: string) => request<T>(path, { method: 'DELETE' }),
}

async function requestFull<T>(path: string, options?: RequestInit): Promise<T> {
  const token = useAuthStore.getState().token
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options?.headers as Record<string, string>),
  }
  if (token) headers['Authorization'] = `Bearer ${token}`

  const res = await fetch(path, { ...options, headers })
  const json = await res.json()

  if (!json.success) {
    const err = new ApiError(json.error?.code || 'UNKNOWN', json.error?.message || 'Unknown error', res.status)
    if (onError) onError(err)
    throw err
  }
  return json as T
}
