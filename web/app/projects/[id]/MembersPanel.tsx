'use client'

import { useEffect, useState } from 'react'
import { ProjectMember, Role } from '@/app/lib/types'

const ROLES: Role[] = ['viewer', 'maintainer', 'owner']

export default function MembersPanel({ projectId, canManage }: { projectId: string; canManage: boolean }) {
  const [members, setMembers] = useState<ProjectMember[]>([])
  const [reloadToken, setReloadToken] = useState(0)
  const [kind, setKind] = useState<'login' | 'team'>('login')
  const [name, setName] = useState('')
  const [role, setRole] = useState<Role>('maintainer')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    let cancelled = false
    fetch(`/api/projects/${projectId}/members`)
      .then(res => (res.ok ? res.json() : null))
      .then(data => { if (!cancelled && data) setMembers(data.members ?? []) })
    return () => { cancelled = true }
  }, [projectId, reloadToken])

  async function call(url: string, init: RequestInit) {
    setBusy(true)
    setError('')
    try {
      const res = await fetch(url, init)
      const data = await res.json().catch(() => ({}))
      if (!res.ok) throw new Error(data.error ?? 'Request failed')
      setReloadToken(t => t + 1)
      return true
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Unknown error')
      return false
    } finally {
      setBusy(false)
    }
  }

  async function add(e: React.FormEvent) {
    e.preventDefault()
    if (!name.trim()) return
    const ok = await call(`/api/projects/${projectId}/members`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ [kind]: name.trim(), role }),
    })
    if (ok) setName('')
  }

  return (
    <div className="mb-6 border border-zinc-200 rounded-lg p-4">
      <h2 className="text-sm font-semibold text-zinc-700 mb-3">Members</h2>
      <ul className="divide-y divide-zinc-100 mb-3">
        {members.map(m => (
          <li key={m.id} className="flex items-center justify-between py-2 text-sm">
            <span className="font-mono text-zinc-800">
              {m.login ? m.login : <><span className="text-zinc-400">team/</span>{m.team_slug}</>}
            </span>
            <div className="flex items-center gap-3">
              <span className="text-xs text-zinc-500">{m.role}</span>
              {canManage && (
                <button
                  onClick={() => call(`/api/projects/${projectId}/members/${m.id}`, { method: 'DELETE' })}
                  disabled={busy}
                  className="text-xs text-red-500 hover:text-red-700 disabled:opacity-50"
                >
                  Remove
                </button>
              )}
            </div>
          </li>
        ))}
      </ul>
      {canManage && (
        <form onSubmit={add} className="flex items-center gap-2">
          <select value={kind} onChange={e => setKind(e.target.value as 'login' | 'team')}
            className="border border-zinc-300 rounded px-2 py-1.5 text-sm bg-white">
            <option value="login">User</option>
            <option value="team">Team</option>
          </select>
          <input value={name} onChange={e => setName(e.target.value)}
            placeholder={kind === 'login' ? 'github-login' : 'team-slug'}
            className="flex-1 border border-zinc-300 rounded px-2 py-1.5 text-sm font-mono" />
          <select value={role} onChange={e => setRole(e.target.value as Role)}
            className="border border-zinc-300 rounded px-2 py-1.5 text-sm bg-white">
            {ROLES.map(r => <option key={r} value={r}>{r}</option>)}
          </select>
          <button type="submit" disabled={busy}
            className="text-sm font-medium px-3 py-1.5 bg-zinc-900 text-white rounded hover:bg-zinc-700 disabled:opacity-50">
            Add
          </button>
        </form>
      )}
      {error && <p className="text-xs text-red-500 mt-2">{error}</p>}
    </div>
  )
}
