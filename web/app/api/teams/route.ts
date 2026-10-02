import { NextRequest } from 'next/server'
import { apiFetch, relayJSON } from '@/app/lib/upstream'

export async function GET() {
  return relayJSON(await apiFetch('/api/v1/teams'))
}

export async function POST(req: NextRequest) {
  return relayJSON(await apiFetch('/api/v1/teams', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: await req.text(),
  }))
}
