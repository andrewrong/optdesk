"use client"

import * as React from "react"
import * as RechartsPrimitive from "recharts"
import { cn } from "@/lib/utils"

// The shadcn chart component (ChartContainer, ChartTooltip, ChartTooltipContent, ChartLegend) over recharts.
// Written by hand after the registry item `radix-nova/chart`: the shadcn CLI could not reach the registry from
// the worker's sandbox (see SOURCES.md). The tooltip shows the `text` Python sent for a datum when there is one.

export type ChartConfig = Record<string, { label?: React.ReactNode; color?: string }>

const ChartContext = React.createContext<{ config: ChartConfig } | null>(null)

function useChart() {
  const context = React.useContext(ChartContext)
  if (!context) throw new Error("useChart must be used within a <ChartContainer />")
  return context
}

function ChartContainer({
  id,
  className,
  children,
  config,
  ...props
}: React.ComponentProps<"div"> & {
  config: ChartConfig
  children: React.ComponentProps<typeof RechartsPrimitive.ResponsiveContainer>["children"]
}) {
  const uniqueId = React.useId()
  const chartId = `chart-${id ?? uniqueId.replace(/:/g, "")}`
  return (
    <ChartContext.Provider value={{ config }}>
      <div
        data-slot="chart"
        data-chart-id={chartId}
        className={cn(
          "flex justify-center text-xs [&_.recharts-cartesian-axis-tick_text]:fill-muted-foreground [&_.recharts-cartesian-grid_line[stroke='#ccc']]:stroke-border/50 [&_.recharts-curve.recharts-tooltip-cursor]:stroke-border [&_.recharts-layer]:outline-hidden [&_.recharts-surface]:outline-hidden",
          className
        )}
        {...props}
      >
        <ChartStyle id={chartId} config={config} />
        <RechartsPrimitive.ResponsiveContainer initialDimension={{ width: 640, height: 288 }}>
          {children}
        </RechartsPrimitive.ResponsiveContainer>
      </div>
    </ChartContext.Provider>
  )
}

// The colour of a series is a CSS variable of the page theme, set once per chart as `--color-<key>`.
function ChartStyle({ id, config }: { id: string; config: ChartConfig }) {
  const colors = Object.entries(config).filter(([, item]) => item.color)
  if (colors.length === 0) return null
  const rules = colors.map(([key, item]) => `  --color-${key}: ${item.color};`).join("\n")
  return <style>{`[data-chart-id=${id}] {\n${rules}\n}`}</style>
}

const ChartTooltip = RechartsPrimitive.Tooltip

type TooltipEntry = {
  dataKey?: string | number
  name?: string | number
  value?: unknown
  color?: string
  payload?: Record<string, unknown>
}

function ChartTooltipContent({
  active,
  payload,
  label,
  className,
}: {
  active?: boolean
  payload?: TooltipEntry[]
  label?: React.ReactNode
  className?: string
}) {
  const { config } = useChart()
  if (!active || !payload?.length) return null
  return (
    <div
      className={cn(
        "grid min-w-32 items-start gap-1.5 rounded-lg border border-border/50 bg-background px-2.5 py-1.5 text-xs shadow-xl",
        className
      )}
    >
      {label ? <div className="font-medium">{label}</div> : null}
      {payload.map((entry) => {
        const key = String(entry.dataKey ?? entry.name)
        const text = entry.payload?.[`${key}_text`]
        const shown = typeof text === "string" ? text : String(entry.value ?? "")
        return (
          <div key={key} className="flex items-center justify-between gap-4">
            <span className="flex items-center gap-1.5 text-muted-foreground">
              <span
                aria-hidden
                className="size-2 shrink-0 rounded-[2px]"
                style={{ background: entry.color }}
              />
              {config[key]?.label ?? entry.name}
            </span>
            <span className="font-mono font-medium text-foreground tabular-nums">{shown}</span>
          </div>
        )
      })}
    </div>
  )
}

const ChartLegend = RechartsPrimitive.Legend

function ChartLegendContent({
  payload,
  className,
}: {
  payload?: { value?: React.ReactNode; color?: string; dataKey?: string | number }[]
  className?: string
}) {
  const { config } = useChart()
  if (!payload?.length) return null
  return (
    <div className={cn("flex items-center justify-center gap-4 pb-2", className)}>
      {payload.map((item) => (
        <div key={String(item.dataKey ?? item.value)} className="flex items-center gap-1.5">
          <span
            aria-hidden
            className="size-2 shrink-0 rounded-[2px]"
            style={{ background: item.color }}
          />
          {config[String(item.dataKey)]?.label ?? item.value}
        </div>
      ))}
    </div>
  )
}

export { ChartContainer, ChartLegend, ChartLegendContent, ChartStyle, ChartTooltip, ChartTooltipContent }
