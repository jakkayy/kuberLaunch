'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'

export default function SetupMonitoring({
  projectId,
  grafanaUrl,
}: {
  projectId: string
  grafanaUrl: string
}) {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const router = useRouter()

  if (grafanaUrl) {
    return (
      <a
        href={grafanaUrl}
        target="_blank"
        rel="noopener noreferrer"
        className="text-xs text-orange-600 font-medium px-3 py-2 border border-orange-200 rounded bg-orange-50 hover:bg-orange-100 transition-colors"
      >
        ✓ View Grafana
      </a>
    )
  }

  async function setup() {
    setLoading(true)
    setError('')
    try {
      const res = await fetch(`/api/projects/${projectId}/monitoring`, {
        method: 'POST',
      })
      const data = await res.json()
      if (!res.ok) throw new Error(data.error ?? 'Failed to setup monitoring')
      router.refresh()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Unknown error')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex flex-col items-end gap-1">
      <button
        onClick={setup}
        disabled={loading}
        className="flex items-center gap-2 border border-zinc-300 text-zinc-700 text-sm font-medium px-4 py-2 rounded hover:border-zinc-500 hover:bg-zinc-50 disabled:opacity-50 disabled:cursor-not-allowed transition-all"
      >
        <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <path d="M3 3v18h18" />
          <path d="m19 9-5 5-4-4-3 3" />
        </svg>
        {loading ? 'Setting up…' : 'Setup Monitoring'}
      </button>
      {error && <p className="text-xs text-red-500">{error}</p>}
    </div>
  )
}
