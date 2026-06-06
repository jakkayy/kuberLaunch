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
  argocd_app?: string
  grafana_url?: string
  status: ProjectStatus
  created_at: string
  updated_at: string
}

export interface GeneratedFile {
  path: string
  content: string
}

export type DeploymentStatus = 'pending' | 'building' | 'deploying' | 'success' | 'failed'

export interface Deployment {
  id: string
  project_id: string
  branch: string
  workflow_run_id?: number
  image?: string
  triggered_by: string
  status: DeploymentStatus
  created_at: string
  updated_at: string
}
