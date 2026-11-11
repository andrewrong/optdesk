// global_assets: the day and five-day figure of each proxy as bars, the sector table and the rotation.
import { Chart } from '@/charts/Chart'
import type { ChartSpec } from '@/charts/spec'
import type { GlobalAssetsBlock } from '@/generated/report_page.v2'
import { GapMark, Tabular, isGap } from './cells'
import { Frame } from './frame'

type Proxy = Exclude<GlobalAssetsBlock['proxies'], { reason: string }>[number]

// A proxy with a Gap in a cell is not a bar; its text is listed under the chart.
function barSpec(proxies: readonly Proxy[]): { spec: ChartSpec | null; listed: string[] } {
  const bars: { label: string; day: Proxy['day']; five: Proxy['five_day'] }[] = []
  const listed: string[] = []
  for (const proxy of proxies) {
    if (isGap(proxy.day) || isGap(proxy.five_day)) {
      const gaps = [proxy.day, proxy.five_day].filter(isGap).map((gap) => gap.text)
      listed.push(`${proxy.label}: ${gaps.join(' · ')}`)
    } else {
      bars.push({ label: proxy.label, day: proxy.day, five: proxy.five_day })
    }
  }
  if (bars.length === 0) return { spec: null, listed }
  const point = (cell: { text: string; value?: number | null }) => ({
    value: cell.value ?? null,
    text: cell.text,
  })
  return {
    listed,
    spec: {
      kind: 'bar',
      categories: bars.map((bar) => bar.label),
      series: [
        { name: 'Today', points: bars.map((bar) => point(bar.day)) },
        { name: 'Five days', points: bars.map((bar) => point(bar.five)) },
      ],
      legend: true,
    },
  }
}

export function GlobalAssets({ block }: { block: GlobalAssetsBlock }) {
  const { proxies, sector, rotation } = block
  const bars = isGap(proxies) ? null : barSpec(proxies)
  return (
    <Frame block={block}>
      {isGap(proxies) ? <GapMark gap={proxies} /> : null}
      {bars?.spec ? <Chart spec={bars.spec} ariaText={block.chart_aria} name="global-assets" /> : null}
      {bars && bars.listed.length > 0 ? (
        <ul className="text-xs text-muted-foreground" data-proxy-gaps>
          {bars.listed.map((line) => (
            <li key={line}>{line}</li>
          ))}
        </ul>
      ) : null}
      {isGap(sector) ? (
        <GapMark gap={sector} />
      ) : (
        <section>
          <h3 className="text-sm font-medium">{sector.heading}</h3>
          <Tabular
            labels={sector.columns}
            rows={sector.rows.map((r) => [r.sector, r.etf, r.d1, r.d5, r.d20])}
          />
        </section>
      )}
      {rotation ? (
        <section data-rotation>
          <h3 className="text-sm font-medium">{rotation.title}</h3>
          <p className="text-sm">{rotation.detail}</p>
        </section>
      ) : null}
    </Frame>
  )
}
