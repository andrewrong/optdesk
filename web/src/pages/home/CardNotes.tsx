// Notes and the card-level gaps list shared by the home cards: text as sent, as grey text.

export function CardNotes({
  notes,
  gaps,
  showGaps = true,
}: {
  notes: string[]
  gaps: { text: string }[]
  showGaps?: boolean
}) {
  return (
    <>
      {notes.map((note, index) => (
        <p key={`${index}-${note}`} className="text-xs text-muted-foreground">
          {note}
        </p>
      ))}
      {showGaps && gaps.length > 0 ? (
        <div data-gaps className="text-xs text-muted-foreground">
          <ul className="list-disc pl-4">
            {gaps.map((gap, index) => (
              <li key={`${index}-${gap.text}`}>{gap.text}</li>
            ))}
          </ul>
        </div>
      ) : null}
    </>
  )
}
