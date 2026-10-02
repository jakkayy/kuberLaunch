import { NextRequest, NextResponse } from 'next/server'
import { SESSION_COOKIE, verifySession } from '@/app/lib/session-token'

// Optimistic auth check: every page and /api route requires a valid session
// cookie. The Go API verifies the same token again on each request.
export default async function proxy(req: NextRequest) {
  const user = await verifySession(req.cookies.get(SESSION_COOKIE)?.value)
  if (user) return NextResponse.next()

  if (req.nextUrl.pathname.startsWith('/api/')) {
    return Response.json({ error: 'authentication required' }, { status: 401 })
  }
  const login = new URL('/login', req.nextUrl)
  login.searchParams.set('next', req.nextUrl.pathname + req.nextUrl.search)
  return NextResponse.redirect(login)
}

export const config = {
  matcher: ['/((?!login|auth/|_next/static|_next/image|favicon.ico).*)'],
}
