import { Link, useRouterState } from '@tanstack/react-router'
import type { PropsWithChildren } from 'react'

const pathLabels: Record<string, string> = {
  '/chat': '智能问答',
  '/reports': '报告生成',
  '/admin': '系统管理',
}

export function AppLayout({ children }: PropsWithChildren) {
  const routerState = useRouterState()
  const pathname = routerState.location.pathname
  const currentLabel =
    Object.entries(pathLabels).find(([key]) => pathname.startsWith(key))?.[1] ?? '首页'

  return (
    <div className="flex h-full flex-col bg-background text-foreground">
      {/* Top bar */}
      <header className="flex h-14 items-center justify-between border-b border-primary/30 bg-primary/5 px-6">
        <div className="flex items-center gap-3">
          <div className="flex h-8 w-8 items-center justify-center rounded-md bg-primary text-primary-foreground text-sm font-semibold">
            电
          </div>
          <span className="text-sm text-primary">电力行业知识助手</span>
          <span className="text-sm font-semibold">{currentLabel}</span>
        </div>
        <nav className="flex items-center gap-1 text-sm">
          <Link
            to="/chat"
            className="rounded-md px-2 py-1 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
            activeProps={{ className: 'text-primary bg-primary/10' }}
          >
            问答
          </Link>
          <Link
            to="/reports/generate"
            className="rounded-md px-2 py-1 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
            activeProps={{ className: 'text-primary bg-primary/10' }}
          >
            报告
          </Link>
          <Link
            to="/admin"
            className="rounded-md px-2 py-1 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
            activeProps={{ className: 'text-primary bg-primary/10' }}
          >
            管理
          </Link>
        </nav>
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          <span className="inline-block h-2 w-2 rounded-full bg-primary" aria-hidden="true" />
          <span>系统运行中</span>
        </div>
      </header>

      {/* Content */}
      <main className="flex-1 overflow-hidden">{children}</main>
    </div>
  )
}
