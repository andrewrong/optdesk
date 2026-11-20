# ot page `data_health_page`

Order: 130

Route: `data-health`. Name for `ot page`: `ot page --name data_health_page`. Family file: `cmd/ot/data_health_site.go`; registered with `registerPage` in an `init()` (`cmd/ot/pagereg.go`). The common rules of every page are in `docs/contracts/ot.md`. Design: `docs/design/data-health.md`, entry 6.

## Route

`ot serve` serves `/data-health`: GET and HEAD only, another method 405, a build error 500 with the fixed text `internal error`. The page is `page.data_health_page.v1`, built per request for the day of the clock through `data_health_page.Build` (`internal/pages/data_health_page`) over the coverage reader `coverage.Reader`, opened once with the line `OT_PG_DSN_PAGES` through `siteOpeners` (opener in the ext deps under `data_health`). The jobs of the page come from `dataHealthJobs()`: every row of `jobTable`, with the calendar, the ledger datasets and the slot (`DueAt`) of its manifest; `earnings_refresh` and `dividend_capture` are present-only. The dataset row takes the first job of `jobTable` order that lists the dataset (`master.option_list`: `option_list_preopen`).

## Gap `coverage_unavailable`

A missing line `OT_PG_DSN_PAGES`, the demo profile, a failed open or an error of the reader gives the page with the one gap `coverage_unavailable` and no dataset. It is never a failed page: the route still answers 200 with a document that validates.

## Repair button

A cell in state `missing` or `expired` has the verdict `button` when its job can run that day, else `not_refetchable` with the text of the reason; the verdict is the builder's and `cmd/ot` repeats no rule. A button and the button "repair N days" of a dataset post to `/owner-job-console` (`jobConsolePostRoute`), the route of the job console; the posting itself is entry 8, not this document. A plain run is queued, never a forced one.

## Form rules

The page carries `form` `{post_path, expires, mac}` only when the owner edit is on (`siteReaders.edit` set), built like the form of `job_console_page` with `post_path` `/owner-job-console` and the token of `ownerDummyToken`; without the owner edit `form` is null and no cell and no dataset has a button.

## Version

v1: introduced with this document (order 130).

## Gateways

Design: `docs/design/gateway-health.md#9-gateway-health-wire`. The line `OT_GATEWAY_SOCKETS` of the env file is a comma list of `alias=socket path`, such as `main=<path>,wife=<path>`; an alias is lower-case letters and digits and appears once; an unset line means no gateway (the page has no `gateways` array and the scheduler makes no gateway pass); a malformed line is ignored with one log line that names the key only.

- Per request, `ot serve` (and `ot page`) reads one `broker.Health` per gateway socket through `broker.HealthReader` (`grpcbroker.DialHealth`), all at once, each under a 2 s deadline for the dial and the call, and copies it to `Input.Gateways` by alias, in the order of the line. `Answered` is false when the dial or the call fails: the builder gives `unreachable` and the page is never an error page. `cmd/ot` holds no rule: the verdict keys and words are the builder's.
- Per tick, the scheduler reads the same readings and passes one `notify.GatewayFact` per gateway, built by `notify.FactOf` from the `Health` just read, to `notify.GatewayNotifier` (`core.NewGateways`, over the sender and the sent store of the other notifiers) after the slow-run pass. It keeps no previous verdict; the notifier and its marks decide what is sent. A failed or panicking pass is logged by class and never reaches the ticker.
