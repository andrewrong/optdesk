// The note of the bars source, beside a bars table. The page builder writes the sentence (shared.BarSourceOf); the browser
// shows it as given, reads no bars_source kind and computes nothing.
export function BarsSourceNote({ note }: { note?: string }) {
  if (typeof note !== 'string' || note === '') return null
  return (
    <p className="text-sm text-muted-foreground" data-bars-source-note>
      {note}
    </p>
  )
}
