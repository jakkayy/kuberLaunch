'use client'

import { useState, useEffect, useCallback } from 'react'

export default function SecretsPanel({ projectId }: { projectId: string }) {
  const [keys, setKeys] = useState<string[]>([])
  const [loading, setLoading] = useState(true)
  const [newKey, setNewKey] = useState('')
  const [newValue, setNewValue] = useState('')
  const [saving, setSaving] = useState(false)
  const [deleting, setDeleting] = useState<string | null>(null)
  const [error, setError] = useState('')

  const fetchKeys = useCallback(async () => {
    const res = await fetch(`/api/projects/${projectId}/secrets`)
    if (res.ok) {
      const data = await res.json()
      setKeys(data.keys ?? [])
    }
    setLoading(false)
  }, [projectId])

  useEffect(() => { fetchKeys() }, [fetchKeys])

  async function addSecret(e: React.FormEvent) {
    e.preventDefault()
    if (!newKey.trim() || !newValue.trim()) return
    setSaving(true)
    setError('')
    try {
      const res = await fetch(`/api/projects/${projectId}/secrets`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key: newKey.trim(), value: newValue.trim() }),
      })
      const data = await res.json()
      if (!res.ok) throw new Error(data.error ?? 'Failed to save secret')
      setNewKey('')
      setNewValue('')
      fetchKeys()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Unknown error')
    } finally {
      setSaving(false)
    }
  }

  async function deleteSecret(key: string) {
    setDeleting(key)
    setError('')
    try {
      const res = await fetch(`/api/projects/${projectId}/secrets/${encodeURIComponent(key)}`, {
        method: 'DELETE',
      })
      if (!res.ok) {
        const data = await res.json()
        throw new Error(data.error ?? 'Failed to delete secret')
      }
      fetchKeys()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Unknown error')
    } finally {
      setDeleting(null)
    }
  }

  return (
    <div className="border border-zinc-200 rounded-lg p-4 mb-6">
      <h2 className="text-sm font-semibold text-zinc-700 mb-4">Secrets</h2>

      {/* Add secret form */}
      <form onSubmit={addSecret} className="flex gap-2 mb-4">
        <input
          type="text"
          placeholder="KEY"
          value={newKey}
          onChange={e => setNewKey(e.target.value.toUpperCase().replace(/[^A-Z0-9_]/g, ''))}
          className="font-mono text-xs border border-zinc-200 rounded px-3 py-2 w-40 focus:outline-none focus:border-zinc-400"
        />
        <input
          type="password"
          placeholder="value"
          value={newValue}
          onChange={e => setNewValue(e.target.value)}
          className="font-mono text-xs border border-zinc-200 rounded px-3 py-2 flex-1 focus:outline-none focus:border-zinc-400"
        />
        <button
          type="submit"
          disabled={saving || !newKey.trim() || !newValue.trim()}
          className="text-xs font-medium px-4 py-2 bg-zinc-900 text-white rounded hover:bg-zinc-700 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
        >
          {saving ? 'Saving…' : 'Add'}
        </button>
      </form>

      {error && <p className="text-xs text-red-500 mb-3">{error}</p>}

      {/* Secret list */}
      {loading ? (
        <p className="text-xs text-zinc-400">Loading…</p>
      ) : keys.length === 0 ? (
        <p className="text-xs text-zinc-400">No secrets stored yet.</p>
      ) : (
        <ul className="space-y-1">
          {keys.map(key => (
            <li
              key={key}
              className="flex items-center justify-between px-3 py-2 bg-zinc-50 rounded font-mono text-xs"
            >
              <span className="text-zinc-700">{key}</span>
              <button
                onClick={() => deleteSecret(key)}
                disabled={deleting === key}
                className="text-red-400 hover:text-red-600 disabled:opacity-40 transition-colors ml-4"
                title="Delete"
              >
                {deleting === key ? '…' : '✕'}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
