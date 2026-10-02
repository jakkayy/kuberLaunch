import { SignJWT, jwtVerify } from 'jose'

// Must match api/internal/auth (Issuer / Audience). The session cookie value is
// itself the bearer token forwarded to the Go API.
export const SESSION_COOKIE = 'kl_session'
export const SESSION_TTL_SECONDS = 12 * 60 * 60
const ISSUER = 'kuberlaunch-web'
const AUDIENCE = 'kuberlaunch-api'

export interface SessionUser {
  githubId: number
  login: string
  name: string
  avatarUrl: string
}

function secretKey() {
  const secret = process.env.AUTH_JWT_SECRET
  if (!secret || secret.length < 32) {
    throw new Error('AUTH_JWT_SECRET must be set to at least 32 characters')
  }
  return new TextEncoder().encode(secret)
}

export async function signSession(user: SessionUser): Promise<string> {
  return new SignJWT({ login: user.login, name: user.name, avatar_url: user.avatarUrl })
    .setProtectedHeader({ alg: 'HS256' })
    .setSubject(String(user.githubId))
    .setIssuer(ISSUER)
    .setAudience(AUDIENCE)
    .setIssuedAt()
    .setExpirationTime(`${SESSION_TTL_SECONDS}s`)
    .sign(secretKey())
}

export async function verifySession(token: string | undefined): Promise<SessionUser | null> {
  if (!token) return null
  try {
    const { payload } = await jwtVerify(token, secretKey(), {
      algorithms: ['HS256'],
      issuer: ISSUER,
      audience: AUDIENCE,
    })
    return {
      githubId: Number(payload.sub),
      login: String(payload.login ?? ''),
      name: String(payload.name ?? ''),
      avatarUrl: String(payload.avatar_url ?? ''),
    }
  } catch {
    return null
  }
}
