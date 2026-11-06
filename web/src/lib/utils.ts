import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

// The standard shadcn `cn` helper: clsx and tailwind-merge, the two designated helper libraries.
// The radix-nova registry imports an npm package named `cn` instead; that package is not designated
// (ADR 0034), so the imports of the copied components point here.
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}
