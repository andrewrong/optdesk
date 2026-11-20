# option structure matcher: position recognition names debit verticals, unequal-lot pairs, calendars, diagonals, straddles, strangles, iron condors and collars instead of "other"

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-20

Source: owner UI doc row 12 (asks optdesk-5479 and optdesk-5507): the structures page shows "Unidentified" for a put debit spread and for a pair of unequal lots. The owner asked for a library; the survey found none in Go, Python or TypeScript that recognises strategies from legs. Decided: port the algorithm of QuantConnect Lean's OptionStrategyMatcher (C#, Apache-2.0). Advisor decisions of 2026-11-20: count live legs only; the remainder of an unequal pair is its own row; the Adversarial section covers partial closes and assignment.

Goal: every open option position of an alias is shown as a named structure (vertical credit or debit, calendar, diagonal, straddle, strangle, iron condor, collar, covered call, single leg). When lots are unequal, the largest whole multiple forms the structure and the leftover lots are their own row. "Unidentified" is left only for legs no table entry names.

Existing design: `docs/design/option-positions.md` Decision 4 made the rule (credit verticals of equal quantity, covered calls, singles, the rest `other`; "a position is never split"). This design replaces that rule inside `internal/compute/optionstruct` and says so here; it does not edit `option-positions.md`.

## Today

All paths at origin/main of 2026-11-20.

- `internal/compute/optionstruct/recognise.go:97-130` `recogniseGroup` pairs a sold and a bought leg of one right and one expiry only when `pairable` (`:132-135`) finds equal absolute quantity; `pairUnit` (`:145-153`) names a credit pair `put_spread` or `call_spread` and a debit pair `other`. Everything else goes to `entangled` (`:165-201`), which makes any group of related legs one `other` unit, and to `classifySingles` (`:205-237`), which names single legs and a covered call from the shares of the cycles (`cycleShares`, `:241-261`).
- The kinds are `put_spread`, `call_spread`, `covered_call`, `naked_call`, `short_put`, `long_put`, `long_call` and `other` (`recognise.go:12-21`). `Evaluate` (`optionstruct.go:24-46`) puts `other` units in `StructureResult.Other` and the rest in `Structures`.
- `compute.Structure.Kind` is a free string (`internal/compute/api.go:653`); no schema holds an enum of kinds. `contracts/schemas/page.structures_list.v1.json:281` has `kind_text` as a plain string (1 to 200 characters) and `web/src/pages/structure/StructuresList.tsx:37` prints it; the web needs no change.
- The existing ledger already splits a partly closed position into a closed piece and an open piece of the remaining quantity (`internal/compute/optionledger/lots.go:45-98` `matchOptions`, `statusOf` `:140-151`: closed, expired, assigned, exercised). So `LedgerPosition.Status == "open"` pieces carry live quantity only. An assigned piece leaves the open list and its shares enter `LedgerCycle.Shares`.
- Consumers of the kinds (found by grep over `internal`, `cmd`, `contracts`, `web`):
  - `internal/pages/structures_page/list.go:25-29` `kindTexts` (English labels; an unknown kind prints the raw kind string, `:267-270`), `list.go:155-210` `noteOf` (the `other` note), `list.go:233-253` `otherReason`, `detail.go:203-212` (title and subline), `cells.go:169` (`naked_call` max loss "No limit"), `zh_texts.go` (Chinese notes and reasons).
  - `internal/pages/risk_page/builder.go:62,99,122` counts `res.Structures` into Delta and Vega and adds the note `textOtherKinds` when `res.Other` is not empty.
  - `internal/compute/pagefig/optiontiles.go:24-60` (home options tiles): adds the net premium of every structure that has a sold leg unless its kind is `long_put` or `long_call`, counts `naked_call` as uncovered, and turns a negative open premium into the gap `no_data` (`:55-59`). A debit structure has a negative net premium, so today a debit pair (kind `other`) already makes this tile a gap.
  - `internal/jobs/bookrisk/load.go:314` reads both lists and no kind; `internal/pages/home_options/member.go` and `cmd/ot` (`wire.go:340`) pass the constructor through and read no kind.
