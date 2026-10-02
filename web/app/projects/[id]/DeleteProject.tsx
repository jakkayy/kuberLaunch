'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'

export default function DeleteProject({ projectId, slug }: { projectId: string; slug: string }) {
  const [confirm, setConfirm] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const router = useRouter()

  async function remove(e: React.FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError('')
    const res = await fetch(`/api/projects/${projectId}`, {
      method: 'DELETE',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ confirm }),
    })
    if (res.ok) {
      router.push('/')
      router.refresh()
      return
    }
    const data = await res.json().catch(() => ({}))
    setError(data.error ?? 'Delete failed')
    setBusy(false)
  }

  return (
    <div className="mb-6 border border-red-200 rounded-lg p-4">
      <h2 className="text-sm font-semibold text-red-700 mb-1">Delete project</h2>
      <p className="text-xs text-zinc-500 mb-3">
        Removes the ArgoCD application (and its running workloads) and Vault access, and archives the
        GitHub repo. The repo, its history and the audit log are kept.
      </p>
      <form onSubmit={remove} className="flex gap-2">
        <input
          value={confirm}
          onChange={e => setConfirm(e.target.value)}
          placeholder={`type ${slug} to confirm`}
          className="flex-1 border border-zinc-300 rounded px-2 py-1.5 text-sm font-mono"
        />
        <button
          type="submit"
          disabled={busy || confirm !== slug}
          className="text-sm font-medium px-4 py-1.5 bg-red-600 text-white rounded hover:bg-red-700 disabled:opacity-40 disabled:cursor-not-allowed"
        >
          {busy ? 'Deleting…' : 'Delete'}
        </button>
      </form>
      {error && <p className="text-xs text-red-500 mt-2">{error}</p>}
    </div>
  )
}
