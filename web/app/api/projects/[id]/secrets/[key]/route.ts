import { NextRequest } from 'next/server'
import { apiFetch, relayJSON } from '@/app/lib/upstream'

export async function DELETE(
  _req: NextRequest,
  { params }: { params: Promise<{ id: string; key: string }> }
) {
  const { id, key } = await params
  return relayJSON(
    await apiFetch(`/api/v1/projects/${id}/secrets/${encodeURIComponent(key)}`, { method: 'DELETE' })
  )
}
