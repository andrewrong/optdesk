# ot page `flex_import`

Order: none (no name in the known-pages text)

Route: `flex-import`. Name for `ot page`: none: the page is a route only. Post route: `owner-flex-import` (only with `--owner-edit`). Family files: `cmd/ot/flex_import_site.go` (the page, registered with `registerPage` in an `init()`) and `cmd/ot/flex_import_post.go` (the post). The common rules of every page are in `docs/contracts/ot.md`.

## Window

The page is built per request (`ot serve`). A GET lists the 50 newest import records (login `import`); a post shows the result of the files it read.

## Lines it reads

The gate reads the env file and the Flex file. `OT_PG_DSN_OPTION_LEDGER` must be set and `flex.toml` under `OT_HOME` must load. The list of past imports is read through `deps.openOptionLedgerRO` with `OT_PG_DSN_PAGES`. The post opens one writer per post, at the first record to write, with `deps.openOptionLedgerStore` and `OT_PG_DSN_OPTION_LEDGER`, and closes it. The account map is the one of the option ledger job (`flexAccounts`).

## Left out

Without `--owner-edit`, with the ledger line unset or with a Flex file that does not load, the page is read-only: `form` is null and one gap line says why (the line names a key or a file, never a value) and `POST /owner-flex-import` is 404 when `--owner-edit` is off. A list of past imports that cannot be read is an empty list and the gap `past imports cannot be read`; the form stays.

## Contract

- `GET` and `HEAD /flex-import`: the page `page.flex_import.v1` built with `flex_import.Build`. With `--owner-edit` and a working gate it carries the form token (`form`: `expires`, `mac`, `max_files` 8, `max_file_text`) and `recent`; otherwise `form` is null.
- `POST /owner-flex-import` (`--owner-edit` only; another method 405 with `Allow: POST`): a multipart body of the fields `expires` and `mac` first, then up to 8 file parts (plain XML or gzip). The order of the checks: the declared length against 64 MiB, the two fields, the token (`CheckUpload`), the gate, then the files. A file part before the fields is refused. Parts of another name are ignored and not counted; the ninth file is not read and shows `too_many_files`.
- Each file is imported alone with `ImportFlexReport`; a file that is refused or fails gets `RecordImportFailure` with a reason (`not_report`, `too_large`, `bad_gzip`, `too_many_files`, `store_error`) and never stops the others. Two files of the same post have distinct query ids (request id and ordinal). The file name is shown only (base name, at most 64 bytes) and is never stored. When the writer cannot be opened every file is `failed` with `store_error` and no record is written.
- Codes: 200 the page with the result (also the read-only page); 403 `upload refused: the form is not valid or has expired, load the page again`, nothing written; 404 without `--owner-edit`; 405 another method; 413 `upload too large: at most 64 MiB` (declared over, length unknown, or the stream over the cap).
- The log lines of the post carry the component, the status and the file index only: no file name, account number, token or DSN.
- The page has no name for `ot page`.

## Version

v1: the route `flex-import` and the post route `owner-flex-import`.
