'use client'

import { useCallback, useEffect, useRef, useState } from 'react'
import { useRouter } from 'next/navigation'

type Step = 'github' | 'argocd' | 'monitoring'
type StepStatus = 'pending' | 'running' | 'done' | 'failed'

interface SetupState {
  steps: { step: Step; status: StepStatus; message: string }[]
  running: boolean
  complete: boolean
  failed: boolean
}

const STEP_LABEL: Record<Step, string> = {
  github: 'Connect GitHub',
  argocd: 'Register ArgoCD',
  monitoring: 'Setup Monitoring (optional)',
}

export default function OneClickSetup({
  projectId,
  isFullySetup,
}: {
  projectId: string
  isFullySetup: boolean
}) {
  const [state, setState] = useState<SetupState | null>(null)
  const [error, setError] = useState('')
  const esRef = useRef<EventSource | null>(null)
  const router = useRouter()

  // Setup runs on the server; the stream only reports progress, so closing
  // the page does not interrupt it.
  const watch = useCallback(() => {
    esRef.current?.close()
    const es = new EventSource(`/api/projects/${projectId}/setup/stream`)
    esRef.current = es
    es.onmessage = (e) => {
      const next: SetupState = JSON.parse(e.data)
      setState(next)
      if (!next.running && (next.complete || next.failed)) {
        es.close()
        router.refresh()
      }
    }
    es.onerror = () => es.close()
  }, [projectId, router])

  useEffect(() => {
    let cancelled = false
    fetch(`/api/projects/${projectId}/setup`)
      .then(res => (res.ok ? res.json() : null))
      .then((data: SetupState | null) => {
        if (cancelled || !data) return
        setState(data)
        if (data.running) watch()
      })
    return () => {
      cancelled = true
      esRef.current?.close()
    }
  }, [projectId, watch])

  if (isFullySetup) return null

  async function run() {
    setError('')
    const res = await fetch(`/api/projects/${projectId}/setup`, { method: 'POST' })
    const data = await res.json().catch(() => ({}))
    if (!res.ok) {
      setError(data.error ?? 'Failed to start setup')
      return
    }
    setState(data)
    watch()
  }

  const running = state?.running || state?.steps.some(s => s.status === 'running')
  const started = state?.steps.some(s => s.status !== 'pending')
  const label = running ? 'Running…' : state?.complete ? '✓ Done' : state?.failed ? 'Resume' : 'Setup All'

  return (
    <div className="border border-blue-200 bg-blue-50 rounded-lg p-4 mb-6">
      <div className="flex items-center justify-between mb-3">
        <div>
          <h2 className="text-sm font-semibold text-blue-900">Golden Path Setup</h2>
          <p className="text-xs text-blue-600 mt-0.5">One click — GitHub + ArgoCD + Monitoring · safe to re-run</p>
        </div>
        <button
          onClick={run}
          disabled={!!running || !!state?.complete}
          className="text-sm font-medium px-5 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
        >
          {label}
        </button>
      </div>

      {started && state && (
        <ul className="space-y-1 mt-2">
          {state.steps.map((s) => (
            <li key={s.step} className="flex items-center gap-2 text-xs">
              <span className={
                s.status === 'done' ? 'text-green-600' :
                s.status === 'failed' ? 'text-red-500' :
                s.status === 'running' ? 'text-blue-500 animate-pulse' :
                'text-zinc-400'
              }>
                {s.status === 'done' ? '✓' : s.status === 'failed' ? '✕' : s.status === 'running' ? '●' : '○'}
              </span>
              <span className="font-medium text-zinc-700">{STEP_LABEL[s.step] ?? s.step}</span>
              <span className="text-zinc-400 truncate">{s.message}</span>
            </li>
          ))}
        </ul>
      )}

      {error && <p className="text-xs text-red-500 mt-2">{error}</p>}
    </div>
  )
}
