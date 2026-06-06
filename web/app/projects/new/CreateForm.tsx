'use client'

import { useActionState } from 'react'
import { createProject } from '@/app/actions'

const RUNTIMES = [
  { value: 'go',      label: 'Go',      desc: 'gin · port 8080' },
  { value: 'nextjs',  label: 'Next.js', desc: 'React · port 3000' },
  { value: 'nestjs',  label: 'NestJS',  desc: 'Node.js · port 3000' },
  { value: 'fastapi', label: 'FastAPI', desc: 'Python · port 8000' },
]

export default function CreateForm() {
  const [state, action, pending] = useActionState(createProject, null)

  return (
    <form action={action} className="space-y-6">
      {state?.error && (
        <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded text-sm">
          {state.error}
        </div>
      )}

      <div>
        <label className="block text-sm font-medium text-zinc-700 mb-1">
          Project name
        </label>
        <input
          type="text"
          name="name"
          required
          minLength={2}
          maxLength={50}
          placeholder="user-service"
          className="w-full border border-zinc-300 rounded px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-zinc-500"
        />
        <p className="text-xs text-zinc-400 mt-1">
          lowercase + hyphens — used as Kubernetes namespace
        </p>
      </div>

      <div>
        <label className="block text-sm font-medium text-zinc-700 mb-2">
          Runtime
        </label>
        <div className="grid grid-cols-2 gap-2">
          {RUNTIMES.map((rt, i) => (
            <label key={rt.value} className="cursor-pointer">
              <input
                type="radio"
                name="runtime"
                value={rt.value}
                required
                defaultChecked={i === 0}
                className="peer sr-only"
              />
              <div className="border border-zinc-200 rounded-lg px-4 py-3 hover:border-zinc-400 peer-checked:border-zinc-900 peer-checked:bg-zinc-50 transition-all">
                <p className="font-medium text-sm text-zinc-900">{rt.label}</p>
                <p className="text-xs text-zinc-500 font-mono mt-0.5">{rt.desc}</p>
              </div>
            </label>
          ))}
        </div>
      </div>

      <div className="grid grid-cols-2 gap-4">
        <div>
          <label className="block text-sm font-medium text-zinc-700 mb-1">
            Database
          </label>
          <select
            name="database"
            className="w-full border border-zinc-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-zinc-500 bg-white"
          >
            <option value="postgres">PostgreSQL</option>
            <option value="mysql">MySQL</option>
            <option value="mongodb">MongoDB</option>
            <option value="none">None</option>
          </select>
        </div>
        <div>
          <label className="block text-sm font-medium text-zinc-700 mb-1">
            Cache
          </label>
          <select
            name="cache"
            className="w-full border border-zinc-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-zinc-500 bg-white"
          >
            <option value="redis">Redis</option>
            <option value="none">None</option>
          </select>
        </div>
      </div>

      <button
        type="submit"
        disabled={pending}
        className="w-full bg-zinc-900 text-white py-2.5 rounded text-sm font-medium hover:bg-zinc-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
      >
        {pending ? 'Creating…' : 'Create Project'}
      </button>
    </form>
  )
}
