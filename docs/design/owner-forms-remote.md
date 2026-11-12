# Owner forms: one token, no loopback guard

Owner, 2026-11-12: "optdesk 的程序只接受从本机地址（127.0.0.1:28000）提交的表单 / 这个限制能不能去掉，只要有token验证就好" and
"--owner-edit这是啥？？？为什么那么多限制". Port 28000 is published to the tailnet by `tailscale serve` (a local proxy),
so today `hostNameOK` and `settingsform.Check` refuse the phone. Dev mode: speed over wording.

**Who may act changes.** Before: a process of the owner's machine, with a page MAC handed to whoever GETs the page.
After: anyone who can reach the port and holds `OT_OWNER_TOKEN`. The network (tailnet, loopback, LAN) is no longer part
of the proof; the token is the only one, so its 32-byte minimum is the only brute-force defence (no rate limit).

## Modules

- ot (`cmd/**`), the only module edited. `internal/compute/settingsform` (module compute, including its
  `upload.go`), `settingsapi`, the page builders, schemas and web are not edited.

## Interface

None changed. `Contracts: unchanged`.

## Decisions

1. **Routes exist as today.** `/owner-settings`, `/owner-job-console`, `/owner-underlyings` and the flex import
   upload `POST /owner-flex-import` (`flexImportPostRoute`, `site.go`; landed with #1901) are registered when the
   store DSN line exists (`rd.edit != nil`, `site.go` registration) and answer 404 otherwise, as now. With the DSN
   but `OT_OWNER_TOKEN` unset or under 32 bytes, every owner POST answers `403` with the line `owner token not set:
   set OT_OWNER_TOKEN`. `--owner-edit` is still accepted and ignored (one log line) so no launcher breaks; it no
   longer gates anything.
2. **No network guard.** `hostNameOK`, `ownerLoopbackNames`, the `Check` and `CheckUpload` calls (the latter in
   `postOwnerFlexImport`, `flex_import_post.go`, which also reads `expires`/`mac` from the multipart fields), the
   remote-address, Host and Origin checks and the loopback `--addr` requirement for owner routes are gone from `cmd/ot`. Content type and the 64 KiB
   cap stay (body limits); the flex upload keeps its own declared-length and `MaxBytesReader` cap (64 MiB). `--live-quotes` keeps its loopback rule (not an owner form).
3. **One owner token, `OT_OWNER_TOKEN`**, replaces `OT_FORM_KEY` as the proof. Name only here; the value is never
   printed, logged, put in a URL, a page, a test or a fixture. Read through the env file `envFile()` of `env.go`
   reads (the runner secrets file when `CREW_SECRETS`/`CREW_SERVICE` are set, else `$OT_HOME/.env`); the owner sets
   the line. Minimum 32 bytes.
4. **One path: cookie plus login page.** `GET /owner-login` shows one field; `POST /owner-login` (field
   `owner_token`) compares it, and on a match sets cookie `ot_owner` holding the token itself (`HttpOnly;
   SameSite=Strict; Path=/; Max-Age=2592000`) and answers 303 to `/`. Every owner POST reads the cookie. No Bearer
   header, no form-field token. Both the login and `ownerCookieOK` call the one function `tokenMatches(got, want)`
   (`crypto/subtle.ConstantTimeCompare` over equal-length SHA-256 sums). No match: `403 forbidden`, no store read, no
   write. `Secure` is left off (loopback is plain http; `tailscale serve` encrypts the tailnet leg).
5. **CSRF** is `SameSite=Strict` on the cookie plus the token. No Origin check (`tailscale serve` may rewrite Host
   and default ports differ, so an Origin-equals-Host test could refuse every phone post).
6. **Page form fields stay, inert.** The page contracts require `expires` and `mac` (schemas, builders, web).
   `cmd/ot` passes a constant well-formed dummy (`expires` `0`, `mac` 64 zeros) where it passed `form.Token(now)`
   and ignores the posted values; this covers `flex_import_site.go`, which builds its token with `form.Token`, as
   the three other pages do. `form.Parse` is still used (it verifies no MAC; it takes the key `OT_FORM_KEY` named
   today, and that argument goes with the key). Retiring the fields is Left out. The gap texts that
   mention `--owner-edit` are left alone (Left out).
7. **Overrides** (named; the owner's merge of this file stands for them, those files are owner-only):
   `settings-form.md` Decision 5 (loopback remote address, loopback Host and Origin, MAC and expiry, loopback
   listen, `OT_FORM_KEY`, and its sentence "a tunnel, a reverse proxy ... not allowed by any flag");
   `flex-xml-import.md` line 23 (the `--owner-edit`, loopback `--addr`, `OT_FORM_KEY`, hidden MAC pattern);
   `job-console.md` point 3 ("who may act": the page token becomes the owner token); the underlyings design's guard.
8. **Still refused.** A POST without the cookie token (403); a wrong content type; a body over the cap; any owner
   write that calls neither `guardOwnerPost` nor `ownerCookieOK`. For `POST /owner-flex-import` the cookie is checked by
   `ownerCookieOK(r, e)` (the one function that reads the cookie and calls `tokenMatches`; `guardOwnerPost` calls it
   too, but reads a 64 KiB urlencoded form, which a multipart upload is not, so the flex post calls `ownerCookieOK`
   directly) before `MultipartReader` is called, so no
   part is read, no file parsed and nothing imported without it: 403.
9. **Named smaller points.** `site.go` is 621 lines on main (not 603): the one login registration line is the only
   growth. `wire.go` is 790 lines and must stay under 800 (the key plumbing it holds is deleted, none added).
   `GET /owner-login` and `POST /owner-login` without a DSN answer 404 like the other owner routes (they are
   registered under `rd.edit != nil`); with the DSN and no token, the login answers 403 with the same
   `owner token not set` line. `settingsform/upload.go` and `settingsapi.UploadChecker` stay, unused (Left out).

## Issues

1. **ot: one owner token on every owner POST.** Module ot.
   Files: `cmd/ot/ownertoken.go`, `cmd/ot/ownertoken_test.go`, `cmd/ot/settings.go`, `cmd/ot/settings_test.go`,
   `cmd/ot/serve.go`, `cmd/ot/serve_test.go`, `cmd/ot/env.go`, `cmd/ot/env_test.go`, `cmd/ot/wire.go`,
   `cmd/ot/site.go`, `cmd/ot/jobconsole_site.go`, `cmd/ot/jobconsole_site_test.go`, `cmd/ot/jobconsole_post.go`,
   `cmd/ot/jobconsole_post_test.go`, `cmd/ot/underlyings_post.go`, `cmd/ot/underlyings_post_test.go`,
   `cmd/ot/underlyings_site.go`, `cmd/ot/underlyings_site_test.go`, `cmd/ot/flex_import_post.go`,
   `cmd/ot/flex_import_post_test.go`, `cmd/ot/flex_import_site.go`, `cmd/ot/flex_import_site_test.go`,
   `cmd/ot/flex_import_kit_test.go`, `cmd/ot/flex_import_limits_test.go`.
   The owner test helpers that build `ownerEdit{form, listen}` with a key and tokens (`settings_test.go`,
   `jobconsole_post_test.go`, `underlyings_site_test.go`, `underlyings_post_test.go`, and the flex import kit and
   post tests, which sign the upload with `expires`/`mac`) are rewritten to post the
   cookie; `serve_test.go`'s `--owner-edit` loopback refusal test is deleted.
   Design: this file, Decisions 1 to 6, 8 and 9; overrides the parts named in Decision 7.
   Acceptance (results, read in code and tests):
   - `grep -n "hostNameOK\|ownerLoopbackNames\|form\.Check\|CheckUpload\|OT_FORM_KEY\|envFormKey" cmd/ot/*.go | grep -v _test`
     prints nothing, and `form.Token` is not called in `cmd/ot` outside the dummy helper;
     `serve.go` has no loopback test for the owner routes and a test starts with and without `--owner-edit`.
   - `tokenMatches` is called by the `/owner-login` handler and by `ownerCookieOK` and by nothing else; every owner
     POST handler calls `guardOwnerPost` first, except `postOwnerFlexImport`, which calls `ownerCookieOK` first
     (`TestOwnerPostsAllGuarded` posts to
     each route, `/owner-flex-import` among them, without a cookie: 403).
   - `POST /owner-flex-import` without the cookie answers 403 with a multipart body whose first part is not read
     (a test body with a broken stream after the first part still gets 403, not the broken-upload path), imports
     nothing, and with the right cookie on a non-loopback `RemoteAddr` and `Host: x.ts.net:28000` the files are
     imported; the 64 MiB cap and 8-file limit tests pass unchanged.
   - A POST with the right cookie succeeds on a request with `Host: x.ts.net:28000` and a non-loopback `RemoteAddr`;
     a wrong cookie and no cookie give 403.
   - With the DSN present and the token unset, every owner POST gives 403 and the body holds `OT_OWNER_TOKEN`;
     with no DSN the routes are 404 as before.
   - `POST /owner-login` with the right token sets `ot_owner` with `HttpOnly`, `SameSite=Strict` and answers 303; a
     wrong one answers 403.
   - The pages of the three routes carry `expires` `0` and a 64-zero `mac` and validate against their schemas.
   - Tests generate the token; no file in the diff holds a token value.
   - New files under 600 lines; `site.go` (621 lines) gets one registration line for the login route and no growth
     beyond that, else reported in the PR.
   Contracts: unchanged.

## Left out

- Retiring `expires`/`mac` from the schemas, builders, web and fixtures; deleting `settingsform` and the `settingsapi`
  `Token`/`Check`/`CheckUpload` declarations, and `settingsform/upload.go` with its test (module compute, interface
  change; they stay, `Check` and `CheckUpload` unused).
- The gap texts naming `--owner-edit` and their fixtures; `docs/contracts/**` mentions of it.
- `--live-quotes`/`POST /options/refresh` loopback rule; rate limiting, per-device tokens, logout, `Secure` cookie,
  audit log; the `Tailscale-User-Login` header as a second proof.
- Edits to the owner-only design files named in Decision 7.

## Adversarial

Review file: owner-forms-remote-adversarial.md.

- B1 (page schemas require expires/mac): adopted. Fields stay, constant dummy, retiring is Left out (Decision 6).
- B2 (gap-text fixtures not in Files): adopted by dropping the pages issue; one issue, ot only.
- B3 (404 vs 403): adopted. 404 without a DSN as today, 403 naming the key with DSN and no token (Decision 1).
- B4 (cookie vs compare inconsistent): adopted and simplified: cookie holds the token, one `tokenMatches`, no Bearer or
  field path (Decision 4).
- Drop Origin check: adopted (Decision 5), Host rewriting by `tailscale serve` is unverifiable offline.
- Attack: is there an owner write the design does not name? Yes, `POST /owner-flex-import` (#1901, multipart, guarded
  by `CheckUpload` and the loopback rule). Answered: it is in Decisions 1, 2, 6, 8 and Files; cookie before any
  part is read; `CheckUpload` is gone from `cmd/ot`; `settingsform/upload.go` stays unused (Left out). The grep of
  origin/main for `form.Check` (settings.go), `CheckUpload` (flex_import_post.go) and `form.Token` (settings.go,
  jobconsole_site.go, underlyings_site.go, flex_import_site.go) finds each in Files.
- Env file wording (`CREW_SECRETS`): adopted (Decision 3).
- `form.Parse` still used, `form.Token` not in the grep: adopted (Decision 6, acceptance grep).
- wire.go, rewritten test helpers, login in `ownertoken.go`, site.go 603 lines: adopted in Files and acceptance.
- Point 2 (no `--addr 0.0.0.0` stop): adopted as stated risk: the token is the only barrier, 32-byte minimum.
- Point 3(a) (keep the page MAC): not adopted, it keeps `OT_FORM_KEY` and misses "one token".
- Point 4 (dead `Check` code left): accepted, named in Left out.
