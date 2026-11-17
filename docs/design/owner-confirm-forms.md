# Owner confirm forms: the confirm step of /underlyings and /settings re-posts the owner's edit, so a save can be made

## Goal

No edit on `/underlyings` can be saved (owner, 2026-11-17), and `/settings` loses the owner's change at its confirm step (verified below). Each confirm step must post the preview's values back with `confirm=1`, and a test must follow the flow as the page makes it. The two-step flow of `docs/design/underlyings.md` Decision 5 stays. The owner merges design PRs one at a time, so both pages are one design. Plan item P1.

## Today

Read at origin/main 4438c820.
- `/underlyings`: the first post (no `confirm`) answers 200 with the confirm page and writes nothing (`cmd/ot/underlyings_post.go:86-88`); the second post with `confirm=1` writes (`:90`, `confirm` read at `:131`). `ConfirmCard` (`web/src/pages/underlyings/view.tsx:195-223`) has no `<form>`, no token and no button. `Confirm` (`contracts/schemas/page.underlyings.v1.json:69-77`, `internal/pages/underlyings/builder.go:42-45`) has `changes` and `stops` only. The page already has the top-level `form` token in confirm mode (`cmd/ot/underlyings_site.go:96-99`, `view.tsx:226`).
- `/settings`, end to end: `postOwnerSettings` (`cmd/ot/settings.go:162`) parses the post (`:195`); with no `confirm` it answers `page(200, settingsExtras{confirm: ...})` (`:208-214`), and `buildSettingsPage` (`:96-120`) builds the page with `Owner: rd.owner` (`:109`), the store. The builder takes param values and `excluded` from the store (`internal/pages/settings_page/builder.go:66-72`, `rows.go` `newParamRows`). `SettingsPage.tsx:78-85` makes the confirm form's hidden `param.*`, `row` and `excluded` from `payload.params` and `payload.rows`. So confirm posts the stored values, `Parse` finds no difference and the answer is the redirect `?saved=0` (`settings.go:200-203`). The preview page does not re-render the posted values; that is the gap.
- Why the tests missed it: the Go tests post `confirm=1` by hand (`cmd/ot/underlyings_post_test.go:28,69,81,85,95,101,105,115,124,150`; `cmd/ot/settings_test.go:231,240,263`); the web tests check that the card renders (`view.test.tsx:134`) and that the settings form has `confirm` and `expires` (`SettingsPage.test.tsx:101`). No test makes the second post from what the first answered.

## Decisions

1. `/underlyings`: `Confirm` gains only `fields`, a list of `{name, value}`: the posted fields of the preview. The token and `post_path` come from the page's top-level `form`. The echo is a whitelist of known names (`key`, `symbol`, `market`, `kind`, `name`, `status`, `issuer`, `exchange`, `currency`, `buy_price`, `sale_price`, `report_date_override`, `note`, `options`, `fundamentals`), the first value of each, in that order; `expires`, `mac`, `confirm` and any other name are not echoed. An unchecked switch is not posted, so it is not in `fields` and stays off (`underlyings_post.go:127-129`, an absent switch is off).
2. No cap truncates. `fields` has at most 15 items, the size of the whitelist, so no pair is dropped. A posted whitelisted value longer than 300 characters is refused by the route at the first post (422, "Refused <name>: longer than 300 characters"), and `Build` fails on such a value or on a confirm with no `form`; neither clips (the store limits are 200 for `note`, `underlyingpg/validate.go:27`).
3. `ConfirmCard` renders `<form method="post" action={form.post_path}>` with `expires`, `mac`, one hidden input per field, `confirm=1` and a "Confirm and save" button. The page computes nothing.
4. `/settings`: no schema change. The preview builds the page from the posted values: `cmd/ot` wraps `rd.owner` in a reader whose `Params` and `Exclusions` apply the parsed change (`ch.Params`, `ch.Exclusions`) over the stored rows, for the preview build only. The existing confirm form then posts the owner's values, `Parse` sees the same change, and the save follows. The edit table below the card shows the unsaved values, which is the preview.
5. Contract version: `page.underlyings` goes v1 to v2 by `docs/design/week1.md:106` (an additive change after the first consumer bumps the version; `internal/pages/underlyings` and `view.tsx` are consumers). v1 stays frozen with its fixtures. `page.settings_page` keeps v2: its payload does not change.
6. Tests follow the flow as the page makes it (Interface); the `/settings` test is in #2637 (Left out). Library: none new; own code, a few lines on each page (`/settings` already has the form shape).

