import 'server-only'
import { cookies } from 'next/headers'
import { SESSION_COOKIE, SESSION_TTL_SECONDS, SessionUser, signSession, verifySession } from './session-token'

export async function createSession(user: SessionUser) {
  const token = await signSession(user)
  const store = await cookies()
  store.set(SESSION_COOKIE, token, {
    httpOnly: true,
    secure: process.env.NODE_ENV === 'production',
    sameSite: 'lax',
    path: '/',
    maxAge: SESSION_TTL_SECONDS,
  })
}

export async function deleteSession() {
  const store = await cookies()
  store.delete(SESSION_COOKIE)
}

export async function getSessionToken(): Promise<string | undefined> {
  return (await cookies()).get(SESSION_COOKIE)?.value
}

export async function getSessionUser(): Promise<SessionUser | null> {
  return verifySession(await getSessionToken())
}

// safeRedirectPath only allows same-origin relative paths as post-login targets.
export function safeRedirectPath(next: string | null | undefined): string {
  if (!next || !next.startsWith('/') || next.startsWith('//') || next.startsWith('/\\')) return '/'
  return next
}
