import { NextRequest, NextResponse } from 'next/server'
import { deleteSession } from '@/app/lib/session'

// POST /auth/logout — POST only, so a cross-site link cannot sign the user out.
export async function POST(req: NextRequest) {
  await deleteSession()
  return NextResponse.redirect(new URL('/login', req.nextUrl), { status: 303 })
}
