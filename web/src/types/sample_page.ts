import { samples } from '@/samples'
import type { SamplePage } from '@/generated/sample_page.v1'

export const SUPPORTED_VERSIONS = [1] as const
export type Payload = SamplePage

// The sample payload a `sample_page` marker names, or a thrown reason: an unknown page or version fails closed.
export function sampleOf(input: unknown): unknown {
  const marker = input as { schema_version?: unknown; page?: unknown } | null
  if (marker?.schema_version !== 1) throw new Error('sample_page: unsupported schema_version')
  if (typeof marker.page !== 'string' || !Object.hasOwn(samples, marker.page)) {
    throw new Error('sample_page: unknown page')
  }
  return samples[marker.page]
}
