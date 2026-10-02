import { NextRequest } from 'next/server'
import { apiFetch, relayJSON } from '@/app/lib/upstream'

export async function DELETE(
  _req: NextRequest,
  { params }: { params: Promise<{ id: string; member_id: string }> }
) {
  const { id, member_id } = await params
  return relayJSON(await apiFetch(`/api/v1/projects/${id}/members/${encodeURIComponent(member_id)}`, { method: 'DELETE' }))
}
