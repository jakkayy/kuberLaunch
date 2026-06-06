import Link from 'next/link'
import { notFound } from 'next/navigation'
import { getProject, getProjectFiles } from '@/app/lib/api'
import FileViewer from './FileViewer'
import ConnectGitHub from './ConnectGitHub'

const RUNTIME_LABEL: Record<string, string> = {
  go: 'Go', nextjs: 'Next.js', nestjs: 'NestJS', fastapi: 'FastAPI',
}

const STATUS_COLOR: Record<string, string> = {
  ready:    'text-green-600',
  creating: 'text-amber-500',
  failed:   'text-red-500',
}

export default async function ProjectPage({
  params,
}: {
  params: Promise<{ id: string }>
}) {
  const { id } = await params
  const [project, files] = await Promise.all([getProject(id), getProjectFiles(id)])

  if (!project) notFound()

  const meta = [
    RUNTIME_LABEL[project.runtime] ?? project.runtime,
    project.database !== 'none' ? project.database : null,
    project.cache !== 'none' ? project.cache : null,
  ].filter(Boolean).join(' · ')

  return (
    <div>
      <div className="flex items-start justify-between mb-6">
        <div>
          <div className="flex items-center gap-2 mb-2">
            <Link href="/" className="text-sm text-zinc-500 hover:text-zinc-900 transition-colors">
              Projects
            </Link>
            <span className="text-zinc-300">/</span>
            <span className="text-sm text-zinc-700 font-mono">{project.name}</span>
          </div>
          <div className="flex items-center gap-3">
            <h1 className="text-xl font-semibold text-zinc-900 font-mono">{project.name}</h1>
            <span className={`text-sm font-medium ${STATUS_COLOR[project.status] ?? 'text-zinc-500'}`}>
              ● {project.status}
            </span>
          </div>
          <p className="text-xs text-zinc-400 mt-1 font-mono">{meta}</p>
        </div>

        <div className="flex items-center gap-2 shrink-0">
          <ConnectGitHub projectId={id} repoUrl={project.repo_url ?? ''} />
          <a
            href={`/api/projects/${id}/download`}
            download
            className="bg-zinc-900 text-white text-sm font-medium px-4 py-2 rounded hover:bg-zinc-700 transition-colors"
          >
            Download zip
          </a>
        </div>
      </div>

      {/* GitHub repo link */}
      {project.repo_url && (
        <div className="flex items-center gap-2 mb-5 px-4 py-3 bg-green-50 border border-green-200 rounded-lg">
          <span className="text-green-600 text-sm">●</span>
          <span className="text-sm text-zinc-600">Connected to GitHub:</span>
          <a
            href={project.repo_url}
            target="_blank"
            rel="noopener noreferrer"
            className="text-sm font-mono text-blue-600 hover:underline"
          >
            {project.repo_url.replace('https://github.com/', '')}
          </a>
        </div>
      )}

      {files.length > 0 ? (
        <FileViewer files={files} />
      ) : (
        <div className="text-center py-12 text-sm text-zinc-400 border border-dashed border-zinc-200 rounded-lg">
          No files generated
        </div>
      )}
    </div>
  )
}
