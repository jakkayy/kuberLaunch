import { NextRequest } from 'next/server'

const API_URL = process.env.API_URL ?? 'http://localhost:8080'

export async function GET(
  _req: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params
  try {
    const upstream = await fetch(`${API_URL}/api/v1/projects/${id}/argocd/status`, { cache: 'no-store' })
    const text = await upstream.text()
    const data = JSON.parse(text)
    return Response.json(data, { status: upstream.status })
  } catch {
    return Response.json({ health: 'Unknown', sync: 'Unknown' }, { status: 200 })
  }
}
