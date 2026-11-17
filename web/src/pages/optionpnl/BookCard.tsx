// One book card (an account's currency) and the shared three-column lines table. Every text is a field of the
// payload; nothing is compared, summed, sorted or formatted here.
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableRow } from '@/components/ui/table'
import type { Book, Line } from '@/generated/option_pnl_page.v1'

// The tone of a line selects a text class only; none has no class.
const TONE_CLASS: Record<Line['tone'], string> = {
  none: '',
  good: 'text-success',
  bad: 'text-down',
  muted: 'text-muted-foreground',
}

export function LineRow({ line }: { line: Line }) {
  return (
    <TableRow data-line={line.id}>
      <TableCell className="whitespace-normal">{line.label}</TableCell>
      <TableCell className={`text-right tabular-nums ${TONE_CLASS[line.tone]}`.trim()} data-tone={line.tone}>
        {line.text}
      </TableCell>
      <TableCell className="whitespace-normal text-xs text-muted-foreground" data-hint>
        {line.hint}
      </TableCell>
    </TableRow>
  )
}

export function LinesTable({ lines }: { lines: Line[] }) {
  return (
    <Table>
      <TableBody>
        {lines.map((line) => (
          <LineRow key={line.id} line={line} />
        ))}
      </TableBody>
    </Table>
  )
}

export function BookCard({ book }: { book: Book }) {
  return (
    <Card data-book={book.id}>
      <CardHeader>
        <CardTitle>{book.currency}</CardTitle>
        <CardDescription>{book.counts}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        {book.left_out ? (
          <p className="text-sm" data-left-out>
            {book.left_out}
          </p>
        ) : null}
        <LinesTable lines={[book.headline, ...book.lines]} />
      </CardContent>
    </Card>
  )
}
