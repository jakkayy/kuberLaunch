'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'

interface Upgrade {
  from: number
  to: number
  url?: string
  state: 'up_to_date' | 'open' | 'merged'
}

export default function TemplateUpgrade({
  projectId,
  current,
  latest,
}: {
  projectId: string
  current: number
  latest: number
}) {
  const [upgrade, setUpgrade] = useState<Upgrade | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const router = useRouter()

  if (current >= latest) return null

  async function run() {
    setBusy(true)
    setError('')
    try {
      const res = await fetch(`/api/projects/${projectId}/templates/upgrade`, { method: 'POST' })
      const data = await res.json()
      if (!res.ok) throw new Error(data.error ?? 'Upgrade failed')
      setUpgrade(data)
      if (data.state !== 'open') router.refresh()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Unknown error')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex items-center gap-3 mb-2 px-4 py-3 bg-amber-50 border border-amber-200 rounded-lg text-sm">
      <span className="text-amber-700">
        Templates v{current} → v{latest} available
      </span>
      {upgrade?.url && (
        <a href={upgrade.url} target="_blank" rel="noopener noreferrer" className="text-blue-600 hover:underline font-mono text-xs">
          {upgrade.state === 'open' ? 'Review upgrade PR' : 'Upgrade PR'}
        </a>
      )}
      <button
        onClick={run}
        disabled={busy}
        className="ml-auto text-xs font-medium px-3 py-1 border border-amber-300 text-amber-800 rounded hover:bg-amber-100 disabled:opacity-50"
      >
        {busy ? 'Working…' : upgrade?.state === 'open' ? 'Check PR status' : 'Open upgrade PR'}
      </button>
      {error && <span className="text-xs text-red-500">{error}</span>}
    </div>
  )
}
