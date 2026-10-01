const ERRORS: Record<string, string> = {
  invalid_state: 'Login expired or was tampered with. Please try again.',
  token_exchange_failed: 'GitHub did not accept the login. Please try again.',
  user_lookup_failed: 'Could not read your GitHub profile.',
  not_org_member: 'Your GitHub account is not a member of the allowed organization.',
}

export default async function LoginPage({
  searchParams,
}: {
  searchParams: Promise<{ next?: string; error?: string }>
}) {
  const { next, error } = await searchParams
  const loginHref = `/auth/login${next ? `?next=${encodeURIComponent(next)}` : ''}`

  return (
    <div className="max-w-sm mx-auto mt-20 bg-white border border-zinc-200 rounded-lg p-8 text-center">
      <h1 className="text-lg font-semibold text-zinc-900 mb-1">Sign in to kuberLaunch</h1>
      <p className="text-sm text-zinc-500 mb-6">Use your GitHub account to continue.</p>
      {error && (
        <p className="text-sm text-red-600 bg-red-50 border border-red-200 rounded px-3 py-2 mb-4">
          {ERRORS[error] ?? 'Login failed.'}
        </p>
      )}
      <a
        href={loginHref}
        className="block bg-zinc-900 text-white text-sm font-medium px-4 py-2.5 rounded hover:bg-zinc-700 transition-colors"
      >
        Sign in with GitHub
      </a>
    </div>
  )
}
