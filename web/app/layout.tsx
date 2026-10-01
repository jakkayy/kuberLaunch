import type { Metadata } from 'next'
import { Geist, Geist_Mono } from 'next/font/google'
import Link from 'next/link'
import './globals.css'
import { getSessionUser } from './lib/session'

const geistSans = Geist({ variable: '--font-geist-sans', subsets: ['latin'] })
const geistMono = Geist_Mono({ variable: '--font-geist-mono', subsets: ['latin'] })

export const metadata: Metadata = {
  title: 'kuberLaunch',
  description: 'Internal Developer Platform — deploy services in one click',
}

export default async function RootLayout({ children }: { children: React.ReactNode }) {
  const user = await getSessionUser()

  return (
    <html lang="en" className={`${geistSans.variable} ${geistMono.variable} h-full`}>
      <body className="min-h-full flex flex-col bg-zinc-50 antialiased">
        <header className="bg-zinc-900 text-white px-6 py-3 flex items-center justify-between shrink-0">
          <Link href="/" className="flex items-center gap-2">
            <span className="font-bold text-lg tracking-tight">kuberLaunch</span>
            <span className="text-[10px] bg-zinc-700 text-zinc-400 px-1.5 py-0.5 rounded font-mono">IDP</span>
          </Link>
          {user && (
            <div className="flex items-center gap-3">
              <Link
                href="/projects/new"
                className="bg-white text-zinc-900 text-sm font-medium px-3 py-1.5 rounded hover:bg-zinc-100 transition-colors"
              >
                + New Project
              </Link>
              <span className="text-sm text-zinc-300 font-mono">{user.login}</span>
              <form action="/auth/logout" method="post">
                <button type="submit" className="text-xs text-zinc-400 hover:text-white transition-colors">
                  Sign out
                </button>
              </form>
            </div>
          )}
        </header>
        <main className="flex-1 max-w-4xl w-full mx-auto px-6 py-8">
          {children}
        </main>
      </body>
    </html>
  )
}
