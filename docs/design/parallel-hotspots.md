# Parallel hotspots: shared files that serialise independent module issues

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-09
Revision: 3
Revision note: revision 2 follows the adversarial read (the window scratchpad file adv-hotspots.md, answered in `## Adversarial`): the entry-cell edit is dropped for tree discovery of both package kinds, the `deps` change is smaller, the dataset and fixture changes are left out (only two issues hit them, and the fixture claim was false), and the migration says plainly which issues keep their hunks.
Revision 3 follows the design verdict on f2ff88f: `internal/contracts/genpb` stays importable by the sub-packages of dependents of contracts (Decision 1, acceptance 4 of issue 1); issue 2 rewrites the `ot` import sentence and line 27 of `modules.md` so that the row states the enforced rule; the stale revision-1 references in `## Adversarial` are renamed; the owning row of a new `api.go` package, the expected edges and the rejected alternatives are written down; and the facts are read again on e1c3fed (the `deps` struct has 69 fields, not 65; 822, 827, 883, 843 and 863 landed; 908, 926 and 942 are new).

Status: draft for design review.

Source: owner goal 2026-11-09 (the queue ran at 1-2 of `[drive] parallel = 8`); `docs/design/arch-generated.md` (the first hotspot, same shape). Read on origin/main at e1c3fed (the line numbers of `scripts/arch/main.go` are unchanged since aa7290b): `scripts/arch/main.go` (`implPackage` line 47, `parseImpl` 83-97, `declarationsOnly` 118, `declListed` 122, `listedImports` 146-166, `derive` 172-210, `deriveTree` 295, `generate` 378-397), `cmd/ot/wire.go`, `cmd/ot/wire_test.go`, `scripts/hygiene/main.go`, `docs/design/modules.md`, `docs/contracts/*.md`, `git log --since=2026-11-08 --name-only`, and the 22 open issues (Files lines).

## What main holds today

Changes since 2026-11-08: `modules.md` 20, `ot.md` 18, `wire.go` 14, `wire_test.go` 13, `records.go` 10, the fixture manifest 10, `store.md` 9, `generate.sh` 8.

| hotspot | why two issues of different modules collide | open issues that name it |
|---|---|---|
| H1 `scripts/arch/main.go`: `declarationsOnly` (line 118) and the backtick scan `implPackage` (lines 47, 90, 150) | a declarations-only package is known only by being in the Go list and in an entry cell of `modules.md`; an implementation package that `cmd/ot` or a dependent may import is known only by a backticked `internal/<a>/<b>` in `modules.md` text | Files: 852 854 855 786 (four interface issues) and 894 (same file). Body only (cell edit by the window): 852 854 855 name `modules.md` |
| H3 `cmd/ot/wire.go` (`deps` struct, 69 fields, `newDeps`) and `wire_test.go` (`NumField() != 69`) | each new job, page or command adds a field, a `newDeps` line and a bumped count | Files: 908 (1 of 22); `wire.go` had 15 changes and `wire_test.go` 14 since 2026-11-08, from issues that have landed (883, 843) |
| H4 `docs/contracts/ot.md`, `store.md`, `compute.md` | one file per module, edited by every issue of it | ot.md: 942 926 908 886; store.md: 854 852; compute.md: 855 786 (8 issues) |

Open issues serialised by H1, H3 or H4: 852 854 855 786 894 908 942 926 886 (9 of 22). The four interface issues name H1 and H4 each.

## Goal

Two issues of different modules never name the same file for H1, H3 and H4: a new declarations-only or implementation package, a new command's deps, and a new interface document each add files and edit none of the shared ones.

## Decisions

