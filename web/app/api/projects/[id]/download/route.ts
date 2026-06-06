import { NextRequest } from 'next/server'

const API_URL = process.env.API_URL ?? 'http://localhost:8080'

export async function GET(
  _req: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params
  const upstream = await fetch(`${API_URL}/api/v1/projects/${id}/download`)

  if (!upstream.ok) {
    return new Response('Not found', { status: 404 })
  }

  return new Response(upstream.body, {
    headers: {
      'Content-Type': 'application/zip',
      'Content-Disposition': `attachment; filename="${id}-templates.zip"`,
    },
  })
}
