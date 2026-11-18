// The colour of a verdict: a shadcn Badge whose variant is looked up from the level the builder sent.
import { Badge } from '@/components/ui/badge'
import type { Payload } from '@/types/base_data_page'

export type Level = Payload['top']['level']
const VARIANT = { pass: 'success', warn: 'warning', fail: 'down', unknown: 'unknown' } as const

export function LevelBadge({ level }: { level: Level }) {
  return (
    <Badge variant={VARIANT[level]} data-level={level}>
      {level}
    </Badge>
  )
}