1. H1, tree discovery of both package kinds (replaces the Go list and the backtick scan). `scripts/arch` walks `internal/*/*` (the tree walk `goDirs` already does) and classifies each package directory by files, no text and no list:
   - generated: the two in the existing `generated` variable (`internal/contracts/gen`, `internal/contracts/genpb`); never an implementation package. Importable: `internal/contracts/genpb` only (the generated gRPC services), by the sub-packages of every row that depends on contracts, as today. Today that edge exists only because the contracts entry cell lists it and `listedImports` reads the cell; once cells are not read, a constant `importableGenerated = []string{"internal/contracts/genpb"}` in `scripts/arch/main.go` gives the same edge. `internal/contracts/gen` stays importable by the root `contracts` only, as today, and no root component of a dependent receives `genpb`. The classification does not add every `generated` package to the listed set.
   - declarations-only: holds `api.go` and no other non-test `.go` file except `doc.go`.
   - test kit: a non-test file imports `"testing"` (today `internal/compute/contracttest`); not importable by `cmd/ot`, importable by dependents as today (the cell named it for that).
   - implementation: any other package with a non-test `.go` file; `cmd/ot` may import it when its owning row is in the depends cell of `ot` (below), and nothing else may.
   - owning row: the row whose path glob matches the package first (the first-row rule of `modules.md`; `internal/store/**` is store, and so on). No entry cell or `## Interface` line names a new declarations-only package any more, so the glob is the record of who owns it. Adding a package whose only non-test file is `api.go` is an interface change: its issue carries the `(interface)` mark and a Design line (AGENTS.md).
   What replaces what, checked in the code: `declListed` (122-131) and `listedImports` (146-166) read the classification instead of entry cells and `declarationsOnly`; `parseImpl` (83-97, the "one constructor each" list that `ot` may import) is replaced by "every discovered package that is not generated, not a test kit and owned by a row in the depends cell of `ot`" (the component `internal/profile` of repo is such a package, so it needs no line of its own); `derive` (172-210) takes the sub-package list from the tree instead of from the `impl` argument, so the first output of `generate` now also reads the tree (the sentence in `arch-generated` that the first file "takes the table only" ends; both files still come out of `generate`). `deriveTree` is unchanged but for its `listed` argument. Entry cells of `modules.md` are not edited and no longer read; its "one constructor each" list and the sentences that describe the old reading are rewritten by issue 2. Cost: (a) `cmd/ot` may now import any discovered implementation package of the rows in its depends cell, where before the text listed which; the list was a human whitelist and the lint no longer enforces it. The row must not say otherwise, so issue 2 rewrites the `ot` import sentence, the "one constructor each" list, the sentence "Any other import of a package of a module by `cmd/ot` is a finding" and line 27 of `modules.md` to state this rule, and the enforced set of `ot` equals the set the row states; `ot` is the one row that composes, and review still sees every import in `cmd/ot`; (b) `api.go` and `doc.go` are names a CI step depends on; a package with another file is an implementation package, so a mistake makes it importable by `cmd/ot` only, never by a sibling module; (c) one function of about 40 lines and at least four tests; (d) the crew check that flags an entry-file edit still reads the cells: a new `api.go` in a sub-package is not in a cell, so the flag rests on the author's `(interface)` mark and Design line (AGENTS.md); I could not read crew (unknown, see Left out).
2. H3, one deps source per command, no reflect. A command's file declares `type <name>Deps struct{...}` and in `init` puts a constructor into a package map keyed by the command name (`extDeps["accountimport"] = func() any { return accountImportDeps{...} }`); `deps` gets one field `ext map[string]any`, filled by `newDeps` from that map; the command reads it with `depsOf[T any](d deps, name string) T`; a test builds a `deps` with `ext` set to fakes. The `NumField() != 69` assertion in `TestNewDepsFilled` is removed (the loop that checks each field is not nil stays), so no later issue edits that number. The 69 existing fields stay. Cost: about 25 lines in `cmd/ot/depsext.go`; a missing key is a type-assertion panic at the command, caught by the command's own test, not by the compiler; `deps` has two sources (field, map) until old fields move one by one; no library fits (a DI framework is heavier than the struct). Own implementation, because the glue is a map and a generic function; it sits behind `depsOf`, which a library could replace (owner 2026-11-08).
3. H4, one contract document per package or command: `docs/contracts/store/<package>.md`, `docs/contracts/compute/<package>.md`, `docs/contracts/ot/<command>.md`. The three old files stay unchanged, as indexes. A new document is in the `docs` row (`docs/**`) until its module's row takes the glob, and a store issue editing a docs-row path is a path of another module (AGENTS.md), so issue 2 adds the three globs to the three rows, once. Cost: one small `modules.md` edit, one extra link for readers. Unknown: whether crew reads the "contract document" column or the contract cell to check an interface issue; if it compares to the file `docs/contracts/store.md` the new files need a crew change (Left out).
4. Own code and libraries: none new beyond Decisions 1 and 2.
5. Order: issue 1 after #894 (same files); issue 2 after issue 1 (its text states the rule of issue 1); issue 3 independent of both.

