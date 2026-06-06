'use server'

import { revalidatePath } from 'next/cache'
import { redirect } from 'next/navigation'

const API_URL = process.env.API_URL ?? 'http://localhost:8080'

type ActionState = { error: string } | null

export async function createProject(
  _prevState: ActionState,
  formData: FormData
): Promise<ActionState> {
  const body = {
    name: formData.get('name') as string,
    runtime: formData.get('runtime') as string,
    database: formData.get('database') as string,
    cache: formData.get('cache') as string,
  }

  const res = await fetch(`${API_URL}/api/v1/projects`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })

  if (!res.ok) {
    const err = await res.json()
    return { error: err.error ?? 'Failed to create project' }
  }

  const data = await res.json()
  revalidatePath('/')
  redirect(`/projects/${data.project.id}`)
}
