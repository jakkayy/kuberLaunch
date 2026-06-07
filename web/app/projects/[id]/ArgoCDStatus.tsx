'use client'

import { useState, useEffect, useCallback } from 'react'

interface Status {
  health: string
  sync: string
}

const HEALTH_COLOR: Record<string, string> = {
  Healthy:     'text-green-600 bg-green-50 border-green-200',
  Progressing: 'text-blue-600 bg-blue-50 border-blue-200',
  Degraded:    'text-red-600 bg-red-50 border-red-200',
  Suspended:   'text-yellow-600 bg-yellow-50 border-yellow-200',
  Missing:     'text-zinc-500 bg-zinc-50 border-zinc-200',
  Unknown:     'text-zinc-400 bg-zinc-50 border-zinc-200',
}

const SYNC_COLOR: Record<string, string> = {
  Synced:    'text-green-600',
  OutOfSync: 'text-amber-500',
  Unknown:   'text-zinc-400',
}

export default function ArgoCDStatus({
  projectId,
  argocdApp,
}: {
  projectId: string
  argocdApp: string
}) {
  const [status, setStatus] = useState<Status | null>(null)
  const [rolling, setRolling] = useState(false)
  const [error, setError] = useState('')

  const fetchStatus = useCallback(async () => {
    const res = await fetch(`/api/projects/${projectId}/argocd/status`)
    if (res.ok) {
      const data = await res.json()
      setStatus(data)
    }
  }, [projectId])

  useEffect(() => {
    fetchStatus()
    const id = setInterval(fetchStatus, 15000)
    return () => clearInterval(id)
  }, [fetchStatus])

  async function rollback() {
    if (!confirm('Rollback ไปยัง revision ก่อนหน้า?')) return
    setRolling(true)
    setError('')
    try {
      const res = await fetch(`/api/projects/${projectId}/argocd/rollback`, { method: 'POST' })
      const data = await res.json()
      if (!res.ok) throw new Error(data.error ?? 'Rollback failed')
      setTimeout(fetchStatus, 3000)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Unknown error')
    } finally {
      setRolling(false)
    }
  }

  if (!argocdApp) return null

  const healthClass = HEALTH_COLOR[status?.health ?? ''] ?? 'text-zinc-400 bg-zinc-50 border-zinc-200'
  const syncClass = SYNC_COLOR[status?.sync ?? ''] ?? 'text-zinc-400'

  return (
    <div className="flex items-center gap-2 mb-5 px-4 py-3 bg-purple-50 border border-purple-200 rounded-lg">
      <span className="text-purple-600 text-sm">●</span>
      <span className="text-sm text-zinc-600">ArgoCD:</span>
      <a
        href="http://argocd.localhost:8090"
        target="_blank"
        rel="noopener noreferrer"
        className="text-sm font-mono text-purple-700 hover:underline"
      >
        {argocdApp}
      </a>

      {status && (
        <>
          <span className={`text-xs font-medium px-2 py-0.5 border rounded ${healthClass}`}>
            {status.health || '…'}
          </span>
          <span className={`text-xs font-medium ${syncClass}`}>
            {status.sync || '…'}
          </span>
        </>
      )}

      <div className="ml-auto flex items-center gap-2">
        {error && <span className="text-xs text-red-500">{error}</span>}
        <button
          onClick={rollback}
          disabled={rolling}
          className="text-xs font-medium px-3 py-1 border border-purple-300 text-purple-700 rounded hover:bg-purple-100 disabled:opacity-40 transition-colors"
        >
          {rolling ? 'Rolling back…' : 'Rollback'}
        </button>
      </div>
    </div>
  )
}