## Modules

No new row. Row paths changed in `docs/design/modules.md` (issue 2): store gains `docs/contracts/store/**`, compute gains `docs/contracts/compute/**`, ot gains `docs/contracts/ot/**`. Issue 2 also rewrites the text of the same file that states the rules issue 1 replaces: line 27 (a declarations-only package is one an entry cell names and the `declarationsOnly` list), the sentence of the `depends` bullet that says `scripts/arch` derives the sub-package imports from the entry cells, the `ot` import list ("one constructor each") with the sentence that any other import by `cmd/ot` is a finding, and the `imports` cell of the `ot` row of the second table ("the implementation packages named above"). The new text says: a package is declarations-only, a test kit or an implementation package by its files; `ot` imports every discovered implementation package of the rows in its depends cell and `internal/profile`; `genpb` is the one generated package that sub-packages of dependents import. The entry cells are not edited; they go on naming the packages and are documentation. A new `api.go` package belongs to the row whose path glob matches it first, and adding one is an interface change with a Design line.

## Interface

None. No Go interface file, schema or proto changes.

## Issues

Every test calls `t.Parallel()`, writes files only under a temporary directory, never sleeps. Acceptance is by reading, grep and git; test runs are under "Checked by CI". `BASE` is `git merge-base HEAD origin/main`. No line names a generated arch file.

### 1 arch-tree-discovery (repo)

Module: repo. Size: M.
Files: `scripts/arch/main.go`, `scripts/arch/main_test.go`
Design: docs/design/parallel-hotspots.md (Decision 1)
Order: after #894 (same two files; its overlap is accepted once).
Goal: declarations-only, test-kit and implementation packages come from the tree; the Go list and the backtick scan are gone.
Acceptance:
1. `grep -nw "declarationsOnly" scripts/arch/main.go` prints nothing, and `grep -nw "implPackage" scripts/arch/main.go` prints nothing.
2. `grep -nw "func parseImpl" scripts/arch/main.go` prints nothing (read `generate`: it takes the package list from the tree).
3. `grep -n '"api.go"\|"doc.go"\|"testing"' scripts/arch/main.go` prints a line for each of the three, and `grep -n 'internal/contracts/genpb' scripts/arch/main.go` prints at least two lines (the `generated` variable and `importableGenerated`).
4. In `scripts/arch/main_test.go`, read that tests on a `t.TempDir` tree (a table with rows `contracts`, `a`, `b` and `c`; `b` depends on `a` and `contracts`, `ot` depends on `a` and `b` but not on `c`) assert both the presence and the absence of each edge, one case per line:
   - declarations-only `internal/a/decl` (`api.go` alone): in the imports of the root `b`, of `b/sub`, of the own-row `a/sub` and of `ot`.
   - test kit `internal/a/kit` (a non-test file imports `"testing"`): in the imports of `b/sub` only; not of the root `b`, not of `a/sub`, not of `ot`.
   - implementation `internal/a/impl` (`api.go` plus `x.go`): in the imports of `ot` only; not of `b`, `b/sub` or `a/sub`. An implementation package of row `c` is not in the imports of `ot`.
   - generated: `internal/contracts/genpb` in the imports of `b/sub` (a dependent of contracts) and not in those of the root `b`; `internal/contracts/gen` in the imports of the root `contracts` only.
   `grep -c "^func Test" scripts/arch/main_test.go` is greater than at BASE by at least 4.
5. `git diff --name-only BASE` lists no path outside the Files line.
Checked by CI: `go test ./scripts/arch -race -count=1`; ci-go generates and lints on this tree with the same components as before (the lint passes with `internal/store/bar`, `internal/compute/contracttest` and `internal/contracts/genpb` unchanged in behaviour: each is imported by the same components as at BASE).

### 2 modules-contract-doc-globs (docs)

