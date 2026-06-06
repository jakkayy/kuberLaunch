import { NextRequest } from 'next/server'

const API_URL = process.env.API_URL ?? 'http://localhost:8080'

export async function DELETE(
  _req: NextRequest,
  { params }: { params: Promise<{ id: string; key: string }> }
) {
  const { id, key } = await params
  const upstream = await fetch(
    `${API_URL}/api/v1/projects/${id}/secrets/${encodeURIComponent(key)}`,
    { method: 'DELETE' }
  )
  const data = await upstream.json()
  return Response.json(data, { status: upstream.status })
}
