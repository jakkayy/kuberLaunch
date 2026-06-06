export type Runtime = 'nextjs' | 'nestjs' | 'go' | 'fastapi'
export type Database = 'postgres' | 'mysql' | 'mongodb' | 'none'
export type Cache = 'redis' | 'none'
export type ProjectStatus = 'creating' | 'ready' | 'failed'

export interface Project {
  id: string
  name: string
  slug: string
  runtime: Runtime
  database: Database
  cache: Cache
  repo_url?: string
  status: ProjectStatus
  created_at: string
  updated_at: string
}

export interface GeneratedFile {
  path: string
  content: string
}
