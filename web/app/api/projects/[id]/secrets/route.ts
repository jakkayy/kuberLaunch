import { NextRequest } from 'next/server'
import { apiFetch, relayJSON } from '@/app/lib/upstream'

export async function GET(
  _req: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params
  return relayJSON(await apiFetch(`/api/v1/projects/${id}/secrets`))
}

export async function POST(
  req: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params
  const body = await req.text()
  return relayJSON(await apiFetch(`/api/v1/projects/${id}/secrets`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body,
  }))
}
