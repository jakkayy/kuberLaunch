import { NextRequest } from 'next/server'
import { apiFetch, relayJSON } from '@/app/lib/upstream'

export async function POST(
  _req: NextRequest,
  { params }: { params: Promise<{ id: string; env: string }> }
) {
  const { id, env } = await params
  return relayJSON(await apiFetch(`/api/v1/projects/${id}/environments/${encodeURIComponent(env)}/promote`, { method: 'POST' }))
}
