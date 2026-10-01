export type Runtime = 'nextjs' | 'nestjs' | 'go' | 'fastapi'
export type Database = 'postgres' | 'mysql' | 'none'
export type Cache = 'redis' | 'none'
export type ProjectStatus = 'creating' | 'ready' | 'failed'
export type Role = 'viewer' | 'maintainer' | 'owner'

const ROLE_RANK: Record<Role, number> = { viewer: 1, maintainer: 2, owner: 3 }

export function roleAtLeast(role: Role | undefined, min: Role): boolean {
  return !!role && ROLE_RANK[role] >= ROLE_RANK[min]
}

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
  my_role?: Role
  created_at: string
  updated_at: string
}

export interface ProjectMember {
  id: string
  project_id: string
  user_id?: string
  login?: string
  team_id?: string
  team_slug?: string
  role: Role
  created_at: string
}

export interface Team {
  id: string
  slug: string
  name: string
  my_role?: Role
  created_at: string
}

export interface TeamMember {
  user_id: string
  login: string
  role: Role
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