## Modules

No row of `docs/design/modules.md` changes. Entry 1 spans contracts (`contracts/**`, `internal/contracts/**`, `web/src/generated/**`, `docs/contracts/page-schemas.md`), pages (`internal/pages/**`, `docs/contracts/pages.md`), ot (`cmd/**`, `docs/contracts/ot/**`) and web (`web/src/types/underlyings.ts`). It is the judged cut: the builder emits v2 only when the schema, the type and the route agree, and it lists exactly its Files. Entry 2 is web only. The /settings half is not an entry of this design (Left out).

## Interface

Owner of the schema: contracts; of the Go type: pages. `Confirm` of `page.underlyings.v2`: `changes`, `stops` as v1, and required `fields`: at most 15 objects of `name` (1 to 300 characters) and `value` (at most 300 characters), no extra property. Go: `type Field struct{ Name, Value string }`, `Confirm` gains `Fields []Field`. TypeScript: `Underlyings` of `underlyings.v2.ts`.

Round-trip tests, one Go test per page, each builds the second post only from the payload of the first answer: the path from `form.post_path` (`confirm.form.post_path` is not read), `expires` and `mac` from the payload, the fields from the payload as the card makes them; `confirm=1` is the card's own constant. `/underlyings`: `TestPostUnderlyingsConfirmRoundTripFromPayload`, cases: a text (name), a number (buy_price), an emptied value (note), a switch turned on (options), a switch turned off (an absent switch on a row that has it on stays off after confirm, not on), and the add form (symbol, market, kind). The `/settings` round-trip test belongs to the outside bug issue (Left out), not to this Interface.

## Issues

### 1 underlyings-confirm-payload (interface)

Module: contracts, pages, ot, web (the judged cut above). Size: L.
Files: contracts/schemas/page.underlyings.v2.json, web/src/generated/underlyings.v2.ts, contracts/fixtures/page.underlyings.v2.golden.json, contracts/fixtures/page.underlyings.v2.confirm.json, contracts/fixtures/page.underlyings.v2.readonly.json, contracts/fixtures/page.underlyings.v2.bad-version.json, contracts/fixtures/MANIFEST.synthetic-v1.json, internal/contracts/underlyings_v2_test.go, docs/contracts/page-schemas.md, internal/pages/underlyings/builder.go, internal/pages/underlyings/page.go, internal/pages/underlyings/builder_test.go, internal/pages/underlyings/page_test.go, docs/contracts/pages.md, cmd/ot/underlyings_post.go, cmd/ot/underlyings_site.go, cmd/ot/underlyings_post_test.go, cmd/ot/underlyings_site_test.go, docs/contracts/ot/pages/underlyings_page.md, web/src/types/underlyings.ts, web/test/pages/underlyings/view.test.tsx
Design: docs/design/owner-confirm-forms.md#interface
Contracts: page.underlyings v2 (new); v1 stays frozen
Order: first.
Goal: the v2 schema, type, fixtures, builder and route carry `fields`, and a second post made from them saves. In `web/src/types/underlyings.ts` it adds v2 to the supported versions (`[1, 2]`), so the v1 fixtures of `web/test/pages/underlyings/view.test.tsx` still pass. The fixture `page.underlyings.v1.bad-version.json` holds `schema_version` 2, which is now supported, so the bad-version case of `view.test.tsx` reads `page.underlyings.v2.bad-version.json`, whose `schema_version` is 3; no other line of that test changes.
Acceptance: `TestUnderlyingsV2Fixtures` in `internal/contracts/underlyings_v2_test.go`, `TestBuildConfirmCarriesFields` in `internal/pages/underlyings/builder_test.go` and `TestPostUnderlyingsConfirmRoundTripFromPayload` in `cmd/ot/underlyings_post_test.go` pass (the draft lists every item).

