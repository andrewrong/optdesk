import { ChevronDown, Lightbulb, Menu, type LucideIcon } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetTrigger } from '@/components/ui/sheet'

// The links of the position menu (Decision 15 of docs/design/option-positions.md).
const POSITION_LINKS: readonly Record<'label' | 'path', string>[] = [
  { label: 'Structures', path: '/options/structures' },
  { label: 'Result', path: '/options/result' },
  { label: 'Notices', path: '/options/notices' },
  { label: 'Tracking', path: '/options/tracking' },
  { label: 'Runs', path: '/ops/today' },
]

// Ten of the eleven items (Decision 3 of docs/design/unified-site.md), in order; the eleventh is the menu after them. Fixed shell text: no payload field is read.
const ITEMS: readonly (Record<'label' | 'path', string> & { prefix?: boolean; icon?: LucideIcon })[] = [
  { label: 'Home', path: '/home' },
  { label: 'Macro', path: '/macro' },
  { label: 'Household', path: '/household' },
  { label: 'Positions', path: '/positions' },
  { label: 'Risk', path: '/risk' },
  { label: 'Options', path: '/options' },
  { label: 'Option data', path: '/options/data', prefix: true },
  { label: 'Option candidates', path: '/options/candidates' },
  { label: 'Advice', path: '/advice', icon: Lightbulb },
  { label: 'Settings', path: '/settings' },
  { label: 'Job console', path: '/job-console' },
]

// The page being viewed: a trailing slash is ignored and `/` counts as `/home`.
function currentPath(pathname: string): string {
  const trimmed = pathname.replace(/\/+$/, '')
  return trimmed === '' ? '/home' : trimmed
}

export function TopNav() {
  const current = currentPath(window.location.pathname)
  const isActive = (path: string, prefix?: boolean) =>
    path === current || (prefix === true && current.startsWith(`${path}/`))
  return (
    <nav aria-label="Main navigation" className="p-2">
      <Sheet>
        <SheetTrigger asChild>
          <Button variant="ghost" size="icon" aria-label="Menu" className="size-11 md:hidden">
            <Menu aria-hidden />
          </Button>
        </SheetTrigger>
        <SheetContent side="left" className="overflow-y-auto">
          <SheetHeader>
            <SheetTitle>Menu</SheetTitle>
          </SheetHeader>
          <ul className="flex flex-col gap-1 px-2 pb-4">
            {ITEMS.map(({ label, path, prefix, icon: Icon }) => {
              const active = isActive(path, prefix)
              return (
                <li key={path}>
                  <Button asChild variant={active ? 'secondary' : 'ghost'} className="min-h-11 w-full justify-start">
                    <a href={path} aria-current={active ? 'page' : undefined}>
                      {Icon ? <Icon aria-hidden /> : null}
                      {label}
                    </a>
                  </Button>
                </li>
              )
            })}
            <li className="px-2 pt-2 text-sm font-medium text-muted-foreground">Option positions</li>
            {POSITION_LINKS.map(({ label, path }) => (
              <li key={path}>
                <Button asChild variant="ghost" className="min-h-11 w-full justify-start pl-6">
                  <a href={path}>{label}</a>
                </Button>
              </li>
            ))}
          </ul>
        </SheetContent>
      </Sheet>
      <div className="hidden md:flex gap-1">
        {ITEMS.map(({ label, path, prefix, icon: Icon }) => {
          const active = isActive(path, prefix)
          return (
            <Button key={path} asChild variant={active ? 'secondary' : 'ghost'} className="shrink-0">
              <a href={path} aria-current={active ? 'page' : undefined}>
                {Icon ? <Icon aria-hidden /> : null}
                {label}
              </a>
            </Button>
          )
        })}
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" className="shrink-0">
              Option positions
              <ChevronDown aria-hidden />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent>
            {POSITION_LINKS.map(({ label, path }) => (
              <DropdownMenuItem key={path} asChild>
                <a href={path}>{label}</a>
              </DropdownMenuItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </nav>
  )
}
