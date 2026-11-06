// The local view-model types of the household page: aliases of the payload types that the blocks share.
import type { Payload } from '@/types/household_page'

export type Gap = NonNullable<Payload['overview']['gap']>
export type Cell = Payload['overview']['rows'][number]['amount']
export type MoneyBlock = Payload['overview']
export type Shown = { text: string; tone?: Cell['tone'] } | null | undefined
export type Accounts = NonNullable<Payload['accounts']>
export type Scope = Payload['scopes'][number]
export type Tag = Scope['accounts'][number]
export type Movers = Payload['movers']
export type MoverLine = NonNullable<Movers['stocks']>[number]