### 2 underlyings-confirm-card

Module: web. Size: S.
Files: web/src/pages/underlyings/view.tsx, web/test/pages/underlyings/view.test.tsx, web/src/types/underlyings.ts
Design: docs/design/owner-confirm-forms.md#decisions
Order: after 1. PR 2629 (the Edit link and On/Off text in `view.tsx`) is already on main.
Goal: `ConfirmCard` is the second form of the flow; the web type accepts v2 only.
Acceptance: the `new:` tests of `web/test/pages/underlyings/view.test.tsx` (form posts the payload's fields, token and `confirm=1`; unchecked switch stays off; no form without the page `form`) pass on the v2 fixtures (the draft lists every item).

## Left out

- The /settings half (Decision 4) is built as its own one-module bug issue, #2637, with `Design: none`, since it edits cmd/ot only; this design lists no entry for it. Record for that issue: the test `TestPostSettingsConfirmRoundTripFromPayload` in `cmd/ot/settings_test.go`, cases: an integer and a decimal param, an emptied param (reset to default), a row excluded, a row un-excluded (absent `excluded` stays off), and two rows with one excluded (repeated `row` and `excluded` survive); it builds the hidden fields by the rule of `SettingsPage.tsx:78-85` (`param.<key>`, `row`, `excluded` of the payload), since it cannot run the card.
- Rejected alternatives: keeping the pending change on the server and signing only its id (needs a store, an expiry and a cleanup for a two-post flow that has none, and the token already binds nothing, Doubts 2); echoing the whole post body (would carry `expires`, `mac` and unknown names into the payload); a cap that clips long values (a silent write of a changed value, Decision 2); splitting entry 1 by module (a window where the builder emits v2 and no route fills `fields`, Doubts 1).

- A token the post checks: `ownerDummyToken` stays (Doubts). `/settings` schema and web card: unchanged (Decision 4).
- Redesign of the preview, a cancel link, a result page beyond the `?saved=<n>` redirect.
- Other two-step owner posts: `grep -n '"confirm"' cmd/ot/*.go` finds only `settings.go` and `underlyings_post.go`.

## Doubts

1. Entry 1 spans four modules; a split would leave a window where the builder emits v2 and no route fills `fields`.
2. The token binds nothing: `ownerDummyToken` is constant and the post ignores it (a deviation from the HMAC token of `docs/design/underlyings.md` Decision 5 that main already has, `underlyings_site.go:97`). The second post proves the form, not the token.
3. A row a job changes between the two posts is overwritten without appearing on the card: the second post writes the fields that differ from the row at that time. Stated limit.
4. `fields` bakes the wire names of the form into the payload; a rename of a form field is a version bump.
5. The settings preview wrapper makes the edit table show unsaved values until the owner confirms or leaves; a reload shows the stored ones.

## Adversarial

Source: <runner home>/state/optdesk-drafts/designs/owner-confirm-forms/adversarial-2026-11-17.md.
- B1 (files outside entry 1): adopted; `underlyings_site.go` and `docs/contracts/pages.md` are in its Files.
- B2 (v3 schema and `TestGeneratedV3Types`): gone, no v3 schema now (S1).
- B3 (cap truncates): adopted, Decision 2.
- Gap 1: the settings card is not run by a Go test; the test applies its rule and the web test covers underlyings. The token deviation is named in Doubts 2. Adopted.
- Risk 2: cap, order and duplicates: adopted (whitelist order, repeated `row` and `excluded` in the settings cases).
- Simpler 3: both adopted (Decisions 1 and 4); `Confirm.form` is gone.
- Lock-in 4: `form` removed, one frozen schema; v1 payload without `fields`: entry 2 sets the web type to v2 only. Rest in Doubts 4.
- Claim 5: B1 fixed; line references refreshed to 4438c820.
- Slip 6: no v3, no v3 type order; entries 1 and 2 as before.
- Valid cut: not a rule-made cut; Doubts 1. Round-trip: the path comes from the payload, repeated values tested. Adopted.
- Advisor 1 (type switch and fixtures in one entry) vanishes with S1. Advisor 2 (cases per page, add form): adopted in Interface.
