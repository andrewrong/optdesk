// Package account is the AccountStore over pgx for the DSN line OT_PG_DSN_ACCOUNT (role ot_w_account): the writer of
// account.capture, account.position, account.value and account.execution, insert-only, and the Reader of the views
// rd.account_*_r1 (docs/design/account-data.md Decisions 5 and 10). No statement of this package changes or removes a row.
package account
