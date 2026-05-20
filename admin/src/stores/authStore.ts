import { create } from 'zustand'

interface AuthState {
  token: string | null
  setToken: (token: string) => void
  clearToken: () => void
}

export const useAuthStore = create<AuthState>((set) => ({
  token: sessionStorage.getItem('jwt') || null,
  setToken: (token) => {
    sessionStorage.setItem('jwt', token)
    set({ token })
  },
  clearToken: () => {
    sessionStorage.removeItem('jwt')
    set({ token: null })
  },
}))
