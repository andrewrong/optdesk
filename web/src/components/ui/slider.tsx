import * as React from "react"
import { cn } from "@/lib/utils"
import { Slider as SliderPrimitive } from "radix-ui"

// Read-only use: the gauge passes `disabled`; the range stays hidden so only the track and the thumb show.
function Slider({
  className,
  children,
  thumbLabel,
  ...props
}: React.ComponentProps<typeof SliderPrimitive.Root> & { thumbLabel?: string }) {
  return (
    <SliderPrimitive.Root
      data-slot="slider"
      className={cn(
        "relative flex w-full touch-none items-center select-none data-disabled:opacity-100",
        className
      )}
      {...props}
    >
      <SliderPrimitive.Track
        data-slot="slider-track"
        className="relative h-1.5 w-full grow overflow-hidden rounded-full bg-muted"
      >
        <SliderPrimitive.Range data-slot="slider-range" className="hidden" />
        {children}
      </SliderPrimitive.Track>
      <SliderPrimitive.Thumb
        data-slot="slider-thumb"
        aria-label={thumbLabel}
        className="block size-3 shrink-0 rounded-full border border-primary bg-background ring-ring/50 transition-[color,box-shadow] focus-visible:ring-4 focus-visible:outline-hidden"
      />
    </SliderPrimitive.Root>
  )
}

export { Slider }
