# Arch files generated at check time: `arch.toml` and `.go-arch-lint.yml` leave git

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-09
Revision: 2
Revision note: revision 2 follows the adversarial read (`scratchpad/adv-arch-generated.md` of the drafting session (not in the repository; the pull request body gives its full path), `## Adversarial`): `--check` is removed instead of reinterpreted, the migration counts and the acceptance-text rule are stated, the false "nothing else reads them" is corrected, the CI cost is estimated.

Status: draft for design review. One entry touches `.github/**` (owner-only): the window writes it.

Source: owner decision 2026-11-09, answer "a": stop committing the generated files; CI generates them at check time. Read on origin/main at d6acc6b: `scripts/arch/main.go` and its test, `.github/workflows/checks.yml` and `ci-go.yml`, `docs/design/modules.md`, `AGENTS.md`, `README.md`, `.gitignore`, `docs/contracts/*`, `go.mod`.

## What main holds today

- `scripts/arch` derives both files from the `depends` column of `modules.md` (`generate`). `arch.toml` takes the table only; `.go-arch-lint.yml` also reads the Go tree (`deriveTree` drops absent directories; go-arch-lint itself flags a package in no component). In recent package issues (#844, #837, #809) only `.go-arch-lint.yml` changed; `arch.toml` changes when `modules.md` changes. `go run ./scripts/arch` writes both; `--check` fails when a committed file differs.
- Readers in code and CI: `checks.yml` step "arch.toml agrees with modules.md" runs `--check`; `ci-go.yml` runs `go tool go-arch-lint check` (reads `.go-arch-lint.yml` at the root), lists both files in `paths:` and in the affected-packages regex (a change widens to `./...`). `README.md` lines 18-20 name them. `scripts/arch/main_test.go` has three tests that use `--check`; `TestArchCheckCommittedFiles` compares the repo's committed files. `modules.md` names them in the `depends` bullet, the ownership bullet, the repo row paths and a contracts cell. `.gitignore`, `AGENTS.md`, `docs/contracts/*` and the Makefile do not name them.
- Readers in text: 57 files under `docs/` name them, and Files lines and acceptance lines of issues copy them (for example `grep -c "gaphistory" .go-arch-lint.yml` is at least 4; `go run ./scripts/arch && git diff --exit-code arch.toml .go-arch-lint.yml`, `docs/design/option-notices.md`). The crew tool does not read either file (grep of `/Users/nomoshen/selfapp/tool` is empty); it resolves a `Files:` path to a row of `modules.md`.
- Queue today (counted 2026-11-09): of 16 open issues, 11 name a file in `Files:` (#855 #854 #852 #847 #840 #838 #827 #826 #822 #813 #786; the same 11 name one in their text); of 111 registered drafts in the scratchpad `drafts-*` directories, 37 name a file, 33 of them in a `Files:` line.
- `ci-go` median wall time over the last 48 successful runs: 83 s (min 74 s, max 111 s).

## Goal

An issue that adds a Go package no longer lists `arch.toml` or `.go-arch-lint.yml` in `Files:`; two such issues never overlap on them; the import limits are still enforced on every pull request.

## Decisions

1. Both files are deleted from git and listed in `.gitignore` (`/arch.toml`, `/.go-arch-lint.yml`). A developer runs `go run ./scripts/arch` before `go tool go-arch-lint check` (README says so).
2. `ci-go.yml` runs `go run ./scripts/arch` immediately before `go tool go-arch-lint check`, same job. `generate` already exits 2 on a modules.md that does not parse or derive, so that step is the check of the table. `paths:` and the affected-packages regex drop the two files and add `docs/design/modules.md` to both (at head it is in neither) (correctness first: a change of `depends` or of an entry cell changes the limits). Cost: a pull request that edits `modules.md`, mostly design PRs, pays a full ci-go run, median 83 s, and widens to `./...`; the design PRs are about 15 of the last 20 commits touching it. Accepted; the "watch CI duration" round compares the per-workflow median afterwards.
3. `scripts/arch --check` is removed (the smaller change: no compare, no git dependency, no new tests). The flag, the compare loop and the three tests that use it go; the other tests of `generate` stay. The `checks.yml` step "arch.toml agrees with modules.md" is deleted, since ci-go now runs the same derivation.
4. Migration, one rule. The window acts after entry 2 merges: it re-registers every registered draft that names either file in `Files:` or in acceptance text (33 and 37 today) without it, and edits every not-yet-started open issue that names one (11 today; no PR open on it). An issue already started drops the hunk on rebase (a PR with a hunk of a deleted file conflicts modify/delete, and the worker deletes the file again). An acceptance line that read a generated file is replaced by "ci-go runs `go run ./scripts/arch` and `go tool go-arch-lint check`" under `Checked by CI`, or by a grep of `docs/design/modules.md`. Entry 2 names both files in its own `Files:` (the deletion), so it overlaps the 11 issues once; that overlap is accepted and ends when entry 2 merges.
5. `modules.md` prose changes in this branch (`## Modules`). The repo row keeps the two paths in its cell until entry 2 lands, because the deletion of a path no row owns would fail `crew issue check` on entry 2; a design follow-up removes them (Left out).
6. Own code stays: `scripts/arch` is the repo's own derivation (no library derives go-arch-lint config from a markdown table). The table format it parses (five cells, the `## Modules` heading, the "one constructor each" sentence) is now an input of a CI step; a reformat of `modules.md` that breaks the parse fails ci-go, which is the check.

## Modules

No new row. `docs/design/modules.md` is edited in this branch: the `depends` bullet says the two files are generated by `go run ./scripts/arch` before go-arch-lint in `ci-go`, from entry 2 on are in `.gitignore` and never committed, only entry 2 naming them in `Files:`, and until it merges are still committed (`docs/design/arch-generated.md`); the `ot` sentence of the same bullet says `scripts/arch` generates component `ot` and `ci-go` lints against it (no `checks.yml` comparison); the ownership bullet drops them from the shared root files; the contracts cell says "the edges of the generated arch.toml".

## Interface

None: no Go interface file, no schema and no proto changes; `scripts/arch` is a command.

## Issues

Every test calls `t.Parallel()`, writes files only under a temporary directory, never sleeps. Acceptance is checked by reading, grep and git; test runs are under "Checked by CI". `BASE` is `git merge-base HEAD origin/main`.

### 1 arch-ci-generate (window)

Module: ci. Size: S. Owner-only: the window writes it (`.github/**`).
Files: `.github/workflows/ci-go.yml`, `.github/workflows/checks.yml`
Design: docs/design/arch-generated.md (Decisions 2, 3)
Order: first.
Goal: ci-go generates the files before the lint and no longer lists them; the checks step that compares them is gone.
Acceptance:
1. `grep -n "go run ./scripts/arch" .github/workflows/ci-go.yml` prints one line, and its line number is lower than that of `go tool go-arch-lint check`.
2. `grep -c "arch.toml\|go-arch-lint.yml" .github/workflows/ci-go.yml` prints 0 (unescaped dots, so it matches the plain spelling); `grep -cF 'arch\.toml' .github/workflows/ci-go.yml` prints 0 and `grep -cF 'go-arch-lint\.yml' .github/workflows/ci-go.yml` prints 0 (the escaped spelling of the regex); `grep -c "docs/design/modules" .github/workflows/ci-go.yml` is at least 2.
3. `grep -c "scripts/arch" .github/workflows/checks.yml` prints 0.
4. `git diff --name-only BASE` lists exactly the two Files.
Checked by CI: ci-go and checks run on the pull request; until entry 2 the files are still committed, and `--check` is gone from checks, so nothing compares them.

### 2 arch-untrack (repo)

Module: repo. Size: S.
Files: `scripts/arch/main.go`, `scripts/arch/main_test.go`, `.gitignore`, `README.md`
Design: docs/design/arch-generated.md (Decisions 1, 3, 4)
Order: after 1 (ci-go must already generate; the window merges 1 first). The overlap of its Files with queued package issues is accepted once.
Goal: the files are deleted from git and ignored; `--check` is removed; README names the two commands.
Acceptance:
1. `git ls-files arch.toml .go-arch-lint.yml` prints nothing; `grep -c "^/arch.toml$\|^/.go-arch-lint.yml$" .gitignore` prints 2.
2. `grep -c '"check"\|bytes.Equal' scripts/arch/main.go` prints 0, and `grep -c "ArchCheck" scripts/arch/main_test.go` prints 0.
3. `grep -c "func Test" scripts/arch/main_test.go` is at least 4 (the tests of `generate` and of the lint on a scratch tree stay).
4. `grep -n "scripts/arch\|go-arch-lint" README.md` prints `go run ./scripts/arch` on a line before `go tool go-arch-lint check`, and `grep -c "check  *# fail when" README.md` prints 0.
5. `git diff --name-only BASE` lists no path outside the Files line.
Checked by CI: `go test ./scripts/arch -race -count=1`, ci-go (generates, then lints).

## Left out

- Removing `arch.toml .go-arch-lint.yml` from the repo row paths of `modules.md` (Decision 5): a one-line design follow-up after entry 2.
- A follow-up issue (module repo) for AGENTS.md: one sentence, "go-arch-lint needs `go run ./scripts/arch` first". AGENTS.md is not edited here.
- Overlap that remains: the entry cells of `modules.md` (a new declarations-only package) and `docs/contracts/*.md` still serialise interface and implementation issues. `scripts/arch/main.go` has no `declarationsOnly` list at d6acc6b; that rule lives in the prose of `modules.md`.
- A crew-side rule that rejects a `Files:` line naming either file: crew is not in this repo; the design reviewer rejects it by Decision 4.
- A Makefile target or hook for generation.
- Keep `arch.toml` committed and generate only `.go-arch-lint.yml`: both files come from the same column and both serialised issues; keeping one keeps the overlap.
- Stop emitting `arch.toml` at all: other readers of the edges (the contracts cell, the design reviewer) cite it; dropping it is a separate change.
- Keep `--check` and reinterpret it as a compare against a fresh generation: with nothing committed there is nothing to compare; the generation step that exits 2 on a bad table is the check (Decision 3).

## Adversarial

Read: `scratchpad/adv-arch-generated.md` of the drafting session (not in the repository; the pull request body gives its full path).

1. Gap against the goal. Adopted: the goal is stated for package issues that only add implementation; `.go-arch-lint.yml` is the real overlap file and `arch.toml` rode along (kept, since the owner chose both). The remaining shared files are in `## Left out`; the "11 waiting" is now counted (11 of 16 open issues).
2. Acceptance text reads the files. Adopted: counts and the rule in Decision 4 (drafts re-registered, issues edited, acceptance lines replaced); the 57 docs are named. The loss of static evidence that a package got an edge is accepted: `Checked by CI` carries it, and `modules.md` still shows the `depends` edges. AGENTS.md sentence: a follow-up issue in `## Left out`.
3. A simpler way. Adopted in part: `--check` is removed and ci-go's own generation step checks the table. Not adopted: stop emitting `arch.toml` or keep it committed alone, because the owner decided both and a half change keeps one file in the `Files:` lines.
4. Lock-in. Adopted: Decision 6 states the table format is an input of a CI step; the CI cost of `modules.md` in `paths:` is estimated in Decision 2 (median 83 s) and kept. A stale ignored local file: README says to run the generator first; not further guarded.
5. False claim. Adopted: "nothing else reads them" is replaced by the readers in text and code. The modify/delete point is adopted: with `--check` gone there is no "fails --check" claim; the rule is "drops the hunk on rebase" and the window's edit of issues.
6. Slip. Adopted: Order of entry 2 is "after entry 1", its own overlap is accepted once; the git-dependent tests are gone with `--check`. Not adopted: a machine-enforced order; the window merges entry 1 first, as it writes it.
