import { NextRequest } from 'next/server'
import { apiFetch, relayJSON } from '@/app/lib/upstream'

export async function POST(
  _req: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params
  return relayJSON(await apiFetch(`/api/v1/projects/${id}/monitoring`, { method: 'POST' }))
}
