'use client'

import { useState } from 'react'
import { GeneratedFile } from '@/app/lib/types'

export default function FileViewer({ files }: { files: GeneratedFile[] }) {
  const [selected, setSelected] = useState(files[0]?.path ?? '')
  const content = files.find(f => f.path === selected)?.content ?? ''

  return (
    <div className="border border-zinc-200 rounded-lg overflow-hidden flex h-[520px] bg-white">
      <div className="w-56 border-r border-zinc-200 overflow-y-auto shrink-0 bg-zinc-50">
        {files.map(f => (
          <button
            key={f.path}
            onClick={() => setSelected(f.path)}
            title={f.path}
            className={`w-full text-left px-3 py-2 text-xs font-mono truncate transition-colors ${
              selected === f.path
                ? 'bg-zinc-200 text-zinc-900'
                : 'text-zinc-600 hover:bg-zinc-100'
            }`}
          >
            {f.path}
          </button>
        ))}
      </div>
      <div className="flex-1 overflow-auto flex flex-col">
        <div className="px-4 py-2 border-b border-zinc-100 bg-zinc-50 shrink-0">
          <span className="text-xs font-mono text-zinc-500">{selected}</span>
        </div>
        <pre className="p-4 text-xs font-mono text-zinc-800 leading-relaxed flex-1 overflow-auto">
          {content}
        </pre>
      </div>
    </div>
  )
}
