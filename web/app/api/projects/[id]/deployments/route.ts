import { NextRequest } from 'next/server'

const API_URL = process.env.API_URL ?? 'http://localhost:8080'

export async function POST(
  req: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params
  const body = await req.text()
  const upstream = await fetch(`${API_URL}/api/v1/projects/${id}/deployments`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body,
  })
  const data = await upstream.json()
  return Response.json(data, { status: upstream.status })
}

export async function GET(
  _req: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params
  const upstream = await fetch(`${API_URL}/api/v1/projects/${id}/deployments`, {
    cache: 'no-store',
  })
  const data = await upstream.json()
  return Response.json(data, { status: upstream.status })
}
