import { ChevronDown, Menu, type LucideIcon } from 'lucide-react'
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
const HOME: NavLink = { label: '首页', path: '/home' }
const NAV_GROUPS: readonly NavGroup[] = [
  {
    label: '市场',
    items: [
      { label: '宏观', path: '/macro' },
      { label: '市场状态', path: '/market-state' },
    ],
  },
  {
    label: '持仓',
    items: [
      { label: '家庭账户', path: '/household' },
      { label: '持仓明细', path: '/positions' },
      { label: '持有公司', path: '/held-family' },
      { label: '风险', path: '/risk' },
    ],
  },
  {
    label: '期权',
    items: [
      { label: '候选', path: '/options/candidates' },
      {
        label: '持有期权',
        links: [
          { label: '组合结构', path: '/options/structures' },
          { label: '价差腿', path: '/options/spread-legs' },
          { label: '结果', path: '/options/result' },
          { label: '通知', path: '/options/notices' },
          { label: '跟踪', path: '/options/tracking' },
          { label: '盈亏拆分', path: '/options/pnl' },
        ],
      },
      { label: '策略回放', path: '/strategy-replay' },
    ],
  },
  {
    label: '公司',
    items: [
      { label: '自选清单', path: '/underlyings' },
      { label: '价位线', path: '/plan/price-lines' },
      { label: '估值', path: '/valuation' },
      { label: '导入估值', path: '/valuation-import' },
      { label: '数据', path: '/company-data' },
    ],
  },
  {
    label: '系统',
    items: [
      { label: '设置', path: '/settings' },
      { label: '任务台', path: '/job-console' },
      { label: '基础数据', path: '/base-data' },
      { label: '数据健康', path: '/data-health' },
      { label: '期权数据', path: '/options/data', prefix: true },
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
            <SheetTitle>菜单</SheetTitle>
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