Module: docs. Size: S.
Files: `docs/design/modules.md`
Design: docs/design/parallel-hotspots.md (Decision 3)
Order: after issue 1 (its text states the rule issue 1 implements; the globs of Decision 3 are in the same file).
Goal: the three module rows own the per-package contract directories, and the text of `modules.md` states the import rules of issue 1.
Acceptance:
1. `grep -n "^| store |" docs/design/modules.md | grep -c "docs/contracts/store/\*\*"` prints 1; the same with the row `compute` and `docs/contracts/compute/\*\*` prints 1; the same with the row `ot` and `docs/contracts/ot/\*\*` prints 1.
2. `grep -c "docs/contracts/store/\*\*" docs/design/modules.md` prints 1 (no other row), and the same for the other two.
3. `grep -c "one constructor each" docs/design/modules.md` prints 0, `grep -c "declarationsOnly" docs/design/modules.md` prints 0, `grep -c "named above for construction" docs/design/modules.md` prints 0, and `grep -c "internal/clock/realclock" docs/design/modules.md` prints 0 (the list is gone).
4. `grep -n "every discovered implementation package" docs/design/modules.md` prints a line in the `ot` paragraph of the `depends` bullet, and `grep -n "genpb" docs/design/modules.md` prints a line that says it is the one generated package that sub-packages of dependents import.
5. `git diff --name-only BASE` lists exactly `docs/design/modules.md`.
Checked by CI: `crew issue check` of this change; ci-go (generation reads the table).

### 3 ot-deps-per-command (ot)

Module: ot. Size: S.
Files: `cmd/ot/depsext.go`, `cmd/ot/depsext_test.go`, `cmd/ot/wire.go`, `cmd/ot/wire_test.go`
Design: docs/design/parallel-hotspots.md (Decision 2)
Order: independent.
Goal: a new command adds its own file and no field of `deps`, no `newDeps` line, no count.
Acceptance:
1. `grep -nw "func depsOf" cmd/ot/depsext.go` prints one line, and `wc -l < cmd/ot/depsext.go` is under 60.
2. `grep -nw "ext" cmd/ot/wire.go` prints the field and its fill in `newDeps` (word match: `context` does not match).
3. `grep -n "NumField() *!= *[0-9]" cmd/ot/wire_test.go` prints nothing, and `grep -n "want 69" cmd/ot/wire_test.go` prints nothing.
4. Each of the 69 field names of `deps` at BASE still appears as a field: for the names in `git show BASE:cmd/ot/wire.go` between `type deps struct {` and the closing brace, none is missing from the new file (read the two lists; the pull request names the count, 69 plus `ext`).
5. `git diff --name-only BASE` lists exactly the four Files.
Checked by CI: `go test ./cmd/ot -race -count=1` (the field-nil loop stays; a test of `depsOf` with a missing key expects the panic text); ci-go.

## Migration of the open issues

An issue already started (a pull request or a worker session) keeps its hunks: it merges with the shared-file edits it already has, and the hunk of a later issue conflicts once, resolved on rebase. Only an issue not yet started is migrated, by the window editing its Files (a registered draft is re-registered). A change that carries the shared file it already edited stays valid (the old files and cells remain, so nothing here deletes a path an issue edits). 822, 827, 883, 843 and 863 have landed and need nothing.

| issue | names | not started: edit to Files | started: keeps |
|---|---|---|---|
| 852 mark | H1 H4 | after issue 1, drop `scripts/arch/main.go`, `scripts/arch/main_test.go`; after issue 2, `store.md` becomes `docs/contracts/store/mark.md`; `internal/store/api.go` stays | scripts/arch and store.md hunks |
| 854 flexsection | H1 H4 | the same; `docs/contracts/store/flexsection.md` | same |
| 855 blockapi | H1 H4 | the same with `compute`; `docs/contracts/compute/blockapi.md` | same |
| 786 trackapi | H1 H4 | the same; `docs/contracts/compute/trackapi.md` | same |
| 894 untrack | H1 | unchanged; issue 1 waits for it | n/a |
| 908 notices site | H3 H4 | after issue 3, a new field goes to the command's own deps; `wire.go`, `wire_test.go` leave Files; `ot.md` becomes `docs/contracts/ot/notices_site.md` (after issue 2) | wire.go, wire_test.go, ot.md hunks |
| 886 account import | H4 | `ot.md` becomes `docs/contracts/ot/accountimport.md` (after issue 2) | ot.md hunk |
| 926 site | H4 | `ot.md` becomes `docs/contracts/ot/site.md` (after issue 2) | ot.md hunk |
| 942 dryrun | H4 | `ot.md` becomes `docs/contracts/ot/dryrun.md` (after issue 2) | ot.md hunk |

