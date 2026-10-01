import { NextRequest } from 'next/server'
import { apiFetch } from '@/app/lib/upstream'

export async function GET(
  _req: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params
  const upstream = await apiFetch(`/api/v1/projects/${id}/download`)

  if (!upstream.ok) {
    return new Response('Not found', { status: upstream.status === 401 ? 401 : 404 })
  }

  return new Response(upstream.body, {
    headers: {
      'Content-Type': 'application/zip',
      'Content-Disposition': `attachment; filename="${id}-templates.zip"`,
    },
  })
}
