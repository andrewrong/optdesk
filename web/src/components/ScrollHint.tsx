// The line over a table that scrolls sideways: it shows under 640px only, where a table is wider than the screen.
const HINT = '左右滑动可看全部列'

export function ScrollHint() {
  return (
    <p data-scroll-hint className="text-xs text-muted-foreground sm:hidden">
      {HINT}
    </p>
  )
}
