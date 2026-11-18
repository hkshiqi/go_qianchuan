import { create } from 'zustand'
import { getAccessStatus, loginAccess, logoutAccess } from '@/api/access'

interface AccessState {
  authenticated: boolean
  checked: boolean
  loading: boolean
  checkStatus: () => Promise<void>
  login: (username: string, password: string) => Promise<void>
  logout: () => Promise<void>
}

export const useAccessStore = create<AccessState>((set) => ({
  authenticated: false,
  checked: false,
  loading: false,

  checkStatus: async () => {
    set({ loading: true })
    try {
      const status = await getAccessStatus()
      set({ authenticated: status.authenticated, checked: true, loading: false })
    } catch {
      set({ authenticated: false, checked: true, loading: false })
    }
  },

  login: async (username, password) => {
    await loginAccess(username, password)
    set({ authenticated: true, checked: true })
  },

  logout: async () => {
    await logoutAccess()
    set({ authenticated: false, checked: true })
  },
}))