- Figures by unit: `risk.go:23-74` has a formula per kind (`spreadRisk` takes `legs[0]` and `legs[1]`); `payoff.go:14-34` returns no payoff for `other` and assumes all legs share an expiry; `marks.go:142-176` computes marks, Greeks, earned share and distances for every kind except `other`, `Decide` only for one sold leg, the compare row only for a single sold leg; `marks.go:280-306` `recentCommission` sums the fills of `l.p.Fills`.
- Tests pinned to the old rule: `recognise_test.go` `TestPutDebitSpreadIsOther`, `TestDiagonalIsOther`, `TestPositionNeverSplit`, and `testdata/other_kinds.json` (cases diagonal, straddle, quantity_mismatch).
- Lean, read at `https://github.com/QuantConnect/Lean/tree/master/Common/Securities/Option/StrategyMatcher` on 2026-11-20: `OptionStrategyDefinitions.cs` is a list of definitions, each `Create(name, underlyingLots, legs...)` with legs `CallLeg(q, predicates)` or `PutLeg(q, predicates)`; a predicate is a function of the legs chosen so far and the candidate position (strike and expiry comparisons). `OptionStrategyLegDefinition.TryMatch` accepts a position of the right and the sign of the leg quantity and the leg multiplier is `position quantity / leg quantity` (integer division, so a whole number of times). A definition match takes the minimum of the leg multipliers (and of the underlying lots) and removes that many units from each position; the rest stays in the collection. the enumerator that sorts by descending leg count orders definitions by leg count, most first, and `OptionStrategyMatcher.MatchOnce` matches greedily in that order. `AllDefinitions` adds the inverted definition (every leg sign reversed) unless it is already named.

## Decisions

1. Source and attribution. The recogniser is a port of the algorithm of Lean's OptionStrategyMatcher (QuantConnect, Apache-2.0): a table of definitions made of leg predicates, matched by descending leg count, each match taking the largest whole multiple available. It is written in Go in `internal/compute/optionstruct` behind the existing interface `compute.OptionStructures`; no Lean code is copied, only the table facts and the loop. Each new file carries the line "Algorithm and definition table after QuantConnect Lean, OptionStrategyMatcher, Apache-2.0, https://github.com/QuantConnect/Lean". Design line of every issue: own implementation, because no Go, Python or TypeScript library recognises strategies from legs. No new dependency. Not ported: Lean's second candidate order (CoveredShortsFirst) and its objective function (fewest unmatched positions); this port is the first candidate only (Decision 5), with a fixed tie-break.
2. Input is the live legs. Only positions with `Status == "open"` are matched; their `Quantity` is already the open quantity. Open pieces of one contract (underlying, currency, right, strike, expiry) form one matching position; its lots are drawn from its pieces in order of `OpenedOn`, then `PositionID`. A partly closed position therefore matches with the lots it still has; an assigned or exercised piece is not open and is not matched. A position whose numbers do not parse is `other` alone, as today.
3. The table. Matching is per underlying and currency. Each definition lists legs with a sign and a lot count; strikes `K`, expiries `E`.
   - Four legs: `iron_condor` (long put K1, short put K2, short call K3, long call K4, K1<K2<K3<K4, one expiry); `reverse_iron_condor` (the same with every sign reversed).
   - Shares and two legs: `collar` (shares of the cycles at least the lot times multiplier, short call Kc, long put Kp, Kp<Kc, one expiry).
   - Two legs, one expiry, one right, opposite sides: credit put vertical (short put above the long put) is `put_spread`; credit call vertical (short call below the long call) is `call_spread`; debit put vertical (short put below the long put) is `put_debit_spread`; debit call vertical (short call above the long call) is `call_debit_spread`. Equal strikes do not match.
   - Two legs, a put and a call, one expiry, same side: `long_straddle` and `short_straddle` (same strike); `long_strangle` and `short_strangle` (put strike below call strike).
   - Two legs, one right, opposite sides, different expiries: `calendar_spread` (same strike) and `diagonal_spread` (different strikes). Lean has the calendar only; the diagonal is added here and the direction is not part of the name (the legs show it).
   - Kinds that already exist keep their strings: `covered_call`, `naked_call`, `short_put`, `long_put`, `long_call`, `put_spread`, `call_spread`, `other`. The names in Lean for the verticals are Bull put, Bear call, Bear put, Bull call; this port keeps the existing strings for the credit pair and adds `_debit_` for the other pair so no stored label changes.
