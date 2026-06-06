import { NextRequest } from 'next/server'

const API_URL = process.env.API_URL ?? 'http://localhost:8080'

export async function POST(
  _req: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params
  const upstream = await fetch(`${API_URL}/api/v1/projects/${id}/repo`, {
    method: 'POST',
  })
  const data = await upstream.json()
  return Response.json(data, { status: upstream.status })
}
