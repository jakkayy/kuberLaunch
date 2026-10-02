'use client'

import { useEffect, useState } from 'react'
import { Promotion } from '@/app/lib/types'

const STATUS_COLOR: Record<string, string> = {
  pending_approval: 'bg-amber-50 text-amber-700',
  applied: 'bg-green-50 text-green-700',
  rejected: 'bg-zinc-100 text-zinc-500',
  failed: 'bg-red-50 text-red-600',
}

export default function Promotions({
  projectId,
  canDecide,
  reloadToken,
}: {
  projectId: string
  canDecide: boolean
  reloadToken: number
}) {
  const [promotions, setPromotions] = useState<Promotion[]>([])
  const [localReload, setLocalReload] = useState(0)
  const [error, setError] = useState('')

  useEffect(() => {
    let cancelled = false
    fetch(`/api/projects/${projectId}/promotions`)
      .then(res => (res.ok ? res.json() : null))
      .then(data => { if (!cancelled && data) setPromotions(data.promotions ?? []) })
    return () => { cancelled = true }
  }, [projectId, reloadToken, localReload])

  async function decide(p: Promotion, decision: 'approve' | 'reject') {
    setError('')
    const reason = decision === 'reject' ? prompt('Reason for rejecting?') ?? '' : ''
    const res = await fetch(`/api/projects/${projectId}/promotions/${p.id}/${decision}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ reason }),
    })
    const data = await res.json().catch(() => ({}))
    if (!res.ok) setError(data.error ?? `${decision} failed`)
    setLocalReload(t => t + 1)
  }

  if (promotions.length === 0) return null

  return (
    <div className="mb-5 border border-zinc-200 rounded-lg">
      <h2 className="text-sm font-semibold text-zinc-700 px-4 py-2 border-b border-zinc-200">Promotions</h2>
      <ul className="divide-y divide-zinc-100 text-sm">
        {promotions.map(p => (
          <li key={p.id} className="flex items-center gap-3 px-4 py-2">
            <span className={`text-xs px-2 py-0.5 rounded-full ${STATUS_COLOR[p.status] ?? ''}`}>{p.status.replace('_', ' ')}</span>
            <span className="font-mono text-xs">{p.from_env} → {p.to_env}</span>
            <span className="font-mono text-xs text-zinc-500">{p.image_tag}</span>
            <span className="text-xs text-zinc-400">
              by {p.requested_by_login}{p.decided_by_login && p.decided_by_login !== p.requested_by_login ? ` · ${p.status === 'rejected' ? 'rejected' : 'approved'} by ${p.decided_by_login}` : ''}
            </span>
            {p.message && <span className="text-xs text-red-500 truncate" title={p.message}>{p.message}</span>}
            {canDecide && p.status === 'pending_approval' && (
              <span className="ml-auto flex gap-2">
                <button onClick={() => decide(p, 'approve')} className="text-xs px-2.5 py-1 bg-green-600 text-white rounded hover:bg-green-700">Approve</button>
                <button onClick={() => decide(p, 'reject')} className="text-xs px-2.5 py-1 border border-zinc-300 rounded hover:bg-zinc-50">Reject</button>
              </span>
            )}
          </li>
        ))}
      </ul>
      {error && <p className="text-xs text-red-500 px-4 pb-2">{error}</p>}
    </div>
  )
}