The body mentions of `modules.md` in 852 854 855 (a cell edit) need no edit of an issue: the window does not edit the cell any more, and a cell edit already made is harmless. #894 (pull request 912) is open and not merged at e1c3fed; issue 1 starts after it lands.

## Left out

- `internal/store/api.go` and its `Open<Name>Func` types (852 edits it; 7 changes since 2026-11-08): the types are the root interface other modules import, and a per-package `Open` func in each store sub-package changes the interface shape, so it needs its own Design line; only 852 names the file now.
- `cmd/ot/runonce.go`, `scheduler.go`, `scheduler_test.go` (9 changes): the job list is assembled there, still one collision for every new job; a job registry is a larger change with no issue naming it yet.
- H5 (`internal/contracts/gen/records.go`, `contracts/generate.sh`) and H6 (the fixture manifest): only 882 (H5, H6) and 784 (H6) name them. Not designed here; note for a later one: the manifest is not read only by `TestFixtureManifest`: `scripts/hygiene/main.go` (lines 22, 44, 87-99) hardcodes `MANIFEST.synthetic-v1.json` and `checks.yml` runs it, and `internal/contracts/schema_test.go` reads the marker; and `--schema-output` with one file per schema has not been run.
- Rejected alternatives (the reasons are in Decisions and `## Adversarial`): an `//arch:decl` marker in each package (a second convention where the `api.go` rule exists already); keeping the text whitelist of `cmd/ot` and only discovering declarations-only packages (the one-constructor list stays a shared file that every new implementation package edits, which is the hotspot); editing the entry cells of `modules.md` from each issue (what H1 is today); a dependency-injection library for the deps of Decision 2 (a framework is heavier than a map and a generic function, and the glue sits behind `depsOf`); reflect or generated registries for `deps` (a type error moves to run time with no gain over the string-keyed map).
- Crew: what it reads to flag an interface edit (the entry cell) and to check an interface issue (the contract document column); unknown from here, not in this repository. A crew change may be needed for a new `api.go` and for `docs/contracts/<module>/`.
- Moving the 69 `deps` fields, and splitting the text of the three contract documents: nothing needs it.
- A rule that rejects a `Files:` line naming a shared file.

## Adversarial

Read: the window scratchpad file adv-hotspots.md.

1. Gap against the goal. Adopted: the goal now names only H1, H3 and H4; `internal/store/api.go` and `runonce.go`/`scheduler.go` are in Left out as still colliding; `modules.md` is shared for new rows of the table and stays so; the serial gates are three (issue 1 after #894, issue 2 after issue 1, issue 3 free), down from five.
2. Biggest risk (the entry-cell removal breaks the lint). Adopted: verified at `implPackage` (47, used 90 and 150), `derive` and `listedImports`; the cells are not edited, and issue 1 replaces both readers with the tree. The revision-1 text for the entry-cell removal is gone. Revision 2 has a Decision 2 (deps per command) and an issue 3 that are unrelated to it.
3. Simpler way. Adopted: tree discovery for both kinds and no cell edit (the revision-1 entry-cell decision and the revision-1 package-list decision, now merged in Decision 1); `NumField() != 69` dropped (H3). Not adopted: an `//arch:decl` marker in each package (a second convention to learn, where the `api.go` rule is already in `modules.md`); H5 and H6 moved to Left out as proposed.
4. Lock-in. Adopted: the `api.go`, `doc.go` and `"testing"` rules are named as CI inputs (Decision 1); the `sums/` marker, `*_r1.go` naming and the reflect registry are gone (a string-keyed map, no reflect; the two sources of `deps` are named as a cost). Per-package contract directories need crew changes: kept, stated as unknown in Decision 3 and Left out.
5. Likely false claims. Adopted: the H6 sentence is deleted and the real readers are cited in Left out; the `--schema-output` run is noted as never done and not relied on; `modules.md` line 27 and the `ot` import sentence are stale after issue 1: issue 2 (docs row) rewrites them, and issue 1 does not edit them.
6. Slip. Adopted: the Migration section says plainly that started issues keep their hunks; issue 1 acceptance 5 now says "no path outside the Files line" (the old "exactly" conflicted with #894); greps use `-w` or anchored patterns (`ext` no longer matches `context`) and no generate.sh grep remains.
