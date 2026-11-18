import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  OAUTH_STATE_TTL_MS,
  clearOAuthState,
  createOAuthState,
  readOAuthState,
} from '../oauthState'

describe('oauthState', () => {
  beforeEach(() => {
    clearOAuthState()
    vi.useRealTimers()
  })

  it('keeps the state available after the callback opens in another tab', () => {
    const state = createOAuthState()

    sessionStorage.clear()

    expect(readOAuthState()).toEqual({ state, expired: false })
  })

  it('marks an old authorization attempt as expired', () => {
    const now = Date.now()
    vi.useFakeTimers()
    vi.setSystemTime(now)
    createOAuthState()
    vi.setSystemTime(now + OAUTH_STATE_TTL_MS + 1)

    expect(readOAuthState().expired).toBe(true)
  })

  it('clears both storage locations', () => {
    createOAuthState()
    clearOAuthState()

    expect(readOAuthState()).toEqual({ state: null, expired: false })
  })
})
