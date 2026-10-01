import { NextRequest } from 'next/server'
import { apiFetch, relayStream } from '@/app/lib/upstream'

export async function GET(
  req: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params
  return relayStream(await apiFetch(`/api/v1/projects/${id}/setup/stream`, { signal: req.signal }))
}
