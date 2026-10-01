import { NextRequest } from 'next/server'
import { apiFetch, relayStream } from '@/app/lib/upstream'

export async function GET(
  req: NextRequest,
  { params }: { params: Promise<{ id: string; dep_id: string }> }
) {
  const { id, dep_id } = await params
  return relayStream(await apiFetch(`/api/v1/projects/${id}/deployments/${dep_id}/stream`, { signal: req.signal }))
}
