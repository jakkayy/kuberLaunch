import { NextRequest } from 'next/server'
import { apiFetch, relayJSON } from '@/app/lib/upstream'

export async function GET(
  _req: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params
  return relayJSON(await apiFetch(`/api/v1/projects/${id}/members`))
}

export async function PUT(
  req: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params
  return relayJSON(await apiFetch(`/api/v1/projects/${id}/members`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: await req.text(),
  }))
}
