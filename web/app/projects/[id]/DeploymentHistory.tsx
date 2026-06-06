import { Deployment } from '@/app/lib/types'

const STATUS_COLOR: Record<string, string> = {
  pending:   'bg-zinc-100 text-zinc-500',
  building:  'bg-amber-50 text-amber-600',
  deploying: 'bg-blue-50 text-blue-600',
  success:   'bg-green-50 text-green-700',
  failed:    'bg-red-50 text-red-600',
}

function timeAgo(iso: string) {
  const diff = Math.floor((Date.now() - new Date(iso).getTime()) / 1000)
  if (diff < 60) return `${diff}s ago`
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`
  return `${Math.floor(diff / 86400)}d ago`
}

export default function DeploymentHistory({ deployments }: { deployments: Deployment[] }) {
  if (deployments.length === 0) {
    return (
      <p className="text-sm text-zinc-400 py-4 text-center border border-dashed border-zinc-200 rounded-lg">
        No deployments yet
      </p>
    )
  }

  return (
    <div className="border border-zinc-200 rounded-lg divide-y divide-zinc-100">
      {deployments.map((d) => (
        <div key={d.id} className="flex items-center justify-between px-4 py-3">
          <div className="flex items-center gap-3">
            <span className={`text-xs font-medium px-2 py-0.5 rounded-full ${STATUS_COLOR[d.status] ?? ''}`}>
              {d.status}
            </span>
            <span className="text-sm font-mono text-zinc-700">{d.branch}</span>
            <span className="text-xs text-zinc-400">by {d.triggered_by}</span>
          </div>
          <span className="text-xs text-zinc-400 font-mono">{timeAgo(d.created_at)}</span>
        </div>
      ))}
    </div>
  )
}
