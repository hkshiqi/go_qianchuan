import { describe, expect, it } from 'vitest'
import { isPublicAuthPath } from '../authRoute'

describe('isPublicAuthPath', () => {
  it('treats login and oauth callback as public auth routes', () => {
    expect(isPublicAuthPath('/login')).toBe(true)
    expect(isPublicAuthPath('/auth/callback')).toBe(true)
  })

  it('keeps protected routes behind the auth guard', () => {
    expect(isPublicAuthPath('/')).toBe(false)
    expect(isPublicAuthPath('/dashboard')).toBe(false)
    expect(isPublicAuthPath('/auth/callback-extra')).toBe(false)
  })
})
