'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { Deployment, DeploymentStatus } from '@/app/lib/types'

const STATUS_COLOR: Record<DeploymentStatus, string> = {
  pending:   'text-zinc-500',
  building:  'text-amber-500',
  deploying: 'text-blue-500',
  success:   'text-green-600',
  failed:    'text-red-500',
}

const STATUS_LABEL: Record<DeploymentStatus, string> = {
  pending:   '⏳ pending',
  building:  '🔨 building…',
  deploying: '🚀 deploying…',
  success:   '✓ success',
  failed:    '✗ failed',
}

export default function DeployButton({
  projectId,
  hasRepo,
  latestDeployment,
}: {
  projectId: string
  hasRepo: boolean
  latestDeployment?: Deployment
}) {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [liveStatus, setLiveStatus] = useState<DeploymentStatus | null>(null)
  const [branch, setBranch] = useState('main')
  const router = useRouter()

  async function deploy() {
    setLoading(true)
    setError('')
    setLiveStatus(null)
    try {
      const res = await fetch(`/api/projects/${projectId}/deployments`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ branch }),
      })
      const data = await res.json()
      if (!res.ok) throw new Error(data.error ?? 'Failed to trigger deployment')

      const depId: string = data.id
      setLiveStatus('building')

      // Subscribe to SSE stream for real-time status
      const es = new EventSource(`/api/projects/${projectId}/deployments/${depId}/stream`)
      es.onmessage = (e) => {
        const payload = JSON.parse(e.data) as { status: DeploymentStatus }
        setLiveStatus(payload.status)
        if (payload.status === 'success' || payload.status === 'failed') {
          es.close()
          setLoading(false)
          router.refresh()
        }
      }
      es.onerror = () => {
        es.close()
        setLoading(false)
        router.refresh()
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Unknown error')
      setLoading(false)
    }
  }

  const activeStatus = liveStatus ?? latestDeployment?.status

  return (
    <div className="flex flex-col items-end gap-1">
      <div className="flex items-center gap-2">
        <input
          type="text"
          value={branch}
          onChange={(e) => setBranch(e.target.value)}
          disabled={loading}
          placeholder="branch"
          className="border border-zinc-300 rounded px-2 py-1.5 text-sm font-mono w-28 focus:outline-none focus:border-zinc-500 disabled:opacity-50"
        />
        <button
          onClick={deploy}
          disabled={loading || !hasRepo}
          title={!hasRepo ? 'Connect to GitHub first' : undefined}
          className="flex items-center gap-2 bg-zinc-900 text-white text-sm font-medium px-4 py-2 rounded hover:bg-zinc-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
        >
          <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <polygon points="5 3 19 12 5 21 5 3" />
          </svg>
          {loading ? 'Deploying…' : 'Deploy'}
        </button>
      </div>
      {activeStatus && (
        <span className={`text-xs font-medium ${STATUS_COLOR[activeStatus]}`}>
          {STATUS_LABEL[activeStatus]}
        </span>
      )}
      {error && <p className="text-xs text-red-500">{error}</p>}
    </div>
  )
}
