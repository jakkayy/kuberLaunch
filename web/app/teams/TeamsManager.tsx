'use client'

import { useEffect, useState } from 'react'
import { Role, Team, TeamMember } from '@/app/lib/types'

const ROLES: Role[] = ['viewer', 'maintainer', 'owner']

async function send(url: string, init: RequestInit) {
  const res = await fetch(url, init)
  const data = await res.json().catch(() => ({}))
  if (!res.ok) throw new Error(data.error ?? 'Request failed')
  return data
}

function TeamCard({ team }: { team: Team }) {
  const [members, setMembers] = useState<TeamMember[]>([])
  const [reloadToken, setReloadToken] = useState(0)
  const [login, setLogin] = useState('')
  const [role, setRole] = useState<Role>('viewer')
  const [error, setError] = useState('')
  const canManage = team.my_role === 'owner'

  useEffect(() => {
    let cancelled = false
    fetch(`/api/teams/${team.slug}/members`)
      .then(res => (res.ok ? res.json() : null))
      .then(data => { if (!cancelled && data) setMembers(data.members ?? []) })
    return () => { cancelled = true }
  }, [team.slug, reloadToken])

  async function run(url: string, init: RequestInit) {
    setError('')
    try {
      await send(url, init)
      setReloadToken(t => t + 1)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Unknown error')
    }
  }

  return (
    <div className="bg-white border border-zinc-200 rounded-lg p-4">
      <div className="flex items-baseline justify-between mb-2">
        <p className="font-medium text-zinc-900">{team.name} <span className="text-xs font-mono text-zinc-400">{team.slug}</span></p>
        {team.my_role && <span className="text-xs text-zinc-500">you: {team.my_role}</span>}
      </div>
      <ul className="text-sm divide-y divide-zinc-100 mb-2">
        {members.map(m => (
          <li key={m.user_id} className="flex justify-between py-1.5">
            <span className="font-mono">{m.login}</span>
            <span className="flex gap-3 text-xs text-zinc-500">
              {m.role}
              {canManage && (
                <button className="text-red-500" onClick={() => run(`/api/teams/${team.slug}/members/${m.login}`, { method: 'DELETE' })}>
                  Remove
                </button>
              )}
            </span>
          </li>
        ))}
      </ul>
      {canManage && (
        <form
          className="flex gap-2"
          onSubmit={e => {
            e.preventDefault()
            if (!login.trim()) return
            run(`/api/teams/${team.slug}/members`, {
              method: 'PUT',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ login: login.trim(), role }),
            }).then(() => setLogin(''))
          }}
        >
          <input value={login} onChange={e => setLogin(e.target.value)} placeholder="github-login"
            className="flex-1 border border-zinc-300 rounded px-2 py-1 text-sm font-mono" />
          <select value={role} onChange={e => setRole(e.target.value as Role)} className="border border-zinc-300 rounded px-2 py-1 text-sm bg-white">
            {ROLES.map(r => <option key={r} value={r}>{r}</option>)}
          </select>
          <button className="text-sm px-3 py-1 bg-zinc-900 text-white rounded">Add</button>
        </form>
      )}
      {error && <p className="text-xs text-red-500 mt-2">{error}</p>}
    </div>
  )
}

export default function TeamsManager() {
  const [teams, setTeams] = useState<Team[]>([])
  const [reloadToken, setReloadToken] = useState(0)
  const [slug, setSlug] = useState('')
  const [name, setName] = useState('')
  const [error, setError] = useState('')

  useEffect(() => {
    let cancelled = false
    fetch('/api/teams')
      .then(res => (res.ok ? res.json() : null))
      .then(data => { if (!cancelled && data) setTeams(data.teams ?? []) })
    return () => { cancelled = true }
  }, [reloadToken])

  async function create(e: React.FormEvent) {
    e.preventDefault()
    setError('')
    try {
      await send('/api/teams', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ slug: slug.trim(), name: name.trim() }),
      })
      setSlug('')
      setName('')
      setReloadToken(t => t + 1)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Unknown error')
    }
  }

  return (
    <div className="space-y-4">
      <form onSubmit={create} className="flex gap-2">
        <input value={slug} onChange={e => setSlug(e.target.value)} placeholder="team-slug" required
          className="border border-zinc-300 rounded px-2 py-1.5 text-sm font-mono" />
        <input value={name} onChange={e => setName(e.target.value)} placeholder="Team name" required
          className="flex-1 border border-zinc-300 rounded px-2 py-1.5 text-sm" />
        <button className="text-sm font-medium px-3 py-1.5 bg-zinc-900 text-white rounded">Create team</button>
      </form>
      {error && <p className="text-xs text-red-500">{error}</p>}
      {teams.length === 0 && <p className="text-sm text-zinc-500">You are not in any team yet.</p>}
      {teams.map(t => <TeamCard key={t.id} team={t} />)}
    </div>
  )
}
