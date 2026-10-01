import { NextRequest } from 'next/server'
import { apiFetch, relayJSON } from '@/app/lib/upstream'

const DECISIONS = new Set(['approve', 'reject'])

export async function POST(
  req: NextRequest,
  { params }: { params: Promise<{ id: string; promotion_id: string; decision: string }> }
) {
  const { id, promotion_id, decision } = await params
  if (!DECISIONS.has(decision)) return Response.json({ error: 'not found' }, { status: 404 })
  return relayJSON(await apiFetch(
    `/api/v1/projects/${id}/promotions/${encodeURIComponent(promotion_id)}/${decision}`,
    { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: await req.text() },
  ))
}
