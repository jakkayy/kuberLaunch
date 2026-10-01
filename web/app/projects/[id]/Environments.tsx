'use client'

import { useCallback, useEffect, useState } from 'react'
import { EnvironmentStatus, EnvName } from '@/app/lib/types'

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

function imageTag(images?: string[]) {
  const img = images?.[0]
  return img ? img.split(':').pop() : '—'
}

export default function Environments({
  projectId,
  registered,
  canRollback,
}: {
  projectId: string
  registered: boolean
  canRollback: boolean
}) {
  const [envs, setEnvs] = useState<EnvironmentStatus[]>([])
  const [busy, setBusy] = useState<EnvName | null>(null)
  const [message, setMessage] = useState('')

  const fetchStatus = useCallback(async () => {
    const res = await fetch(`/api/projects/${projectId}/environments`)
    if (res.ok) {
      const data = await res.json()
      setEnvs(data.environments ?? [])
    }
  }, [projectId])

  useEffect(() => {
    if (!registered) return
    const first = setTimeout(fetchStatus, 0)
    const id = setInterval(fetchStatus, 15000)
    return () => {
      clearTimeout(first)
      clearInterval(id)
    }
  }, [fetchStatus, registered])

  async function rollback(env: EnvName) {
    if (!confirm(`Rollback ${env} ไปยัง revision ก่อนหน้า?\n(auto-sync ของ ${env} จะถูกปิดชั่วคราว)`)) return
    setBusy(env)
    setMessage('')
    try {
      const res = await fetch(`/api/projects/${projectId}/environments/${env}/rollback`, { method: 'POST' })
      const data = await res.json()
      if (!res.ok) throw new Error(data.error ?? 'Rollback failed')
      setMessage(`↩ ${env} rolled back · auto-sync ปิดอยู่`)
      setTimeout(fetchStatus, 3000)
    } catch (e) {
      setMessage(e instanceof Error ? e.message : 'Unknown error')
    } finally {
      setBusy(null)
    }
  }

  if (!registered) return null

  return (
    <div className="mb-5 border border-purple-200 rounded-lg overflow-hidden">
      <div className="px-4 py-2 bg-purple-50 border-b border-purple-200 flex items-center justify-between">
        <h2 className="text-sm font-semibold text-purple-900">Environments</h2>
        {message && <span className="text-xs text-amber-700">{message}</span>}
      </div>
      <table className="w-full text-sm">
        <tbody>
          {envs.map(e => (
            <tr key={e.name} className="border-b border-zinc-100 last:border-0">
              <td className="px-4 py-2 font-mono font-medium text-zinc-800 w-20">{e.name}</td>
              <td className="px-2 py-2">
                {e.argocd_url ? (
                  <a href={e.argocd_url} target="_blank" rel="noopener noreferrer" className="font-mono text-xs text-purple-700 hover:underline">
                    {e.argocd_app}
                  </a>
                ) : (
                  <span className="font-mono text-xs text-zinc-500">{e.argocd_app}</span>
                )}
              </td>
              <td className="px-2 py-2">
                <span className={`text-xs font-medium px-2 py-0.5 border rounded ${HEALTH_COLOR[e.health] ?? HEALTH_COLOR.Unknown}`}>
                  {e.health || '…'}
                </span>
              </td>
              <td className={`px-2 py-2 text-xs font-medium ${SYNC_COLOR[e.sync] ?? 'text-zinc-400'}`}>{e.sync || '…'}</td>
              <td className="px-2 py-2 font-mono text-xs text-zinc-500">{imageTag(e.images)}</td>
              <td className="px-4 py-2 text-right">
                {canRollback && (
                  <button
                    onClick={() => rollback(e.name)}
                    disabled={busy !== null}
                    className="text-xs font-medium px-2.5 py-1 border border-purple-300 text-purple-700 rounded hover:bg-purple-100 disabled:opacity-40"
                  >
                    {busy === e.name ? 'Rolling back…' : 'Rollback'}
                  </button>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
