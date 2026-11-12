# ot page `job_console`

Order: none (no name in the known-pages text)

Route: `job-console`. Name for `ot page`: none: the page is a route only. Post route: `owner-job-console` (only with `--owner-edit`). Family files: `cmd/ot/jobconsole_site.go` (the page, registered with `registerPage` in an `init()`), `cmd/ot/jobconsole_post.go` (the post), `cmd/ot/jobconsole_open.go` (the three openers, registered in `extDeps` under `jobconsole`) and `cmd/ot/jobconsole_sched.go` (the scheduler's side). The design is `docs/design/job-console.md`; the common rules of every page are in `docs/contracts/ot.md`.

## Lines it reads

- The page reads through the pages opener of `jobconsole` (`jobrequestpg.OpenReader`), opened once per process with `OT_PG_DSN_PAGES`.
- The post opens a poster per post with the `OT_PG_DSN_SETTINGS` line that `ownerEditSecrets` returns (`jobrequestpg.OpenPoster`) and closes it after the post.
- The scheduler opens the consumer with the runner DSN it already holds (`jobrequestpg.OpenConsumer`).
- No new `.env` line.

## Gaps

- `job_requests_unavailable`: no reader (no opener, the demo profile, a failed open) or a read error. The page is 200 with the job table and no requests. A deploy ahead of the migration of `settings.job_request` shows this gap.
- `no_such_request`: `?request=<id>` names no request.

## Contract

- `GET` and `HEAD /job-console[?request=<id>]`: `page.job_console.v1`. With `--owner-edit` it carries the form token and `post_path` `/owner-job-console`; without it `form` is null.
- `POST /owner-job-console` (`--owner-edit` only; another method 405). `guardOwnerPost` is called before any field is read; a refusal is 403 with `forbidden`, and no store is opened.
- Field `action` is `run` or `withdraw`; any other value is 422.
  - `run`: `job`, `day` (`YYYY-MM-DD`), `force` (`on`). 422 for an id not in the table, a listed-only job, `force` on a job without force, a bad day, a today-only job whose day is not today in the job's zone, or an any-day job whose day is after today in the market zone. 409 `this request is being withdrawn; press Run again when it shows withdrawn` when the open request of the same job, day and force is being withdrawn. 503 when the store fails or is busy. 303 to `/job-console?request=<id>` otherwise, also for the same request already open. The post knows no slot: a request before the slot is accepted and the scheduler holds it.
  - `withdraw`: `id`. 422 for an id that is no request; otherwise the withdrawal is written (idempotent) and the answer is 303.
- Every refusal text is fixed and holds no DSN, field value or error text.
- The scheduler's console is off, with one warn line, when its consumer cannot be opened, when `Start` fails or when the backfiller cannot force; the scheduler goes on, and a failed `Start` is tried again at every turn.

## Version

v1: the route `job-console` and the post route `owner-job-console`.
