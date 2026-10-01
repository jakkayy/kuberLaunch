import { NextRequest } from 'next/server'
import { apiFetch, relayJSON } from '@/app/lib/upstream'

export async function DELETE(
  _req: NextRequest,
  { params }: { params: Promise<{ team: string; login: string }> }
) {
  const { team, login } = await params
  return relayJSON(await apiFetch(
    `/api/v1/teams/${encodeURIComponent(team)}/members/${encodeURIComponent(login)}`,
    { method: 'DELETE' },
  ))
}
