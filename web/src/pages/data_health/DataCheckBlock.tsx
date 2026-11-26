// The data check block of the data health page (docs/design/data-check.md#10-data-check-web-block): the last run, the
// three counts and one row per finding. Every visible value is a payload field; this file computes, sorts and words nothing.
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import type { Payload } from '@/types/data_health_page'

export function DataCheckBlock({ check }: { check: NonNullable<Payload['data_check']> }) {
  return (
    <Card data-data-check>
      <CardHeader>
        <CardTitle>{check.title}</CardTitle>
        <p className="text-sm text-muted-foreground">{check.note}</p>
      </CardHeader>
      <CardContent className="space-y-3 text-sm">
        {check.none_text ? (
          <p data-none>{check.none_text}</p>
        ) : (
          <>
            <p data-last-run>
              {check.check_day} {check.finished_at} {check.rules_unknown} / {check.rules_total}
            </p>
            <p data-counts className="flex gap-3">
              {(['error', 'warn', 'unknown'] as const).map((sev) => {
                // The payload carries no label per count, so the web layer words none: the count is a bare number and the
                // severity_text of a finding of that severity, when there is one, is its tooltip.
                const word = check.findings.find((f) => f.severity === sev)?.severity_text
                return (
                  <span key={sev} data-count={sev} title={word}>
                    {check.counts[sev]}
                  </span>
                )
              })}
            </p>
            <table className="w-full text-left">
              <tbody>
                {check.findings.map((f, i) => (
                  <tr key={`${i}-${f.rule}-${f.dataset}-${f.day}`} data-finding={f.rule} data-severity={f.severity}>
                    <td>
                      {f.label}
                      {f.is_new ? <span data-new> 新</span> : null}
                    </td>
                    <td>{f.dataset}</td>
                    <td>{f.day}</td>
                    <td>{f.severity_text}</td>
                    <td>{f.evidence_text}</td>
                    <td>{f.why_text}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </>
        )}
      </CardContent>
    </Card>
  )
}
