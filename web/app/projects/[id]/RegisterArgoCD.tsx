'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'

export default function RegisterArgoCD({
  projectId,
  argocdApp,
  hasRepo,
}: {
  projectId: string
  argocdApp: string
  hasRepo: boolean
}) {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const router = useRouter()

  if (argocdApp) {
    return (
      <span className="text-xs text-purple-600 font-medium px-3 py-2 border border-purple-200 rounded bg-purple-50">
        ✓ ArgoCD registered
      </span>
    )
  }

  async function register() {
    setLoading(true)
    setError('')
    try {
      const res = await fetch(`/api/projects/${projectId}/register-argocd`, {
        method: 'POST',
      })
      const data = await res.json()
      if (!res.ok) throw new Error(data.error ?? 'Failed to register')
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
        onClick={register}
        disabled={loading || !hasRepo}
        title={!hasRepo ? 'Connect to GitHub first' : undefined}
        className="flex items-center gap-2 border border-zinc-300 text-zinc-700 text-sm font-medium px-4 py-2 rounded hover:border-zinc-500 hover:bg-zinc-50 disabled:opacity-50 disabled:cursor-not-allowed transition-all"
      >
        <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <path d="M12 2L2 7l10 5 10-5-10-5z" />
          <path d="M2 17l10 5 10-5" />
          <path d="M2 12l10 5 10-5" />
        </svg>
        {loading ? 'Registering…' : 'Register in ArgoCD'}
      </button>
      {error && <p className="text-xs text-red-500">{error}</p>}
    </div>
  )
}
