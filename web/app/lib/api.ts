import 'server-only'
import { Deployment, GeneratedFile, Project } from './types'
import { apiFetch } from './upstream'

export async function getProjects(): Promise<Project[]> {
  const res = await apiFetch(`/api/v1/projects`)
  if (!res.ok) throw new Error('Failed to fetch projects')
  const data = await res.json()
  return data.projects ?? []
}

export async function getProject(id: string): Promise<Project | null> {
  const res = await apiFetch(`/api/v1/projects/${id}`)
  if (!res.ok) return null
  return res.json()
}

export async function getProjectFiles(id: string): Promise<GeneratedFile[]> {
  const res = await apiFetch(`/api/v1/projects/${id}/preview`)
  if (!res.ok) return []
  const data = await res.json()
  return data.files ?? []
}

export async function getDeployments(id: string): Promise<Deployment[]> {
  const res = await apiFetch(`/api/v1/projects/${id}/deployments`)
  if (!res.ok) return []
  const data = await res.json()
  return data.deployments ?? []
}
