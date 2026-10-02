'use client'

import { useEffect, useState } from 'react'
import { AuditEntry } from '@/app/lib/types'

export default function AuditLog({ projectId }: { projectId: string }) {
  const [open, setOpen] = useState(false)
  const [entries, setEntries] = useState<AuditEntry[] | null>(null)

  useEffect(() => {
    if (!open) return
    let cancelled = false
    fetch(`/api/projects/${projectId}/audit`)
      .then(res => (res.ok ? res.json() : null))
      .then(data => { if (!cancelled) setEntries(data?.entries ?? []) })
    return () => { cancelled = true }
  }, [open, projectId])

  return (
    <div className="mb-6 border border-zinc-200 rounded-lg">
      <button
        onClick={() => setOpen(o => !o)}
        className="w-full flex items-center justify-between px-4 py-3 text-sm font-semibold text-zinc-700"
      >
        Audit log
        <span className="text-xs text-zinc-400">{open ? 'Hide' : 'Show'}</span>
      </button>
      {open && (
        <div className="border-t border-zinc-200 max-h-72 overflow-y-auto">
          {entries === null ? (
            <p className="text-xs text-zinc-400 p-4">Loading…</p>
          ) : entries.length === 0 ? (
            <p className="text-xs text-zinc-400 p-4">No activity yet.</p>
          ) : (
            <table className="w-full text-xs">
              <tbody>
                {entries.map(e => (
                  <tr key={e.id} className="border-b border-zinc-100 last:border-0">
                    <td className="px-4 py-1.5 text-zinc-400 whitespace-nowrap">
                      {new Date(e.created_at).toLocaleString()}
                    </td>
                    <td className="px-2 py-1.5 font-mono text-zinc-700">{e.actor_login}</td>
                    <td className="px-2 py-1.5 font-mono">{e.action}</td>
                    <td className="px-2 py-1.5 font-mono text-zinc-500">
                      {Object.entries(e.details ?? {}).map(([k, v]) => `${k}=${v}`).join(' ')}
                    </td>
                    <td className={`px-4 py-1.5 text-right ${e.status >= 400 ? 'text-red-500' : 'text-green-600'}`}>
                      {e.status}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}
    </div>
  )
}
