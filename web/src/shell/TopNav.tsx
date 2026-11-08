import { Lightbulb, type LucideIcon } from 'lucide-react'
import { Button } from '@/components/ui/button'

// The eight items of Decision 3 (docs/design/unified-site.md), in order. Fixed shell text: no payload field is read.
const ITEMS: readonly { label: string; path: string; prefix?: boolean; icon?: LucideIcon }[] = [
  { label: '首页', path: '/home' },
  { label: '宏观', path: '/macro' },
  { label: '家庭资产', path: '/household' },
  { label: '持仓', path: '/positions' },
  { label: '风险', path: '/risk' },
  { label: '期权', path: '/options' },
  { label: '期权数据', path: '/options/data', prefix: true },
  { label: '建议', path: '/advice', icon: Lightbulb },
]

// The page being viewed: a trailing slash is ignored and `/` counts as `/home`.
function currentPath(pathname: string): string {
  const trimmed = pathname.replace(/\/+$/, '')
  return trimmed === '' ? '/home' : trimmed
}

export function TopNav() {
  const current = currentPath(window.location.pathname)
  return (
    <nav aria-label="主导航" className="flex gap-1 overflow-x-auto p-2">
      {ITEMS.map(({ label, path, prefix, icon: Icon }) => {
        const active = path === current || (prefix === true && current.startsWith(`${path}/`))
        return (
          <Button key={path} asChild variant={active ? 'secondary' : 'ghost'} className="shrink-0">
            <a href={path} aria-current={active ? 'page' : undefined}>
              {Icon ? <Icon aria-hidden /> : null}
              {label}
            </a>
          </Button>
        )
      })}
    </nav>
  )
}
