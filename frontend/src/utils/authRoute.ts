export function isPublicAuthPath(pathname: string): boolean {
  return pathname === '/access' || pathname === '/login' || pathname === '/auth/callback'
}
