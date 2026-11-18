import apiClient from './client'

export interface AccessStatus {
  authenticated: boolean
}

export const getAccessStatus = (): Promise<AccessStatus> => apiClient.get('/access/status')

export const loginAccess = (username: string, password: string): Promise<{ success: boolean }> =>
  apiClient.post('/access/login', { username, password })

export const logoutAccess = (): Promise<{ success: boolean }> => apiClient.post('/access/logout')
