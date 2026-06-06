import { GeneratedFile, Project } from './types'

const API_URL = process.env.API_URL ?? 'http://localhost:8080'

export async function getProjects(): Promise<Project[]> {
  const res = await fetch(`${API_URL}/api/v1/projects`, { cache: 'no-store' })
  if (!res.ok) throw new Error('Failed to fetch projects')
  const data = await res.json()
  return data.projects ?? []
}

export async function getProject(id: string): Promise<Project | null> {
  const res = await fetch(`${API_URL}/api/v1/projects/${id}`, { cache: 'no-store' })
  if (!res.ok) return null
  return res.json()
}

export async function getProjectFiles(id: string): Promise<GeneratedFile[]> {
  const res = await fetch(`${API_URL}/api/v1/projects/${id}/preview`, { cache: 'no-store' })
  if (!res.ok) return []
  const data = await res.json()
  return data.files ?? []
}
