'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'

type Step = 'github' | 'argocd' | 'monitoring'

interface Progress {
  step: Step
  status: 'running' | 'done' | 'error'
  message: string
  done: boolean
}

const STEP_LABEL: Record<Step, string> = {
  github: 'Connect GitHub',
  argocd: 'Register ArgoCD',
  monitoring: 'Setup Monitoring',
}

export default function OneClickSetup({
  projectId,
  isFullySetup,
}: {
  projectId: string
  isFullySetup: boolean
}) {
  const [running, setRunning] = useState(false)
  const [steps, setSteps] = useState<Progress[]>([])
  const [done, setDone] = useState(false)
  const [error, setError] = useState('')
  const router = useRouter()

  if (isFullySetup) return null

  function run() {
    setRunning(true)
    setSteps([])
    setDone(false)
    setError('')

    const es = new EventSource(`/api/projects/${projectId}/setup/stream`)

    es.onmessage = (e) => {
      const progress: Progress = JSON.parse(e.data)
      setSteps(prev => {
        const idx = prev.findIndex(s => s.step === progress.step)
        if (idx >= 0) {
          const next = [...prev]
          next[idx] = progress
          return next
        }
        return [...prev, progress]
      })
      if (progress.done) {
        es.close()
        setRunning(false)
        setDone(true)
        router.refresh()
      }
    }

    es.onerror = () => {
      es.close()
      setRunning(false)
      setError('Setup stream disconnected. Refresh to check status.')
    }
  }

  return (
    <div className="border border-blue-200 bg-blue-50 rounded-lg p-4 mb-6">
      <div className="flex items-center justify-between mb-3">
        <div>
          <h2 className="text-sm font-semibold text-blue-900">Golden Path Setup</h2>
          <p className="text-xs text-blue-600 mt-0.5">One click — GitHub + ArgoCD + Monitoring</p>
        </div>
        <button
          onClick={run}
          disabled={running || done}
          className="text-sm font-medium px-5 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
        >
          {running ? 'Running…' : done ? '✓ Done' : 'Setup All'}
        </button>
      </div>

      {steps.length > 0 && (
        <ul className="space-y-1 mt-2">
          {steps.map((s) => (
            <li key={s.step} className="flex items-center gap-2 text-xs">
              <span className={
                s.status === 'done' ? 'text-green-600' :
                s.status === 'error' ? 'text-red-500' :
                'text-blue-500 animate-pulse'
              }>
                {s.status === 'done' ? '✓' : s.status === 'error' ? '✕' : '●'}
              </span>
              <span className="font-medium text-zinc-700">{STEP_LABEL[s.step] ?? s.step}</span>
              <span className="text-zinc-400">{s.message}</span>
            </li>
          ))}
        </ul>
      )}

      {error && <p className="text-xs text-red-500 mt-2">{error}</p>}
    </div>
  )
}
