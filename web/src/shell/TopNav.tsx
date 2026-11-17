import { ChevronDown, Lightbulb, Menu, type LucideIcon } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetTrigger } from '@/components/ui/sheet'

type NavLink = { label: string; path: string; prefix?: boolean; icon?: LucideIcon }
// A labelled run of links inside a menu (not a link itself).
type NavSection = { label: string; links: readonly NavLink[] }
type NavGroup = { label: string; items: readonly (NavLink | NavSection)[] }

// The top bar (Decisions 1 and 5 of docs/design/nav-groups.md): Home and five menus, in order. The desktop bar and the
// phone sheet both read this list; a new page adds one link to one group. Fixed shell text: no payload field is read.
const HOME: NavLink = { label: 'Home', path: '/home' }
const NAV_GROUPS: readonly NavGroup[] = [
  {
    label: 'Market',
    items: [
      { label: 'Macro', path: '/macro' },
      { label: 'Market state', path: '/market-state' },
    ],
  },
  {
    label: 'Portfolio',
    items: [
      { label: 'Household', path: '/household' },
      { label: 'Positions', path: '/positions' },
      { label: 'Risk', path: '/risk' },
    ],
  },
  {
    label: 'Options',
    items: [
      { label: 'Overview', path: '/options' },
      { label: 'Data', path: '/options/data', prefix: true },
      { label: 'Candidates', path: '/options/candidates' },
      {
        label: 'Held options',
        links: [
          { label: 'Structures', path: '/options/structures' },
          { label: 'Result', path: '/options/result' },
          { label: 'Notices', path: '/options/notices' },
          { label: 'Tracking', path: '/options/tracking' },
        ],
      },
      { label: 'Advice', path: '/advice', icon: Lightbulb },
    ],
  },
  {
    label: 'Companies',
    items: [
      { label: 'Watch list', path: '/underlyings' },
      { label: 'Price lines', path: '/plan/price-lines' },
      { label: 'Valuation', path: '/valuation' },
    ],
  },
  {
    label: 'System',
    items: [
      { label: 'Settings', path: '/settings' },
      { label: 'Job console', path: '/job-console' },
      { label: 'Data health', path: '/data-health' },
    ],
  },
]

const isSection = (item: NavLink | NavSection): item is NavSection => 'links' in item

// The page being viewed: a trailing slash is ignored and `/` counts as `/home`.
function currentPath(pathname: string): string {
  const trimmed = pathname.replace(/\/+$/, '')
  return trimmed === '' ? '/home' : trimmed
}

export function TopNav() {
  const current = currentPath(window.location.pathname)
  const isActive = ({ path, prefix }: NavLink) =>
    path === current || (prefix === true && current.startsWith(`${path}/`))
  const groupActive = (group: NavGroup) =>
    group.items.some((item) => (isSection(item) ? item.links.some(isActive) : isActive(item)))

  const sheetLink = (link: NavLink, indent: boolean) => {
    const active = isActive(link)
    const Icon = link.icon
    return (
      <li key={link.path}>
        <Button
          asChild
          variant={active ? 'secondary' : 'ghost'}
          className={`min-h-11 w-full justify-start${indent ? ' pl-6' : ''}`}
        >
          <a href={link.path} aria-current={active ? 'page' : undefined}>
            {Icon ? <Icon aria-hidden /> : null}
            {link.label}
          </a>
        </Button>
      </li>
    )
  }
  const menuLink = (link: NavLink) => {
    const active = isActive(link)
    const Icon = link.icon
    return (
      <DropdownMenuItem
        key={link.path}
        asChild
        className={active ? 'bg-secondary text-secondary-foreground' : undefined}
      >
        <a href={link.path} aria-current={active ? 'page' : undefined}>
          {Icon ? <Icon aria-hidden /> : null}
          {link.label}
        </a>
      </DropdownMenuItem>
    )
  }
  const homeActive = isActive(HOME)

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
            {sheetLink(HOME, false)}
            {NAV_GROUPS.flatMap((group) => [
              <li key={group.label} className="px-2 pt-2 text-sm font-medium text-muted-foreground">
                {group.label}
              </li>,
              ...group.items.flatMap((item) =>
                isSection(item)
                  ? [
                      <li key={item.label} className="px-2 pt-2 pl-6 text-sm font-medium text-muted-foreground">
                        {item.label}
                      </li>,
                      ...item.links.map((link) => sheetLink(link, true)),
                    ]
                  : [sheetLink(item, false)],
              ),
            ])}
          </ul>
        </SheetContent>
      </Sheet>
      <div className="hidden md:flex gap-1">
        <Button asChild variant={homeActive ? 'secondary' : 'ghost'} className="shrink-0">
          <a href={HOME.path} aria-current={homeActive ? 'page' : undefined}>
            {HOME.label}
          </a>
        </Button>
        {NAV_GROUPS.map((group) => (
          <DropdownMenu key={group.label}>
            <DropdownMenuTrigger asChild>
              <Button
                variant={groupActive(group) ? 'secondary' : 'ghost'}
                aria-current={groupActive(group) ? 'true' : undefined}
                className="shrink-0"
              >
                {group.label}
                <ChevronDown aria-hidden />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent>
              {group.items.map((item) =>
                isSection(item) ? (
                  <div key={item.label} role="group" aria-label={item.label}>
                    <DropdownMenuLabel>{item.label}</DropdownMenuLabel>
                    {item.links.map(menuLink)}
                  </div>
                ) : (
                  menuLink(item)
                ),
              )}
            </DropdownMenuContent>
          </DropdownMenu>
        ))}
      </div>
    </nav>
  )
}
