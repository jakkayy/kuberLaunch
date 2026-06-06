import Link from 'next/link'
import { getProjects } from './lib/api'
import { Project } from './lib/types'

const RUNTIME_BADGE: Record<string, string> = {
  go:      'bg-blue-100 text-blue-700',
  nextjs:  'bg-zinc-900 text-white',
  nestjs:  'bg-red-100 text-red-700',
  fastapi: 'bg-green-100 text-green-700',
}

const RUNTIME_LABEL: Record<string, string> = {
  go:      'Go',
  nextjs:  'Next.js',
  nestjs:  'NestJS',
  fastapi: 'FastAPI',
}

const STATUS_COLOR: Record<string, string> = {
  ready:    'text-green-600',
  creating: 'text-amber-500',
  failed:   'text-red-500',
}

function ProjectCard({ project }: { project: Project }) {
  return (
    <Link href={`/projects/${project.id}`}>
      <div className="bg-white border border-zinc-200 rounded-lg px-5 py-4 flex items-center justify-between hover:border-zinc-400 hover:shadow-sm transition-all">
        <div className="flex items-center gap-3">
          <span className={`text-xs font-mono px-2 py-1 rounded ${RUNTIME_BADGE[project.runtime] ?? 'bg-zinc-100 text-zinc-700'}`}>
            {RUNTIME_LABEL[project.runtime] ?? project.runtime}
          </span>
          <div>
            <p className="font-medium text-zinc-900">{project.name}</p>
            <p className="text-xs text-zinc-500 font-mono">{project.slug}</p>
          </div>
        </div>
        <div className="flex items-center gap-4">
          <div className="text-xs text-zinc-400 hidden sm:flex gap-2">
            {project.database !== 'none' && <span>{project.database}</span>}
            {project.cache !== 'none' && <span>{project.cache}</span>}
          </div>
          <span className={`text-sm font-medium ${STATUS_COLOR[project.status] ?? 'text-zinc-500'}`}>
            ● {project.status}
          </span>
        </div>
      </div>
    </Link>
  )
}

export default async function HomePage() {
  const projects = await getProjects()

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-xl font-semibold text-zinc-900">Projects</h1>
        <span className="text-sm text-zinc-500">
          {projects.length} service{projects.length !== 1 ? 's' : ''}
        </span>
      </div>

      {projects.length === 0 ? (
        <div className="text-center py-20 border border-dashed border-zinc-300 rounded-lg bg-white">
          <p className="text-zinc-500 mb-4 text-sm">No projects yet</p>
          <Link
            href="/projects/new"
            className="bg-zinc-900 text-white px-4 py-2 rounded text-sm font-medium hover:bg-zinc-700 transition-colors"
          >
            Create your first project
          </Link>
        </div>
      ) : (
        <div className="flex flex-col gap-3">
          {projects.map(p => <ProjectCard key={p.id} project={p} />)}
        </div>
      )}
    </div>
  )
}
