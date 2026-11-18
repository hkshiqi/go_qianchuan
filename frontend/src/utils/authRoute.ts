export function isPublicAuthPath(pathname: string): boolean {
  return pathname === '/login' || pathname === '/auth/callback'
}