4. Order. Definitions are tried in this order: `iron_condor`, `reverse_iron_condor`, `collar`, the four verticals, the straddles, the strangles, `calendar_spread`, `diagonal_spread` (descending leg count as in Lean, shares counted as a leg; within a count, the order of this list). A definition is applied again and again until it matches nothing, then the next is tried.
5. Choice inside a definition and the multiple. For one definition the matcher looks at every combination of matching positions; the combination with the largest multiple wins, then the narrowest strike distance between its first and last strike, then the lowest position ids. The multiple is the smallest, over legs, of the whole number of times the leg's lot count fits in the position's lots (Lean's integer division); the structure takes exactly that many lots from each leg and the position keeps the rest. So a short put of 3 lots with a long put of 2 lots, the short above the long, gives a `put_spread` of 2 lots and a `short_put` of 1 lot.
6. The remainder is its own row. Lots left after the table are named by the rules that exist today: legs that are still related (a put and a call of one expiry on different sides, which no table entry names) form one `other` unit with its reason; the others go to `classifySingles` (covered call from the shares left after collars, `naked_call`, `short_put`, `long_put`, `long_call`). Butterflies, ladders, backspreads, boxes, conversions and jelly rolls are not in the table; their legs appear as verticals and singles by Decisions 3 to 5, never as one named structure.
7. A split position stays one position. A lot slice is a copy of the piece with `Quantity` set to the slice (text, no sign change) and the commission of each fill scaled by slice over piece quantity, so `recent_commission` is not counted twice; `OpenPrice`, `Multiplier`, `PositionID` and the fills' own quantities are unchanged. A unit whose slice is a whole piece keeps the piece text verbatim, so an unsplit structure is byte-equal to today.
8. Ids. `Structure.ID` stays `alias:` plus the first leg's `PositionID`; when an earlier unit of the same alias already has that id, the later unit gets `#2`, `#3` appended, in the order the units are made (the matching order, then the leftovers in position-id order). A page link is the path-escaped id (`hrefOf`, `format.go:76`). The set of ids for unchanged positions does not change from run to run.
9. Figures of the new units. One generic rule for a unit of any number of legs of one expiry, from the payoff at expiry (piecewise linear, corners at the strikes and at price 0, the shares of a collar at the basis): max loss and max gain are the lowest and highest corner values; a side that grows without end (the slope above the top strike is negative for loss or positive for gain) is the gap `no_rule`; the break-even is the one price where the payoff crosses zero, else `no_rule`. It replaces `spreadRisk` and gives the same numbers for a credit vertical. A unit whose legs have different expiries (calendar, diagonal) has `no_rule` for the three figures and no payoff. The single-leg formulas and `coveredRisk` stay. A collar with no cost basis has max gain `missing_input`, as a covered call does. Marks, Greeks, earned share and distances are unchanged code and work on any number of legs; `Decide` still needs exactly one sold leg and the compare row a single sold leg.
10. The pages. `kindTexts` gets an English label for each new kind (as the existing ones are English): Put debit spread, Call debit spread, Calendar spread, Diagonal spread, Long straddle, Short straddle, Long strangle, Short strangle, Iron condor, Reverse iron condor, Collar. The detail subline of a new kind ends with a Chinese gloss of the kind kept in `zh_texts.go` (what the legs are and whether the net premium was paid or received). `otherReason` loses the opposite-sides-different-expiries reason (a diagonal is now named) and keeps the put-and-call reason. Max loss and max gain `no_rule` show as the page shows a gap today; no Chinese text is written outside `zh_texts.go`. The web, the schemas and the generated types do not change.
11. The risk page needs no change: it already counts every unit of `Structures`; the new kinds now enter Delta and Vega (they were in `Other`, "not counted in risk"), each by the earliest expiry of its legs (a calendar's Vega lies in the near bucket; stated in Left out). The home Open premium tile adds only structures whose net premium is received; a debit structure is skipped and is no longer a gap. Max loss on that tile sums the numbered max losses as before; a structure with the gap `no_rule` counts as unnumbered.
12. Contracts: unchanged. No file of `contracts/schemas`, `contracts/datasets` or `contracts/proto` changes; the Go interface `compute.OptionStructures` and the types of `compute.Structure` do not change; only the set of strings in `Kind`, the id suffix of Decision 8 and the slice quantity of Decision 7 are stated in the comment of `Structure`.
13. Tests (every issue). `t.Parallel()` and `t.TempDir()`, no sleep, no network, no clock; fixtures are JSON of positions in `internal/compute/optionstruct/testdata`.

## Modules

Two modules, both existing rows of `docs/design/modules.md`: compute (`internal/compute/api.go`, `internal/compute/optionstruct/**`, `internal/compute/pagefig/**`) and pages (`internal/pages/structures_page/**`). No row changes. No web, contracts, jobs or ot file changes. Entries 1, 2 and 4 are independent of each other; entry 3 follows 1 and 2; entry 5 follows 1.

## Interface

The Go interface `compute.OptionStructures` (`internal/compute/api.go`), `StructureInput`, `StructureResult` and `Structure` keep their declarations. One interface file is edited: entry 1 changes only the comment of `Structure` (kinds, id suffix, slice quantity, Decisions 3, 7, 8). `internal/compute/api.go` is 839 lines against the ceiling 840 in `docs/design/ratchet.json`; the comment edit must not add more than one line, and a ceiling is not raised. No schema enum changes: `kind_text` is free text.

## Issues

### 1 combo-1-kind-id-doc (interface)

Module: compute. Size: S.
Order: none.
Files: `internal/compute/api.go`
Acceptance: the comment of `Structure` lists the kinds of Decision 3, the id suffix of Decision 8 and the slice quantity of Decision 7; no declaration changes; the file stays at most 840 lines.

### 2 combo-2-risk-generic

Module: compute. Size: M.
Order: none (no file shared with entries 1, 3, 4, 5).
Files: `internal/compute/optionstruct/risk.go`, `internal/compute/optionstruct/payoff.go`, `internal/compute/optionstruct/risk_test.go`, `internal/compute/optionstruct/payoff_test.go`
Acceptance: Decision 9: one generic rule for units of any leg count with one expiry (and for the collar shares), `no_rule` and no payoff when expiries differ; the credit vertical figures are unchanged; tests with a unit of four legs, of three slices of one contract, a long straddle (unbounded gain), a short strangle (unbounded loss), a calendar (all `no_rule`).

### 3 combo-3-matcher

Module: compute. Size: L.
Order: after 1 and after 2 (entry 3 shares no file with entries 4 and 5).
Files: `internal/compute/optionstruct/match_table.go`, `internal/compute/optionstruct/match.go`, `internal/compute/optionstruct/recognise.go`, `internal/compute/optionstruct/optionstruct.go`, `internal/compute/optionstruct/doc.go`, `internal/compute/optionstruct/match_test.go`, `internal/compute/optionstruct/recognise_test.go`, `internal/compute/optionstruct/testdata/match_kinds.json`, `internal/compute/optionstruct/testdata/other_kinds.json`
Acceptance: Decisions 1 to 8; the table of Decision 3 in `match_table.go`, the loop of Decisions 4 and 5 in `match.go`; the three old tests of Today are replaced by tests of the new rule; partial close, assignment and shares cases of the Adversarial section are fixtures.

### 4 combo-4-tiles-net-credit

Module: compute. Size: S.
Order: none.
Files: `internal/compute/pagefig/optiontiles.go`, `internal/compute/pagefig/optiontiles_test.go`
Acceptance: Decision 11: a structure whose net premium is paid adds nothing to Open premium and does not make it a gap; tests for a debit vertical beside a credit vertical.

### 5 combo-5-structures-page-kinds

Module: pages. Size: M.
Order: after 1 (no file shared with entries 2, 3, 4).
Files: `internal/pages/structures_page/list.go`, `internal/pages/structures_page/detail.go`, `internal/pages/structures_page/zh_texts.go`, `internal/pages/structures_page/list_test.go`, `internal/pages/structures_page/detail_test.go`
Acceptance: Decision 10: labels, Chinese gloss in the subline, reason list; golden pages for a put debit spread and a `short_put` remainder with the id suffix `#2` in the link.

## Checked by CI

- `ci-go` for all entries (`go vet`, `go test` with `-race`).

## After merge (checked by the dispatcher, not by review)

- The window opens `/structures` for the alias of the owner's put debit spread and unequal-lot pair and compares each row's name, lots and max loss with the ledger rows, with a screenshot; the Open premium tile of the home options card is read against the credit structures only.

## Left out

- Butterflies, ladders, backspreads, boxes, conversions, reversals, jelly rolls, inverted strangles, protective put, covered put and protective call: not in the table; their legs show as verticals and singles.
- Lean's second candidate order and objective function (Decision 1): rejected, because the first candidate with a fixed tie-break is deterministic and testable; revisit when a real book shows a worse grouping than the owner's reading.
- Rejected alternative: a Python or TypeScript library or a rule table in configuration; no library fits, and a table is code here because predicates compare strikes.
- Rejected alternative: keep "a position is never split" and name only equal-lot pairs; it leaves the owner's unequal pair as `other`.
- Max loss and max gain for calendars and diagonals (they need a model of the far leg's value at the near expiry); they are `no_rule`.
- Splitting the Vega of a calendar over two expiry buckets on the risk page; it lies in the near bucket.
- A hover hint per kind on the list rows (the schema has no field); the Chinese gloss is in the detail subline only.
- Chinese labels for the kinds: the existing labels are English; changing all labels is a separate decision.

## Adversarial

- Gap vs goal: the goal says every position is named; calendars and diagonals are named but have no max loss figure, and butterflies show as two verticals. The owner sees no "Unidentified" for them, but sees a gap for the figure. Adopted as stated, named in Left out.
- Biggest unnamed risk: a grouping the owner would not choose, because the matcher is greedy with no objective function. Example: short put 2 at 100, long put 1 at 95 and long put 1 at 90 groups into two verticals of different widths by the narrowest-width tie-break, where the owner may think of one wide structure. Not adopted away; the fixtures pin the tie-break and the order is in Decisions 4 and 5.
- Simpler way: only add `put_debit_spread`/`call_debit_spread` and split unequal pairs, with no table. Not adopted: calendars, diagonals, straddles, strangles, iron condors and collars were asked for and a table is the same loop; the simple way would need a second loop later.
- Lock-in: the kind strings reach stored labels only through pages built on demand; no dataset stores a kind. The id suffix `#2` is in page links; a bookmark of a remainder row can point to another row when matching changes. Accepted.
- Claim most likely false: "the ledger already splits a partial close, so open pieces hold live quantity" (Today, `lots.go:45-98`); it was read in code, not run against the owner's data. If a partly closed position were left with its full quantity, structures would be wrong. Entry 3 has a fixture of a partial close built from `optionledger` output, not hand-written.
- Likeliest slip: assignment. A short put leg assigned into shares leaves the long put alone (`long_put`) and the shares appear in the cycle, so a covered-call match and a collar both read `LedgerCycle.Shares`; the shares used by a collar must be removed before `classifySingles` reads them, or one lot of shares covers a collar and a call. Entry 3 fixtures: collar plus a second sold call with shares for one lot only (the second is `naked_call`); an assigned put beside a surviving long put.
- A second slip: a slice's commission scaled in text with `fixed4` can lose the last digit; the test sums the slices against the piece.
- Adopted or not: all points above are named in Decisions or Left out; none is waved away, and the web and the schemas are untouched so a wrong grouping costs a label and a figure, never a stored value.
