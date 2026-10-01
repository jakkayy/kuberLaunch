import 'server-only'
import { getSessionToken } from './session'

export const API_URL = process.env.API_URL ?? 'http://localhost:8080'

// apiFetch calls the Go API on behalf of the signed-in user by forwarding the
// session token as a bearer token.
export async function apiFetch(path: string, init: RequestInit = {}): Promise<Response> {
  const headers = new Headers(init.headers)
  const token = await getSessionToken()
  if (token) headers.set('Authorization', `Bearer ${token}`)
  return fetch(`${API_URL}${path}`, { cache: 'no-store', ...init, headers })
}

// relayJSON forwards an upstream JSON response (or a non-JSON error body) to the browser.
export async function relayJSON(upstream: Response): Promise<Response> {
  if (upstream.status === 204) return new Response(null, { status: 204 })
  const text = await upstream.text()
  try {
    return Response.json(JSON.parse(text), { status: upstream.status })
  } catch {
    return Response.json({ error: text || upstream.statusText }, { status: upstream.status })
  }
}

// relayStream forwards an upstream Server-Sent Events stream.
export function relayStream(upstream: Response): Response {
  if (!upstream.ok || !upstream.body) {
    return new Response(`data: ${JSON.stringify({ error: `upstream ${upstream.status}` })}\n\n`, {
      headers: { 'Content-Type': 'text/event-stream' },
    })
  }
  return new Response(upstream.body, {
    headers: {
      'Content-Type': 'text/event-stream',
      'Cache-Control': 'no-cache',
      Connection: 'keep-alive',
    },
  })
}
