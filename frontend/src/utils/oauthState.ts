const OAUTH_STATE_KEY = 'oauth_state'
const OAUTH_STATE_CREATED_AT_KEY = 'oauth_state_created_at'

export const OAUTH_STATE_TTL_MS = 10 * 60 * 1000

function generateState(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.getRandomValues === 'function') {
    const bytes = new Uint8Array(16)
    crypto.getRandomValues(bytes)
    return Array.from(bytes, (value) => value.toString(16).padStart(2, '0')).join('')
  }
  return `${Date.now().toString(36)}${Math.random().toString(36).slice(2)}`
}

export function createOAuthState(): string {
  const state = generateState()
  const createdAt = Date.now().toString()

  // OAuth may complete in a newly opened tab. localStorage keeps the nonce
  // available across tabs; sessionStorage remains as a compatibility fallback.
  localStorage.setItem(OAUTH_STATE_KEY, state)
  localStorage.setItem(OAUTH_STATE_CREATED_AT_KEY, createdAt)
  sessionStorage.setItem(OAUTH_STATE_KEY, state)
  sessionStorage.setItem(OAUTH_STATE_CREATED_AT_KEY, createdAt)

  return state
}

export function readOAuthState(): { state: string | null; expired: boolean } {
  const state = localStorage.getItem(OAUTH_STATE_KEY) ?? sessionStorage.getItem(OAUTH_STATE_KEY)
  const createdAtValue =
    localStorage.getItem(OAUTH_STATE_CREATED_AT_KEY) ??
    sessionStorage.getItem(OAUTH_STATE_CREATED_AT_KEY)
  const createdAt = createdAtValue ? Number(createdAtValue) : 0

  return {
    state,
    expired: createdAt > 0 && Date.now() - createdAt > OAUTH_STATE_TTL_MS,
  }
}

export function clearOAuthState(): void {
  localStorage.removeItem(OAUTH_STATE_KEY)
  localStorage.removeItem(OAUTH_STATE_CREATED_AT_KEY)
  sessionStorage.removeItem(OAUTH_STATE_KEY)
  sessionStorage.removeItem(OAUTH_STATE_CREATED_AT_KEY)
}
