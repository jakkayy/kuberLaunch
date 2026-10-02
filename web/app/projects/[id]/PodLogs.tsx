'use client'

import { useState, useEffect, useRef } from 'react'
import { ENVIRONMENTS, EnvName } from '@/app/lib/types'

export default function PodLogs({ projectId }: { projectId: string }) {
  const [open, setOpen] = useState(false)
  const [lines, setLines] = useState<string[]>([])
  const [streaming, setStreaming] = useState(false)
  const [env, setEnv] = useState<EnvName>('dev')
  const esRef = useRef<EventSource | null>(null)
  const bottomRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [lines])

  function start() {
    if (esRef.current) return
    setLines([])
    setStreaming(true)

    const es = new EventSource(`/api/projects/${projectId}/logs?env=${env}`)
    esRef.current = es

    es.onmessage = (e) => {
      try {
        const line = JSON.parse(e.data)
        setLines(prev => [...prev.slice(-500), line])
      } catch {
        setLines(prev => [...prev.slice(-500), e.data])
      }
    }

    es.onerror = () => {
      es.close()
      esRef.current = null
      setStreaming(false)
    }
  }

  function stop() {
    esRef.current?.close()
    esRef.current = null
    setStreaming(false)
  }

  function toggle() {
    if (!open) {
      setOpen(true)
      start()
    } else {
      stop()
      setOpen(false)
      setLines([])
    }
  }

  return (
    <div className="mb-6 border border-zinc-200 rounded-lg overflow-hidden">
      <div className="flex items-center justify-between px-4 py-3 bg-zinc-50 border-b border-zinc-200">
        <h2 className="text-sm font-semibold text-zinc-700">Pod Logs</h2>
        <div className="flex items-center gap-2">
          {open && (
            <span className={`text-xs ${streaming ? 'text-green-600' : 'text-zinc-400'}`}>
              {streaming ? '● live' : '○ stopped'}
            </span>
          )}
          <select
            value={env}
            onChange={e => setEnv(e.target.value as EnvName)}
            disabled={open}
            className="text-xs border border-zinc-300 rounded px-1.5 py-1 bg-white disabled:opacity-50"
          >
            {ENVIRONMENTS.map(e => <option key={e} value={e}>{e}</option>)}
          </select>
          <button
            onClick={toggle}
            className="text-xs font-medium px-3 py-1 border border-zinc-300 rounded hover:bg-zinc-100 transition-colors"
          >
            {open ? 'Stop' : 'Stream Logs'}
          </button>
        </div>
      </div>

      {open && (
        <div className="bg-zinc-950 text-zinc-100 font-mono text-xs p-4 h-72 overflow-y-auto">
          {lines.length === 0 ? (
            <span className="text-zinc-500">
              {streaming ? 'Waiting for logs…' : 'No logs received.'}
            </span>
          ) : (
            lines.map((line, i) => (
              <div key={i} className="whitespace-pre-wrap break-all leading-5">
                {line}
              </div>
            ))
          )}
          <div ref={bottomRef} />
        </div>
      )}
    </div>
  )
}
