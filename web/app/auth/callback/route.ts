import { NextRequest, NextResponse } from 'next/server'
import { createSession, safeRedirectPath } from '@/app/lib/session'

const OAUTH_STATE_COOKIE = 'kl_oauth_state'

function fail(req: NextRequest, reason: string) {
  const url = new URL('/login', req.nextUrl)
  url.searchParams.set('error', reason)
  const res = NextResponse.redirect(url)
  res.cookies.delete({ name: OAUTH_STATE_COOKIE, path: '/auth' })
  return res
}

// GET /auth/callback — GitHub redirects here after the user authorizes.
export async function GET(req: NextRequest) {
  const code = req.nextUrl.searchParams.get('code')
  const state = req.nextUrl.searchParams.get('state')

  let saved: { state?: string; next?: string } = {}
  try {
    saved = JSON.parse(req.cookies.get(OAUTH_STATE_COOKIE)?.value ?? '{}')
  } catch {}
  if (!code || !state || state !== saved.state) return fail(req, 'invalid_state')

  const tokenRes = await fetch('https://github.com/login/oauth/access_token', {
    method: 'POST',
    headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
    body: JSON.stringify({
      client_id: process.env.GITHUB_OAUTH_CLIENT_ID,
      client_secret: process.env.GITHUB_OAUTH_CLIENT_SECRET,
      code,
      redirect_uri: new URL('/auth/callback', req.nextUrl).toString(),
    }),
  })
  const tokenData = await tokenRes.json().catch(() => ({}))
  const accessToken: string | undefined = tokenData.access_token
  if (!accessToken) return fail(req, 'token_exchange_failed')

  const gh = (path: string) =>
    fetch(`https://api.github.com${path}`, {
      headers: { Authorization: `Bearer ${accessToken}`, Accept: 'application/vnd.github+json' },
      cache: 'no-store',
    })

  const userRes = await gh('/user')
  if (!userRes.ok) return fail(req, 'user_lookup_failed')
  const user = await userRes.json()

  const org = process.env.AUTH_ALLOWED_ORG
  if (org) {
    const membership = await gh(`/user/memberships/orgs/${encodeURIComponent(org)}`)
    const data = membership.ok ? await membership.json() : null
    if (data?.state !== 'active') return fail(req, 'not_org_member')
  }

  await createSession({
    githubId: user.id,
    login: user.login,
    name: user.name ?? user.login,
    avatarUrl: user.avatar_url ?? '',
  })
  const res = NextResponse.redirect(new URL(safeRedirectPath(saved.next), req.nextUrl))
  res.cookies.delete({ name: OAUTH_STATE_COOKIE, path: '/auth' })
  return res
}
