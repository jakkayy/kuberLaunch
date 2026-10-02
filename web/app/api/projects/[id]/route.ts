import { NextRequest } from 'next/server'
import { apiFetch, relayJSON } from '@/app/lib/upstream'

export async function DELETE(
  req: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params
  return relayJSON(await apiFetch(`/api/v1/projects/${id}`, {
    method: 'DELETE',
    headers: { 'Content-Type': 'application/json' },
    body: await req.text(),
  }))
}
