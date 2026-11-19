// The strategy_replay_page payload: the generated v1 type of contracts/schemas/page.strategy_replay_page.v1.json. The structural check of a
// payload (an object with a supported `schema_version`) is the shared placeholder until the generated validators land.
import type { StrategyReplayPage } from '@/generated/strategy_replay_page.v1'
import { placeholderValidate } from './placeholder.ts'

export const SUPPORTED_VERSIONS = [1] as const
export type Payload = StrategyReplayPage

export const validate = placeholderValidate<Payload>(SUPPORTED_VERSIONS)
