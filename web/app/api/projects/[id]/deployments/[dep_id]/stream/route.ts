import { NextRequest } from 'next/server'

const API_URL = process.env.API_URL ?? 'http://localhost:8080'

export async function GET(
  req: NextRequest,
  { params }: { params: Promise<{ id: string; dep_id: string }> }
) {
  const { id, dep_id } = await params
  const upstream = await fetch(
    `${API_URL}/api/v1/projects/${id}/deployments/${dep_id}/stream`,
    { signal: req.signal }
  )
  return new Response(upstream.body, {
    headers: {
      'Content-Type': 'text/event-stream',
      'Cache-Control': 'no-cache',
      Connection: 'keep-alive',
    },
  })
}
