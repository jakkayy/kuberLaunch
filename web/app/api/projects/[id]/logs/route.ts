import { NextRequest } from 'next/server'
import { apiFetch, relayStream } from '@/app/lib/upstream'

export async function GET(
  req: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params
  const env = req.nextUrl.searchParams.get('env') ?? 'dev'
  return relayStream(await apiFetch(`/api/v1/projects/${id}/logs?env=${encodeURIComponent(env)}`, { signal: req.signal }))
}
