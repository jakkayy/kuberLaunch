import { NextRequest, NextResponse } from 'next/server'
import { randomBytes } from 'node:crypto'
import { createSession, safeRedirectPath } from '@/app/lib/session'

const OAUTH_STATE_COOKIE = 'kl_oauth_state'

// GET /auth/login?next=/projects/x — starts GitHub OAuth.
export async function GET(req: NextRequest) {
  const next = safeRedirectPath(req.nextUrl.searchParams.get('next'))

  // Local development without a GitHub OAuth app: sign in as AUTH_DEV_LOGIN.
  const devLogin = process.env.AUTH_DEV_LOGIN
  if (devLogin && process.env.NODE_ENV !== 'production') {
    await createSession({ githubId: 1, login: devLogin, name: devLogin, avatarUrl: '' })
    return NextResponse.redirect(new URL(next, req.nextUrl))
  }

  const clientId = process.env.GITHUB_OAUTH_CLIENT_ID
  if (!clientId) {
    return new Response('GITHUB_OAUTH_CLIENT_ID is not configured', { status: 500 })
  }

  const state = randomBytes(16).toString('hex')
  const authorize = new URL('https://github.com/login/oauth/authorize')
  authorize.searchParams.set('client_id', clientId)
  authorize.searchParams.set('redirect_uri', new URL('/auth/callback', req.nextUrl).toString())
  authorize.searchParams.set('scope', process.env.AUTH_ALLOWED_ORG ? 'read:user read:org' : 'read:user')
  authorize.searchParams.set('state', state)

  const res = NextResponse.redirect(authorize)
  res.cookies.set(OAUTH_STATE_COOKIE, JSON.stringify({ state, next }), {
    httpOnly: true,
    secure: process.env.NODE_ENV === 'production',
    sameSite: 'lax',
    path: '/auth',
    maxAge: 600,
  })
  return res
}
