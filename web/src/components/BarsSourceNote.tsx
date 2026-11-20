// The note of the bars source, beside a bars table. The page builder writes the sentence (shared.BarSourceOf); the browser
// shows it as given, reads no bars_source kind and computes nothing. It takes the payload because only the newer versions
// of a page carry the field.
export function BarsSourceNote({ payload }: { payload: object }) {
  const note = 'bars_source_note' in payload ? payload.bars_source_note : undefined
  if (typeof note !== 'string' || note === '') return null
  return (
    <p className="text-sm text-muted-foreground" data-bars-source-note>
      {note}
    </p>
  )
}
