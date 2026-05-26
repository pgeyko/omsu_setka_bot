import { useAuthStore } from '../../stores/authStore'

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

export interface PaginationMeta {
  total: number
  limit: number
  offset: number
}

export interface ApiResponse<T> {
  success: boolean
  data: T
  meta?: PaginationMeta
  error?: { code: string; message: string }
}

async function request<T>(path: string, options?: RequestInit): Promise<ApiResponse<T>> {
  const token = useAuthStore.getState().token
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options?.headers as Record<string, string>),
  }
  if (token) headers['Authorization'] = `Bearer ${token}`

  const res = await fetch(path, { ...options, headers })
  const json: ApiResponse<T> = await res.json()

  if (!json.success) {
    const err = new ApiError(json.error?.code || 'UNKNOWN', json.error?.message || 'Unknown error', res.status)
    if (onError) onError(err)
    throw err
  }
  return json
}

export const api = {
  get: <T>(path: string) => request<T>(path).then(r => r.data),
  getPaginated: <T>(path: string, params?: { limit?: number; offset?: number }): Promise<{ data: T[]; meta: PaginationMeta }> => {
    const qs = new URLSearchParams()
    if (params?.limit !== undefined) qs.set('limit', String(params.limit))
    if (params?.offset !== undefined) qs.set('offset', String(params.offset))
    const q = qs.toString()
    const sep = path.includes('?') ? '&' : '?'
    const fullPath = path + (q ? sep + q : '')
    return request<T[]>(fullPath).then(r => ({ data: r.data, meta: r.meta! }))
  },
  post: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: 'POST', body: body ? JSON.stringify(body) : undefined }).then(r => r.data),
  put: <T>(path: string, body: unknown) =>
    request<T>(path, { method: 'PUT', body: JSON.stringify(body) }).then(r => r.data),
  delete: <T>(path: string) => request<T>(path, { method: 'DELETE' }).then(r => r.data),
}
