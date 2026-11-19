# valuation_import_page

Design: `docs/design/valuation-import.md#5`. Schema `page.valuation_import_page.v1`.

- Route: `/valuation-import` (page family `valuation_import_page`, order 175, between `valuation_form` and
  `company_quarters_page`). A GET answers the view `empty`: the file form. With `--owner-edit` off the page says the
  import is off.
- Post path: `/valuation-import-post`, an owner post (owner cookie, `--owner-edit` on). A method other than POST is 405.
  Both actions are native multipart form posts that answer the whole page with its document embedded, not JSON.
  The fields (`action`, `file_sha256`, `tick`, `key.<ticker>`, `r_market.<ticker>`) come before the file part, which is
  the last part.
  - `action=preview`: reads the valuation state, the current `settings.underlying` rows and the latest `DGS10` point,
    plans, and answers the view `preview`. Nothing is written.
  - `action=confirm`: plans again on the state of that moment with the posted picks and writes what the plan says,
    then answers the view `result`. When the posted `file_sha256` differs from the hash of the file sent, nothing is
    written and the answer is the preview of that file.
- Cap: the file is at most 1 MiB (`valueimportapi.MaxFileBytes`); a declared length over the file cap plus a 256 KiB
  form margin, or a file part over the cap, is 413. The owner cookie is checked before any byte of the body is read.
- The upload is never written to disk: the parts are read with `MultipartReader` into memory under
  `http.MaxBytesReader`; there is no `ParseMultipartForm`, no file and no temporary file.
- Write order, each one `valuation.Writer` call, through the role `ot_w_settings` (the line of every valuation post):
  the margin table, the R version (`yield_used` empty, the AI switch and expiry of the plan), then per row the profile
  and the review (type, version date, status `effective`, source `owner`, the review id of the plan). A row that is
  `same`, `older`, `refused` or not ticked is not written; `valuation.ErrClosed` is read as already written; any other
  error is the outcome `failed` with a fixed text that holds no value, and the other rows go on.
- No `ot` command exists for the import: the page is the only way in.
